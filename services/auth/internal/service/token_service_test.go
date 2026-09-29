package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/model"
)

func TestTokenServiceRoundTrip(t *testing.T) {
	priv, pub := writeRSAKeys(t, false)
	svc := newTokens(t, &config.Config{
		JWTPrivateKeyPath: priv,
		JWTPublicKeyPath:  pub,
		JWTAccessTTL:      "15m",
	})
	userID := mustID(t)
	raw, err := svc.GenerateAccessToken(userID, string(model.RolePersonal))
	require.NoError(t, err)
	claims, err := svc.ValidateAccessToken(raw)
	require.NoError(t, err)
	require.Equal(t, userID, claims.UserID)
	require.Equal(t, string(model.RolePersonal), claims.Role)
	require.True(t, claims.Exp.After(time.Now()))

	refresh, hash, err := svc.GenerateRefreshToken()
	require.NoError(t, err)
	require.Equal(t, HashToken(refresh), hash)
	require.NotEqual(t, refresh, hash)

	_, err = svc.ValidateAccessToken(raw + "x")
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = svc.ValidateAccessToken("not-a-jwt")
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokenServicePKCS8AndForeignKey(t *testing.T) {
	priv, pub := writeRSAKeys(t, true)
	svc := newTokens(t, &config.Config{JWTPrivateKeyPath: priv, JWTPublicKeyPath: pub, JWTAccessTTL: "15m"})
	userID := mustID(t)
	raw, err := svc.GenerateAccessToken(userID, string(model.RoleAdmin))
	require.NoError(t, err)

	otherPriv, otherPub := writeRSAKeys(t, false)
	other := newTokens(t, &config.Config{JWTPrivateKeyPath: otherPriv, JWTPublicKeyPath: otherPub, JWTAccessTTL: "15m"})
	_, err = other.ValidateAccessToken(raw)
	require.ErrorIs(t, err, ErrInvalidToken)
	_ = svc
}

func TestTokenServiceRejectsBadKeys(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "private.pem")
	pub := filepath.Join(dir, "public.pem")
	require.NoError(t, os.WriteFile(priv, []byte("nope"), 0o600))
	require.NoError(t, os.WriteFile(pub, []byte("nope"), 0o600))

	_, err := NewTokenService(&config.Config{JWTPrivateKeyPath: filepath.Join(dir, "missing.pem"), JWTPublicKeyPath: pub, JWTAccessTTL: "15m"}, nil)
	require.Error(t, err)
	_, err = NewTokenService(&config.Config{JWTPrivateKeyPath: priv, JWTPublicKeyPath: pub, JWTAccessTTL: "15m"}, nil)
	require.Error(t, err)

	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecPriv, err := x509.MarshalPKCS8PrivateKey(ec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(priv, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecPriv}), 0o600))
	ecPub, err := x509.MarshalPKIXPublicKey(&ec.PublicKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pub, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: ecPub}), 0o600))
	_, err = NewTokenService(&config.Config{JWTPrivateKeyPath: priv, JWTPublicKeyPath: pub, JWTAccessTTL: "15m"}, nil)
	require.Error(t, err)

	_, pubRSA := writeRSAKeys(t, false)
	_, err = NewTokenService(&config.Config{JWTPrivateKeyPath: priv, JWTPublicKeyPath: pubRSA, JWTAccessTTL: "nope"}, nil)
	require.Error(t, err)
	_, err = NewTokenService(&config.Config{JWTPrivateKeyPath: priv, JWTPublicKeyPath: pubRSA, JWTAccessTTL: "0s"}, nil)
	require.Error(t, err)
}

func TestTokenServiceBlacklistFailsClosedWithoutRedis(t *testing.T) {
	priv, pub := writeRSAKeys(t, false)
	svc := newTokens(t, &config.Config{JWTPrivateKeyPath: priv, JWTPublicKeyPath: pub, JWTAccessTTL: "15m"})
	err := svc.BlacklistToken(context.Background(), "token", time.Minute)
	require.Error(t, err)
	_, err = svc.IsBlacklisted(context.Background(), "token")
	require.Error(t, err)
}

func newTokens(t *testing.T, cfg *config.Config) TokenService {
	t.Helper()
	svc, err := NewTokenService(cfg, nil)
	require.NoError(t, err)
	return svc
}

func writeRSAKeys(t *testing.T, pkcs8 bool) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
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
