package handler_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/social/internal/handler"
	"github.com/poro/social/internal/repository"
	"github.com/poro/social/internal/routes"
	"github.com/poro/social/internal/service"
	"github.com/poro/social/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

type api struct {
	t   *testing.T
	app *fiber.App
	key *rsa.PrivateKey
}

func newAPI(t *testing.T) *api {
	t.Helper()
	testdb.Available(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "test", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(jwks.Close)
	verifier, err := jwtauth.NewVerifier(jwtauth.Config{JWKSURL: jwks.URL})
	require.NoError(t, err)

	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Use(httpx.RequestID())
	routes.Register(app, routes.Dependencies{
		Social:  handler.New(service.New(testdb.Pool, zap.NewNop())),
		Health:  health.New("social", "test", map[string]health.Check{"postgres": testdb.Pool.Ping}),
		Metrics: func(c *fiber.Ctx) error { return c.SendString("metrics") },
		JWT:     verifier,
		Limiter: nil, // in-process limiter storage
	})
	return &api{t: t, app: app, key: key}
}

func (a *api) token(user uuid.UUID) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": user.String(), "sid": uuid.NewString(), "jti": uuid.NewString(),
		"iss": jwtauth.Issuer, "aud": jwtauth.Audience, "exp": time.Now().Add(time.Hour).Unix(),
		"roles": []string{"PERSONAL"},
	})
	tok.Header["kid"] = "test"
	signed, err := tok.SignedString(a.key)
	require.NoError(a.t, err)
	return signed
}

type response struct {
	Status int             `json:"-"`
	Header http.Header     `json:"-"`
	Data   json.RawMessage `json:"data"`
	Error  *struct {
		Code string `json:"code"`
	} `json:"error"`
}

func (a *api) do(method, path string, user *uuid.UUID, body any) response {
	a.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(a.t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != nil {
		req.Header.Set("Authorization", "Bearer "+a.token(*user))
	}
	res, err := a.app.Test(req, 10000)
	require.NoError(a.t, err)
	defer res.Body.Close()
	out := response{Status: res.StatusCode, Header: res.Header}
	raw, err := io.ReadAll(res.Body)
	require.NoError(a.t, err)
	if len(raw) > 0 && raw[0] == '{' {
		require.NoError(a.t, json.Unmarshal(raw, &out))
	}
	return out
}

func field[T any](t *testing.T, r response, name string) T {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(r.Data, &m))
	var v T
	require.NoError(t, json.Unmarshal(m[name], &v))
	return v
}

func seed(t *testing.T) (video, owner uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	owner, video = uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	require.NoError(t, repository.EnsureUser(ctx, testdb.Pool, owner))
	require.NoError(t, repository.MarkVideoReady(ctx, testdb.Pool, video, owner, time.Now()))
	return video, owner
}

func TestProbes(t *testing.T) {
	a := newAPI(t)
	for _, p := range []string{"/health", "/health/live", "/health/ready", "/api/v1/health", "/api/v1/health/live"} {
		require.Equal(t, 200, a.do("GET", p, nil, nil).Status, p)
	}
	require.Equal(t, 200, a.do("GET", "/metrics", nil, nil).Status)
}

func TestAuthAndInputErrors(t *testing.T) {
	a := newAPI(t)
	video, _ := seed(t)
	user := uuid.Must(uuid.NewV7())

	r := a.do("PUT", "/api/v1/videos/"+video.String()+"/like", nil, nil)
	require.Equal(t, 401, r.Status)
	require.Equal(t, "unauthorized", r.Error.Code)

	r = a.do("PUT", "/api/v1/videos/not-a-uuid/like", &user, nil)
	require.Equal(t, 400, r.Status)
	require.Equal(t, "invalid_id", r.Error.Code)

	r = a.do("PUT", "/api/v1/videos/"+uuid.NewString()+"/like", &user, nil)
	require.Equal(t, 404, r.Status)
	require.Equal(t, "video_not_found", r.Error.Code)

	req := httptest.NewRequest("POST", "/api/v1/videos/"+video.String()+"/comments", bytes.NewReader([]byte("{")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.token(user))
	res, err := a.app.Test(req, 10000)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, 400, res.StatusCode)

	r = a.do("PUT", "/api/v1/users/"+user.String()+"/follow", &user, nil)
	require.Equal(t, 400, r.Status)
	require.Equal(t, "cannot_follow_self", r.Error.Code)
}

func TestSocialFlowOverHTTP(t *testing.T) {
	a := newAPI(t)
	video, owner := seed(t)
	fan := uuid.Must(uuid.NewV7())
	v := "/api/v1/videos/" + video.String()

	r := a.do("PUT", v+"/like", &fan, nil)
	require.Equal(t, 200, r.Status)
	require.Equal(t, 1, field[int](t, r, "likes"))
	require.True(t, field[bool](t, r, "liked_by_me"))

	require.False(t, field[bool](t, a.do("GET", v+"/stats", nil, nil), "liked_by_me"))
	require.True(t, field[bool](t, a.do("GET", v+"/stats", &fan, nil), "liked_by_me"))
	require.Len(t, field[[]any](t, a.do("GET", v+"/likes?limit=5", nil, nil), "items"), 1)
	require.Len(t, field[[]any](t, a.do("GET", "/api/v1/users/"+fan.String()+"/likes", nil, nil), "items"), 1)

	r = a.do("POST", v+"/comments", &fan, map[string]any{"content": "Wouah"})
	require.Equal(t, 201, r.Status)
	commentID := field[string](t, r, "id")
	r = a.do("POST", v+"/comments", &owner, map[string]any{"content": "Merci !", "parent_id": commentID})
	require.Equal(t, 201, r.Status)

	require.Len(t, field[[]any](t, a.do("GET", v+"/comments", &owner, nil), "items"), 1)
	require.Len(t, field[[]any](t, a.do("GET", "/api/v1/comments/"+commentID+"/replies", nil, nil), "items"), 1)

	r = a.do("PATCH", "/api/v1/comments/"+commentID, &fan, map[string]any{"content": "Wouah 🔥"})
	require.Equal(t, 200, r.Status)
	require.True(t, field[bool](t, r, "edited"))

	r = a.do("PUT", "/api/v1/comments/"+commentID+"/like", &owner, nil)
	require.Equal(t, 200, r.Status)
	require.Equal(t, 1, field[int](t, r, "likes_count"))
	require.Equal(t, 200, a.do("DELETE", "/api/v1/comments/"+commentID+"/like", &owner, nil).Status)

	require.Equal(t, 204, a.do("DELETE", "/api/v1/comments/"+commentID, &owner, nil).Status)

	r = a.do("POST", v+"/shares", &fan, map[string]any{"channel": "whatsapp"})
	require.Equal(t, 201, r.Status)
	require.Equal(t, 1, field[int](t, r, "shares_count"))

	u := "/api/v1/users/" + owner.String()
	r = a.do("PUT", u+"/follow", &fan, nil)
	require.Equal(t, 200, r.Status)
	require.Equal(t, 1, field[int](t, r, "followers"))
	require.Len(t, field[[]any](t, a.do("GET", u+"/followers", nil, nil), "items"), 1)
	require.Len(t, field[[]any](t, a.do("GET", "/api/v1/users/"+fan.String()+"/following", nil, nil), "items"), 1)
	require.True(t, field[bool](t, a.do("GET", u+"/stats", &fan, nil), "followed_by_me"))
	require.Equal(t, 0, field[int](t, a.do("DELETE", u+"/follow", &fan, nil), "followers"))

	r = a.do("DELETE", v+"/like", &fan, nil)
	require.Equal(t, 200, r.Status)
	require.Equal(t, 0, field[int](t, r, "likes"))
}

func TestCommentRateLimit(t *testing.T) {
	a := newAPI(t)
	video, _ := seed(t)
	user := uuid.Must(uuid.NewV7())
	path := "/api/v1/videos/" + video.String() + "/comments"
	for i := range 10 {
		require.Equal(t, 201, a.do("POST", path, &user, map[string]any{"content": "spam"}).Status, "comment %d", i+1)
	}
	r := a.do("POST", path, &user, map[string]any{"content": "spam"})
	require.Equal(t, 429, r.Status)
	require.Equal(t, "rate_limited", r.Error.Code)
	require.NotEmpty(t, r.Header.Get("Retry-After"))

	other := uuid.Must(uuid.NewV7())
	require.Equal(t, 201, a.do("POST", path, &other, map[string]any{"content": "ok"}).Status, "limits are per user")
}
