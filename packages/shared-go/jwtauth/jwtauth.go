// Package jwtauth verifies Poro access tokens locally against the auth service JWKS.
//
// Signatures are checked offline. With a revocation registry (ADR-0016), the
// tokens of a session revoked by auth (logout, "sign out other devices",
// password reset, account deletion, stolen refresh token) are refused too.
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

// ErrRevocationUnavailable is returned when the revocation registry cannot be
// read. The request is refused with 503: a revoked session is never accepted.
var ErrRevocationUnavailable = errors.New("revocation registry unavailable")

// Revocations tells whether a session was revoked.
type Revocations interface {
	IsRevoked(ctx context.Context, sessionID uuid.UUID) (bool, error)
}

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
	// Revocations, when set, refuses the tokens of revoked sessions. Only a
	// local development setup may leave it nil.
	Revocations Revocations
}

// Verifier checks RS256 signature, issuer, audience, expiry and, with a
// registry, revocation.
type Verifier struct {
	keys        *keySet
	parser      *jwt.Parser
	revocations Revocations
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
		keys:        &keySet{url: cfg.JWKSURL, client: client, maxAge: maxAge, minGap: defaultMinGap, now: time.Now},
		revocations: cfg.Revocations,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
			jwt.WithIssuer(Issuer),
			jwt.WithAudience(Audience),
			jwt.WithExpirationRequired(),
		),
	}, nil
}

// Verify returns the claims of a valid token. It returns ErrKeysUnavailable
// when the signing keys cannot be loaded, ErrRevocationUnavailable when the
// revocation registry cannot be read, and ErrInvalidToken otherwise.
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
	if v.revocations != nil {
		revoked, err := v.revocations.IsRevoked(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrRevocationUnavailable, err)
		}
		if revoked {
			return nil, fmt.Errorf("%w: session revoked", ErrInvalidToken)
		}
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
	if errors.Is(err, ErrKeysUnavailable) || errors.Is(err, ErrRevocationUnavailable) {
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
