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
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/poro/auth/internal/config"
)

const (
	tokenIssuer        = "poro-auth"
	blacklistKeyPrefix = "auth:jwt:blacklist:"
)

// ErrInvalidToken is returned when an access token fails validation.
var ErrInvalidToken = errors.New("invalid token")

// TokenClaims is the verified identity carried by an access token.
type TokenClaims struct {
	UserID uuid.UUID
	Role   string
	Exp    time.Time
}

// TokenService issues and checks access tokens, and blacklists them on logout.
type TokenService interface {
	GenerateAccessToken(userID uuid.UUID, role string) (string, error)
	GenerateRefreshToken() (raw string, hash string, err error)
	ValidateAccessToken(token string) (*TokenClaims, error)
	HashToken(token string) string
	BlacklistToken(ctx context.Context, token string, ttl time.Duration) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
}

type tokenService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	accessTTL  time.Duration
	redis      *redis.Client
}

type accessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// NewTokenService loads the RSA key pair and parses the access-token lifetime.
// A nil Redis client still allows signing. Blacklist checks then fail closed.
func NewTokenService(cfg *config.Config, rdb *redis.Client) (TokenService, error) {
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

	return &tokenService{
		privateKey: privateKey,
		publicKey:  publicKey,
		accessTTL:  accessTTL,
		redis:      rdb,
	}, nil
}

func (s *tokenService) GenerateAccessToken(userID uuid.UUID, role string) (string, error) {
	now := time.Now().UTC()
	claims := accessClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    tokenIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
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
		if t.Method != jwt.SigningMethodRS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.publicKey, nil
	}, jwt.WithIssuer(tokenIssuer), jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("validate access token: %w", ErrInvalidToken)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("validate access token: %w", ErrInvalidToken)
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return nil, fmt.Errorf("validate access token: %w", ErrInvalidToken)
	}
	return &TokenClaims{UserID: userID, Role: claims.Role, Exp: exp.Time}, nil
}

func (s *tokenService) HashToken(token string) string {
	return HashToken(token)
}

func (s *tokenService) BlacklistToken(ctx context.Context, token string, ttl time.Duration) error {
	if s.redis == nil {
		return fmt.Errorf("blacklist token: redis is not configured")
	}
	if ttl <= 0 {
		return nil
	}
	if err := s.redis.Set(ctx, blacklistKey(token), "1", ttl).Err(); err != nil {
		return fmt.Errorf("blacklist token: %w", err)
	}
	return nil
}

func (s *tokenService) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	if s.redis == nil {
		return false, fmt.Errorf("check token blacklist: redis is not configured")
	}
	n, err := s.redis.Exists(ctx, blacklistKey(token)).Result()
	if err != nil {
		return false, fmt.Errorf("check token blacklist: %w", err)
	}
	return n > 0, nil
}

// HashToken returns the hex SHA-256 of a raw token. The raw value is never logged.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func blacklistKey(token string) string {
	return blacklistKeyPrefix + HashToken(token)
}

func loadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	pemBytes, err := os.ReadFile(path)
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
	pemBytes, err := os.ReadFile(path)
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
