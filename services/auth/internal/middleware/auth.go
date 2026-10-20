// Package middleware holds HTTP middleware for the auth service.
package middleware

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/auth/internal/service"
)

// LocalClaims is the Fiber local key for the verified *service.TokenClaims.
const LocalClaims = "authClaims"

// NewAuth requires a valid access token whose session was not revoked
// (ADR-0016). A registry that cannot be read fails closed with 503, as in
// every other service.
func NewAuth(tokens service.TokenService) fiber.Handler {
	unavailable := httpx.NewAPIError(fiber.StatusServiceUnavailable, "unavailable", "authentication temporarily unavailable")
	return func(c *fiber.Ctx) error {
		raw, err := bearerToken(c.Get(fiber.HeaderAuthorization))
		if err != nil {
			return unauthorized(err)
		}

		claims, err := tokens.ValidateAccessToken(raw)
		if err != nil {
			return unauthorized(err)
		}
		revoked, err := tokens.IsSessionRevoked(c.UserContext(), claims.SessionID)
		if err != nil {
			return unavailable.WithCause(err)
		}
		if revoked {
			return unauthorized(errors.New("session revoked"))
		}

		c.Locals(LocalClaims, claims)
		return c.Next()
	}
}

// Claims reads the verified token claims stored by NewAuth.
func Claims(c *fiber.Ctx) (*service.TokenClaims, bool) {
	claims, ok := c.Locals(LocalClaims).(*service.TokenClaims)
	return claims, ok && claims != nil
}

func bearerToken(header string) (string, error) {
	scheme, token, ok := strings.Cut(header, " ")
	token = strings.TrimSpace(token)
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", errors.New("missing bearer token")
	}
	return token, nil
}

func unauthorized(err error) error {
	return fmt.Errorf("%w: %w", fiber.NewError(fiber.StatusUnauthorized, "unauthorized"), err)
}
