package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/poro/auth/internal/config"
)

func TestTokenServiceRoundTrip(t *testing.T) {
	priv, pub := writeRSAKeys(t, false)
	svc := newTokens(t, tokenConfig(priv, pub), nil)
	userID, sessionID := mustID(t), mustID(t)
	roles := []string{"PERSONAL", "CREATOR", "BUSINESS"}

	raw, err := svc.GenerateAccessToken(userID, roles, sessionID)
	require.NoError(t, err)
	claims, err := svc.ValidateAccessToken(raw)
	require.NoError(t, err)
	require.Equal(t, userID, claims.UserID)
	require.Equal(t, roles, claims.Roles)
	require.Equal(t, sessionID, claims.SessionID)
	require.NotEmpty(t, claims.TokenID)
	require.True(t, claims.Exp.After(time.Now()))

	other, err := svc.GenerateAccessToken(userID, roles, sessionID)
	require.NoError(t, err)
	otherClaims, err := svc.ValidateAccessToken(other)
	require.NoError(t, err)
	require.NotEqual(t, claims.TokenID, otherClaims.TokenID, "every token has its own jti")

	parsed, _, err := jwt.NewParser().ParseUnverified(raw, &jwt.RegisteredClaims{})
	require.NoError(t, err)
	require.Equal(t, svc.JWKS().Keys[0].Kid, parsed.Header["kid"])
	aud, err := parsed.Claims.GetAudience()
	require.NoError(t, err)
	require.Equal(t, jwt.ClaimStrings{TokenAudience}, aud)

	refresh, hash, err := svc.GenerateRefreshToken()
	require.NoError(t, err)
	require.Equal(t, HashToken(refresh), hash)
	require.NotEqual(t, refresh, hash)

	_, err = svc.ValidateAccessToken(raw + "x")
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = svc.ValidateAccessToken("not-a-jwt")
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokenServiceRejectsForeignAndForgedTokens(t *testing.T) {
	priv, pub := writeRSAKeys(t, true)
	svc := newTokens(t, tokenConfig(priv, pub), nil)
	raw, err := svc.GenerateAccessToken(mustID(t), []string{"ADMIN"}, mustID(t))
	require.NoError(t, err)

	otherPriv, otherPub := writeRSAKeys(t, false)
	other := newTokens(t, tokenConfig(otherPriv, otherPub), nil)
	_, err = other.ValidateAccessToken(raw)
	require.ErrorIs(t, err, ErrInvalidToken, "a token from another key is rejected")

	key := loadKey(t, priv)
	forge := func(claims jwt.MapClaims, method jwt.SigningMethod, signKey any) string {
		token := jwt.NewWithClaims(method, claims)
		signed, err := token.SignedString(signKey)
		require.NoError(t, err)
		return signed
	}
	base := func() jwt.MapClaims {
		return jwt.MapClaims{
			"sub": mustID(t).String(), "sid": mustID(t).String(), "jti": "x",
			"iss": TokenIssuer, "aud": TokenAudience, "exp": time.Now().Add(time.Minute).Unix(),
		}
	}

	wrongAud := base()
	wrongAud["aud"] = "another-api"
	_, err = svc.ValidateAccessToken(forge(wrongAud, jwt.SigningMethodRS256, key))
	require.ErrorIs(t, err, ErrInvalidToken)

	noExp := base()
	delete(noExp, "exp")
	_, err = svc.ValidateAccessToken(forge(noExp, jwt.SigningMethodRS256, key))
	require.ErrorIs(t, err, ErrInvalidToken)

	noSession := base()
	delete(noSession, "sid")
	_, err = svc.ValidateAccessToken(forge(noSession, jwt.SigningMethodRS256, key))
	require.ErrorIs(t, err, ErrInvalidToken)

	hmacToken := forge(base(), jwt.SigningMethodHS256, []byte("secret"))
	_, err = svc.ValidateAccessToken(hmacToken)
	require.ErrorIs(t, err, ErrInvalidToken, "algorithm confusion is rejected")

	_, err = svc.ValidateAccessToken(forge(base(), jwt.SigningMethodRS256, key))
	require.NoError(t, err, "the forged baseline is otherwise valid")
}

func TestTokenServiceJWKS(t *testing.T) {
	priv, pub := writeRSAKeys(t, false)
	svc := newTokens(t, tokenConfig(priv, pub), nil)
	set := svc.JWKS()
	require.Len(t, set.Keys, 1)
	jwk := set.Keys[0]
	require.Equal(t, "RSA", jwk.Kty)
	require.Equal(t, "RS256", jwk.Alg)
	require.Equal(t, "sig", jwk.Use)
	require.NotEmpty(t, jwk.Kid)

	key := loadKey(t, priv)
	n, err := base64.RawURLEncoding.DecodeString(jwk.N)
	require.NoError(t, err)
	require.Equal(t, 0, new(big.Int).SetBytes(n).Cmp(key.N))
	e, err := base64.RawURLEncoding.DecodeString(jwk.E)
	require.NoError(t, err)
	require.Equal(t, int64(key.E), new(big.Int).SetBytes(e).Int64())
}

func TestTokenServiceRejectsBadKeys(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "private.pem")
	pub := filepath.Join(dir, "public.pem")
	require.NoError(t, os.WriteFile(priv, []byte("nope"), 0o600))
	require.NoError(t, os.WriteFile(pub, []byte("nope"), 0o600))

	_, err := NewTokenService(tokenConfig(filepath.Join(dir, "missing.pem"), pub), nil)
	require.Error(t, err)
	_, err = NewTokenService(tokenConfig(priv, pub), nil)
	require.Error(t, err)

	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecPriv, err := x509.MarshalPKCS8PrivateKey(ec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(priv, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecPriv}), 0o600))
	ecPub, err := x509.MarshalPKIXPublicKey(&ec.PublicKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pub, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: ecPub}), 0o600))
	_, err = NewTokenService(tokenConfig(priv, pub), nil)
	require.Error(t, err)

	rsaPriv, _ := writeRSAKeys(t, false)
	_, otherPub := writeRSAKeys(t, false)
	_, err = NewTokenService(tokenConfig(rsaPriv, otherPub), nil)
	require.ErrorContains(t, err, "does not match")

	cfg := tokenConfig(rsaPriv, otherPub)
	cfg.JWTAccessTTL = "nope"
	_, err = NewTokenService(cfg, nil)
	require.Error(t, err)
	cfg.JWTAccessTTL = "0s"
	_, err = NewTokenService(cfg, nil)
	require.Error(t, err)
}

func TestTokenServiceRejectsWeakKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	priv, pub := writeKeyPair(t, key, false)
	_, err = NewTokenService(tokenConfig(priv, pub), nil)
	require.ErrorContains(t, err, "at least 2048 bits")
}

func TestTokenServiceBlacklist(t *testing.T) {
	priv, pub := writeRSAKeys(t, false)

	withoutRedis := newTokens(t, tokenConfig(priv, pub), nil)
	require.Error(t, withoutRedis.BlacklistToken(context.Background(), "jti", time.Minute))
	_, err := withoutRedis.IsBlacklisted(context.Background(), "jti")
	require.Error(t, err, "blacklist checks fail closed without Redis")

	mr := miniredis.RunT(t)
	svc := newTokens(t, tokenConfig(priv, pub), redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	listed, err := svc.IsBlacklisted(ctx, "jti")
	require.NoError(t, err)
	require.False(t, listed)

	require.NoError(t, svc.BlacklistToken(ctx, "jti", time.Minute))
	listed, err = svc.IsBlacklisted(ctx, "jti")
	require.NoError(t, err)
	require.True(t, listed)

	require.NoError(t, svc.BlacklistToken(ctx, "expired", 0))
	listed, err = svc.IsBlacklisted(ctx, "expired")
	require.NoError(t, err)
	require.False(t, listed, "an already expired token needs no entry")

	mr.FastForward(2 * time.Minute)
	listed, err = svc.IsBlacklisted(ctx, "jti")
	require.NoError(t, err)
	require.False(t, listed, "entries expire with the token")
}

func tokenConfig(priv, pub string) *config.Config {
	return &config.Config{JWTPrivateKeyPath: priv, JWTPublicKeyPath: pub, JWTAccessTTL: "15m"}
}

func newTokens(t *testing.T, cfg *config.Config, rdb *redis.Client) TokenService {
	t.Helper()
	svc, err := NewTokenService(cfg, rdb)
	require.NoError(t, err)
	return svc
}

func loadKey(t *testing.T, path string) *rsa.PrivateKey {
	t.Helper()
	key, err := loadRSAPrivateKey(path)
	require.NoError(t, err)
	return key
}

func writeRSAKeys(t *testing.T, pkcs8 bool) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return writeKeyPair(t, key, pkcs8)
}

func writeKeyPair(t *testing.T, key *rsa.PrivateKey, pkcs8 bool) (string, string) {
	t.Helper()
	dir := t.TempDir()
	privPath := filepath.Join(dir, "private.pem")
	pubPath := filepath.Join(dir, "public.pem")

	var block *pem.Block
	if pkcs8 {
		der, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		block = &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	} else {
		block = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	}
	require.NoError(t, os.WriteFile(privPath, pem.EncodeToMemory(block), 0o600))
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o600))
	return privPath, pubPath
}
