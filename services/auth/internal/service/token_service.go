// Package service implements the auth business logic.
package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/poro/auth/internal/config"
)

const (
	// TokenIssuer is the iss claim of every access token.
	TokenIssuer = "poro-auth" //nolint:gosec // G101: the public iss claim, not a credential
	// TokenAudience is the aud claim every Poro API must require.
	TokenAudience = "poro-api"

	minRSAKeyBits = 2048
	// revocationMargin keeps a revoked session in the registry a little
	// longer than its last access token, to absorb clock skew between hosts.
	revocationMargin = time.Minute
)

// ErrInvalidToken is returned when an access token fails validation.
var ErrInvalidToken = errors.New("invalid token")

// TokenClaims is the verified identity carried by an access token.
type TokenClaims struct {
	UserID    uuid.UUID
	Roles     []string
	SessionID uuid.UUID // refresh token family of the device session
	TokenID   string    // jti
	Exp       time.Time
}

// JWK is the public RSA signing key in RFC 7517 form.
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKSet is the body of the JWKS endpoint.
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// SessionRegistry is the revocation registry shared by every service
// (ADR-0016); *revocation.Store implements it.
type SessionRegistry interface {
	Revoke(ctx context.Context, ttl time.Duration, sessionIDs ...uuid.UUID) error
	IsRevoked(ctx context.Context, sessionID uuid.UUID) (bool, error)
}

// TokenService issues and checks access tokens, and revokes device sessions in
// the shared registry so that every service refuses their access tokens.
type TokenService interface {
	GenerateAccessToken(userID uuid.UUID, roles []string, sessionID uuid.UUID) (string, error)
	GenerateRefreshToken() (raw string, hash string, err error)
	ValidateAccessToken(token string) (*TokenClaims, error)
	HashToken(token string) string
	RevokeSessions(ctx context.Context, sessionIDs ...uuid.UUID) error
	IsSessionRevoked(ctx context.Context, sessionID uuid.UUID) (bool, error)
	JWKS() JWKSet
}

type tokenService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	jwk        JWK
	accessTTL  time.Duration
	registry   SessionRegistry
}

type accessClaims struct {
	Roles     []string `json:"roles"`
	SessionID string   `json:"sid"`
	jwt.RegisteredClaims
}

// NewTokenService loads the RSA key pair and parses the access-token lifetime.
// The private and public keys must match. A nil registry still allows
// signing; revocations and revocation checks then fail closed.
func NewTokenService(cfg *config.Config, registry SessionRegistry) (TokenService, error) {
	accessTTL, err := time.ParseDuration(cfg.JWTAccessTTL)
	if err != nil {
		return nil, fmt.Errorf("parse access token ttl: %w", err)
	}
	if accessTTL <= 0 {
		return nil, fmt.Errorf("parse access token ttl: must be positive")
	}

	privateKey, err := loadRSAPrivateKey(cfg.JWTPrivateKeyPath)
	if err != nil {
		return nil, err
	}
	publicKey, err := loadRSAPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		return nil, err
	}
	if privateKey.N.BitLen() < minRSAKeyBits {
		return nil, fmt.Errorf("private key must be at least %d bits", minRSAKeyBits)
	}
	if !privateKey.PublicKey.Equal(publicKey) {
		return nil, fmt.Errorf("public key does not match private key")
	}

	return &tokenService{
		privateKey: privateKey,
		publicKey:  publicKey,
		jwk:        newJWK(publicKey),
		accessTTL:  accessTTL,
		registry:   registry,
	}, nil
}

func (s *tokenService) GenerateAccessToken(userID uuid.UUID, roles []string, sessionID uuid.UUID) (string, error) {
	jti, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("sign access token: new jti: %w", err)
	}
	now := time.Now().UTC()
	claims := accessClaims{
		Roles:     roles,
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti.String(),
			Subject:   userID.String(),
			Issuer:    TokenIssuer,
			Audience:  jwt.ClaimStrings{TokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.jwk.Kid
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

func (s *tokenService) GenerateRefreshToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

func (s *tokenService) ValidateAccessToken(token string) (*TokenClaims, error) {
	claims := &accessClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if kid, ok := t.Header["kid"].(string); ok && kid != s.jwk.Kid {
			return nil, fmt.Errorf("unknown key id")
		}
		return s.publicKey, nil
	},
		jwt.WithIssuer(TokenIssuer),
		jwt.WithAudience(TokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	)
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("validate access token: %w", ErrInvalidToken)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("validate access token: %w", ErrInvalidToken)
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil || claims.ID == "" {
		return nil, fmt.Errorf("validate access token: %w", ErrInvalidToken)
	}
	return &TokenClaims{
		UserID:    userID,
		Roles:     claims.Roles,
		SessionID: sessionID,
		TokenID:   claims.ID,
		Exp:       claims.ExpiresAt.Time,
	}, nil
}

func (s *tokenService) HashToken(token string) string {
	return HashToken(token)
}

// RevokeSessions records sessions in the shared registry for as long as their
// access tokens can live: every service refuses those tokens from now on.
func (s *tokenService) RevokeSessions(ctx context.Context, sessionIDs ...uuid.UUID) error {
	if s.registry == nil {
		return errors.New("revoke sessions: revocation registry is not configured")
	}
	if err := s.registry.Revoke(ctx, s.accessTTL+revocationMargin, sessionIDs...); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}

// IsSessionRevoked reports whether a session is in the shared registry.
func (s *tokenService) IsSessionRevoked(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	if s.registry == nil {
		return false, errors.New("check session revocation: revocation registry is not configured")
	}
	revoked, err := s.registry.IsRevoked(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("check session revocation: %w", err)
	}
	return revoked, nil
}

func (s *tokenService) JWKS() JWKSet {
	return JWKSet{Keys: []JWK{s.jwk}}
}

// HashToken returns the hex SHA-256 of a raw token. The raw value is never logged.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newJWK builds the public JWK. The kid is the RFC 7638 thumbprint of the key.
func newJWK(key *rsa.PublicKey) JWK {
	n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
	thumb := sha256.Sum256([]byte(`{"e":"` + e + `","kty":"RSA","n":"` + n + `"}`))
	return JWK{
		Kty: "RSA",
		Use: "sig",
		Alg: jwt.SigningMethodRS256.Alg(),
		Kid: base64.RawURLEncoding.EncodeToString(thumb[:]),
		N:   n,
		E:   e,
	}
}

func loadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	pemBytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("read private key: not a PEM block")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("parse private key: not an RSA key")
	}
	return key, nil
}

func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	pemBytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("read public key: not a PEM block")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("parse public key: not an RSA key")
	}
	return key, nil
}
