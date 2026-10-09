// Package jwtauth verifies Poro access tokens locally against the auth service JWKS.
//
// Verification is offline: a token revoked by logout stays accepted by other
// services until it expires (15 minutes at most).
package jwtauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/poro/shared-go/httpx"
)

const (
	// Issuer is the iss claim of every Poro access token.
	Issuer = "poro-auth"
	// Audience is the aud claim every Poro API requires.
	Audience = "poro-api"

	localClaims = "jwtClaims"
)

// ErrInvalidToken is returned for any token that must be refused with 401.
var ErrInvalidToken = errors.New("invalid token")

// Claims is the verified identity of the caller.
type Claims struct {
	UserID    uuid.UUID
	Roles     []string
	SessionID uuid.UUID
	TokenID   string
	ExpiresAt time.Time
}

// HasRole reports whether the caller holds role.
func (c *Claims) HasRole(role string) bool { return slices.Contains(c.Roles, role) }

// Config locates the JWKS document.
type Config struct {
	JWKSURL    string
	HTTPClient *http.Client  // defaults to a client with a 5 s timeout
	MaxAge     time.Duration // defaults to 5 minutes, the auth Cache-Control max-age
}

// Verifier checks RS256 signature, issuer, audience and expiry.
type Verifier struct {
	keys   *keySet
	parser *jwt.Parser
}

type accessClaims struct {
	Roles     []string `json:"roles"`
	SessionID string   `json:"sid"`
	jwt.RegisteredClaims
}

// NewVerifier builds a verifier. Keys are fetched lazily, so a service can
// start while auth is still booting.
func NewVerifier(cfg Config) (*Verifier, error) {
	if cfg.JWKSURL == "" {
		return nil, errors.New("jwtauth: JWKS URL is required")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = defaultMaxAge
	}
	return &Verifier{
		keys: &keySet{url: cfg.JWKSURL, client: client, maxAge: maxAge, minGap: defaultMinGap, now: time.Now},
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
			jwt.WithIssuer(Issuer),
			jwt.WithAudience(Audience),
			jwt.WithExpirationRequired(),
		),
	}, nil
}

// Verify returns the claims of a valid token. It returns ErrKeysUnavailable
// when the signing keys cannot be loaded, and ErrInvalidToken otherwise.
func (v *Verifier) Verify(ctx context.Context, token string) (*Claims, error) {
	var keyErr error
	parsed, err := v.parser.ParseWithClaims(token, &accessClaims{}, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		key, err := v.keys.key(ctx, kid)
		if err != nil {
			keyErr = err
			return nil, err
		}
		return key, nil
	})
	if errors.Is(keyErr, ErrKeysUnavailable) {
		return nil, keyErr
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !parsed.Valid {
		return nil, ErrInvalidToken
	}
	c := parsed.Claims.(*accessClaims)
	userID, err := uuid.Parse(c.Subject)
	if err != nil {
		return nil, fmt.Errorf("%w: subject is not a uuid", ErrInvalidToken)
	}
	sessionID, err := uuid.Parse(c.SessionID)
	if err != nil {
		return nil, fmt.Errorf("%w: sid is not a uuid", ErrInvalidToken)
	}
	if c.ID == "" {
		return nil, fmt.Errorf("%w: missing jti", ErrInvalidToken)
	}
	return &Claims{
		UserID:    userID,
		Roles:     c.Roles,
		SessionID: sessionID,
		TokenID:   c.ID,
		ExpiresAt: c.ExpiresAt.Time,
	}, nil
}

// Middleware requires a valid Bearer token and stores the claims for ClaimsFrom.
func Middleware(v *Verifier) fiber.Handler {
	unauthorized := httpx.NewAPIError(fiber.StatusUnauthorized, "unauthorized", "unauthorized")
	unavailable := httpx.NewAPIError(fiber.StatusServiceUnavailable, "unavailable", "authentication temporarily unavailable")
	return func(c *fiber.Ctx) error {
		token, ok := bearerToken(c.Get(fiber.HeaderAuthorization))
		if !ok {
			return unauthorized
		}
		return attachClaims(c, v, token, unauthorized, unavailable)
	}
}

// OptionalMiddleware verifies a Bearer token when present and lets anonymous
// callers through. A malformed or invalid token is still 401.
func OptionalMiddleware(v *Verifier) fiber.Handler {
	unauthorized := httpx.NewAPIError(fiber.StatusUnauthorized, "unauthorized", "unauthorized")
	unavailable := httpx.NewAPIError(fiber.StatusServiceUnavailable, "unavailable", "authentication temporarily unavailable")
	return func(c *fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		if strings.TrimSpace(header) == "" {
			return c.Next()
		}
		token, ok := bearerToken(header)
		if !ok {
			return unauthorized
		}
		return attachClaims(c, v, token, unauthorized, unavailable)
	}
}

func attachClaims(c *fiber.Ctx, v *Verifier, token string, unauthorized, unavailable *httpx.APIError) error {
	claims, err := v.Verify(c.UserContext(), token)
	if errors.Is(err, ErrKeysUnavailable) {
		return unavailable.WithCause(err)
	}
	if err != nil {
		return unauthorized
	}
	c.Locals(localClaims, claims)
	return c.Next()
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

// RequireRole refuses callers without role with 403. Mount after Middleware.
func RequireRole(role string) fiber.Handler {
	forbidden := httpx.NewAPIError(fiber.StatusForbidden, "forbidden", "forbidden")
	return func(c *fiber.Ctx) error {
		claims, ok := ClaimsFrom(c)
		if !ok || !claims.HasRole(role) {
			return forbidden
		}
		return c.Next()
	}
}

// ClaimsFrom returns the claims stored by Middleware.
func ClaimsFrom(c *fiber.Ctx) (*Claims, bool) {
	claims, ok := c.Locals(localClaims).(*Claims)
	return claims, ok
}
