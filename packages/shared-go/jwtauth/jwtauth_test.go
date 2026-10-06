package jwtauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"
)

var (
	keyOnce sync.Once
	keyA    *rsa.PrivateKey
	keyB    *rsa.PrivateKey
)

func keys(t *testing.T) (*rsa.PrivateKey, *rsa.PrivateKey) {
	t.Helper()
	keyOnce.Do(func() {
		var err error
		keyA, err = rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		keyB, err = rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
	})
	return keyA, keyB
}

type jwksServer struct {
	*httptest.Server
	mu    sync.Mutex
	keys  map[string]*rsa.PublicKey
	down  bool
	calls atomic.Int32
}

func newJWKSServer(t *testing.T, keys map[string]*rsa.PublicKey) *jwksServer {
	s := &jwksServer{keys: keys}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.calls.Add(1)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.down {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		doc := map[string][]jwk{"keys": {}}
		for kid, k := range s.keys {
			doc["keys"] = append(doc["keys"], jwkOf(kid, k))
		}
		_ = json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *jwksServer) set(keys map[string]*rsa.PublicKey, down bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys, s.down = keys, down
}

func jwkOf(kid string, k *rsa.PublicKey) jwk {
	return jwk{
		Kty: "RSA", Use: "sig", Alg: "RS256", Kid: kid,
		N: base64.RawURLEncoding.EncodeToString(k.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes()),
	}
}

type tokenOpts struct {
	kid      string
	key      *rsa.PrivateKey
	method   jwt.SigningMethod
	issuer   string
	audience string
	exp      time.Time
	sub      string
	sid      string
	jti      string
}

func sign(t *testing.T, o tokenOpts) string {
	t.Helper()
	a, _ := keys(t)
	if o.key == nil {
		o.key = a
	}
	if o.kid == "" {
		o.kid = "a"
	}
	if o.method == nil {
		o.method = jwt.SigningMethodRS256
	}
	if o.issuer == "" {
		o.issuer = Issuer
	}
	if o.audience == "" {
		o.audience = Audience
	}
	if o.exp.IsZero() {
		o.exp = time.Now().Add(time.Minute)
	}
	if o.sub == "" {
		o.sub = uuid.Must(uuid.NewV7()).String()
	}
	if o.sid == "" {
		o.sid = uuid.Must(uuid.NewV7()).String()
	}
	if o.jti == "" {
		o.jti = "jti-1"
	}
	tok := jwt.NewWithClaims(o.method, accessClaims{
		Roles:     []string{"PERSONAL", "CREATOR"},
		SessionID: o.sid,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: o.sub, ID: o.jti, Issuer: o.issuer,
			Audience:  jwt.ClaimStrings{o.audience},
			ExpiresAt: jwt.NewNumericDate(o.exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	tok.Header["kid"] = o.kid
	var signed string
	var err error
	if o.method == jwt.SigningMethodHS256 {
		signed, err = tok.SignedString(jwkHMACSecret(o.key))
	} else {
		signed, err = tok.SignedString(o.key)
	}
	require.NoError(t, err)
	return signed
}

// jwkHMACSecret is what an attacker would use in an algorithm confusion attack.
func jwkHMACSecret(k *rsa.PrivateKey) []byte { return k.N.Bytes() }

func newVerifier(t *testing.T, url string) *Verifier {
	t.Helper()
	v, err := NewVerifier(Config{JWKSURL: url})
	require.NoError(t, err)
	return v
}

func TestVerifyAcceptsAuthTokens(t *testing.T) {
	a, _ := keys(t)
	srv := newJWKSServer(t, map[string]*rsa.PublicKey{"a": &a.PublicKey})
	v := newVerifier(t, srv.URL)
	sub := uuid.Must(uuid.NewV7())

	claims, err := v.Verify(context.Background(), sign(t, tokenOpts{sub: sub.String(), jti: "abc"}))
	require.NoError(t, err)
	require.Equal(t, sub, claims.UserID)
	require.Equal(t, "abc", claims.TokenID)
	require.True(t, claims.HasRole("CREATOR"))
	require.False(t, claims.HasRole("ADMIN"))

	_, err = v.Verify(context.Background(), sign(t, tokenOpts{}))
	require.NoError(t, err)
	require.Equal(t, int32(1), srv.calls.Load(), "keys are cached")
}

func TestVerifyRejectsBadTokens(t *testing.T) {
	a, b := keys(t)
	srv := newJWKSServer(t, map[string]*rsa.PublicKey{"a": &a.PublicKey})
	v := newVerifier(t, srv.URL)

	cases := map[string]string{
		"wrong key":       sign(t, tokenOpts{key: b}),
		"unknown kid":     sign(t, tokenOpts{kid: "zzz", key: b}),
		"wrong issuer":    sign(t, tokenOpts{issuer: "evil"}),
		"wrong audience":  sign(t, tokenOpts{audience: "poro-admin"}),
		"expired":         sign(t, tokenOpts{exp: time.Now().Add(-time.Minute)}),
		"hs256 confusion": sign(t, tokenOpts{method: jwt.SigningMethodHS256}),
		"subject not id":  sign(t, tokenOpts{sub: "admin"}),
		"sid not id":      sign(t, tokenOpts{sid: "x"}),
		"garbage":         "a.b.c",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := v.Verify(context.Background(), token)
			require.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}

func TestVerifyFollowsKeyRotationWithRateLimit(t *testing.T) {
	a, b := keys(t)
	srv := newJWKSServer(t, map[string]*rsa.PublicKey{"a": &a.PublicKey})
	v := newVerifier(t, srv.URL)
	now := time.Now()
	v.keys.now = func() time.Time { return now }

	_, err := v.Verify(context.Background(), sign(t, tokenOpts{}))
	require.NoError(t, err)

	srv.set(map[string]*rsa.PublicKey{"a": &a.PublicKey, "b": &b.PublicKey}, false)
	_, err = v.Verify(context.Background(), sign(t, tokenOpts{kid: "b", key: b}))
	require.ErrorIs(t, err, ErrInvalidToken, "an unknown kid inside the minimum gap does not refetch")
	require.Equal(t, int32(1), srv.calls.Load())

	now = now.Add(defaultMinGap)
	_, err = v.Verify(context.Background(), sign(t, tokenOpts{kid: "b", key: b}))
	require.NoError(t, err, "a new kid is picked up after the gap")
	require.Equal(t, int32(2), srv.calls.Load())

	srv.set(nil, true)
	now = now.Add(defaultMaxAge)
	_, err = v.Verify(context.Background(), sign(t, tokenOpts{}))
	require.NoError(t, err, "cached keys keep working while auth is down")
}

func TestVerifyReportsUnavailableKeys(t *testing.T) {
	srv := newJWKSServer(t, nil)
	srv.set(nil, true)
	v := newVerifier(t, srv.URL)
	_, err := v.Verify(context.Background(), sign(t, tokenOpts{}))
	require.ErrorIs(t, err, ErrKeysUnavailable)

	v2 := newVerifier(t, "http://127.0.0.1:1/jwks")
	_, err = v2.Verify(context.Background(), sign(t, tokenOpts{}))
	require.ErrorIs(t, err, ErrKeysUnavailable)
}

func TestJWKSRejectsWeakAndMalformedKeys(t *testing.T) {
	weak, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	srv := newJWKSServer(t, map[string]*rsa.PublicKey{"weak": &weak.PublicKey})
	v := newVerifier(t, srv.URL)
	_, err = v.Verify(context.Background(), sign(t, tokenOpts{kid: "weak", key: weak}))
	require.ErrorIs(t, err, ErrKeysUnavailable, "a JWKS with only weak keys is unusable")

	for name, k := range map[string]jwk{
		"bad modulus":  {Kty: "RSA", Kid: "x", N: "!!", E: "AQAB"},
		"bad exponent": {Kty: "RSA", Kid: "x", N: base64.RawURLEncoding.EncodeToString(make([]byte, 256)), E: "!!"},
		"even exp":     {Kty: "RSA", Kid: "x", N: jwkOf("x", &keyA.PublicKey).N, E: base64.RawURLEncoding.EncodeToString([]byte{2})},
		"empty exp":    {Kty: "RSA", Kid: "x", N: jwkOf("x", &keyA.PublicKey).N, E: ""},
	} {
		_, err := k.rsaKey()
		require.Error(t, err, name)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{")) }))
	defer bad.Close()
	_, err = newVerifier(t, bad.URL).Verify(context.Background(), sign(t, tokenOpts{}))
	require.ErrorIs(t, err, ErrKeysUnavailable)

	_, err = NewVerifier(Config{})
	require.Error(t, err)
}

func TestMiddleware(t *testing.T) {
	a, _ := keys(t)
	srv := newJWKSServer(t, map[string]*rsa.PublicKey{"a": &a.PublicKey})
	down := newJWKSServer(t, nil)
	down.set(nil, true)

	build := func(v *Verifier) *fiber.App {
		app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
		app.Use(Middleware(v))
		app.Get("/me", func(c *fiber.Ctx) error {
			claims, ok := ClaimsFrom(c)
			require.True(t, ok)
			return c.SendString(claims.UserID.String())
		})
		app.Get("/admin", RequireRole("ADMIN"), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Get("/studio", RequireRole("CREATOR"), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		return app
	}
	app := build(newVerifier(t, srv.URL))
	valid := sign(t, tokenOpts{})

	call := func(app *fiber.App, path, auth string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		return resp.StatusCode
	}
	require.Equal(t, http.StatusOK, call(app, "/me", "Bearer "+valid))
	require.Equal(t, http.StatusOK, call(app, "/me", "bearer "+valid))
	require.Equal(t, http.StatusUnauthorized, call(app, "/me", ""))
	require.Equal(t, http.StatusUnauthorized, call(app, "/me", "Basic abc"))
	require.Equal(t, http.StatusUnauthorized, call(app, "/me", "Bearer nope"))

	opt := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	opt.Use(OptionalMiddleware(newVerifier(t, srv.URL)))
	opt.Get("/pub", func(c *fiber.Ctx) error {
		_, ok := ClaimsFrom(c)
		if ok {
			return c.SendString("auth")
		}
		return c.SendString("anon")
	})
	req := httptest.NewRequest(http.MethodGet, "/pub", nil)
	resp, err := opt.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	req = httptest.NewRequest(http.MethodGet, "/pub", nil)
	req.Header.Set("Authorization", "Bearer "+valid)
	resp, err = opt.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, http.StatusForbidden, call(app, "/admin", "Bearer "+valid))
	require.Equal(t, http.StatusOK, call(app, "/studio", "Bearer "+valid))
	require.Equal(t, http.StatusServiceUnavailable, call(build(newVerifier(t, down.URL)), "/me", "Bearer "+valid))

	bare := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	bare.Get("/", RequireRole("ADMIN"), func(c *fiber.Ctx) error { return nil })
	resp, err = bare.Test(httptest.NewRequest(http.MethodGet, "/", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "RequireRole without claims refuses")
}
