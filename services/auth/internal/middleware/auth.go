// Package middleware holds HTTP middleware for the auth service.
package middleware

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/poro/auth/internal/service"
)

// LocalClaims is the Fiber local key for the verified *service.TokenClaims.
const LocalClaims = "authClaims"

// NewAuth requires a valid access token that is not blacklisted.
// A blacklist check that cannot reach Redis fails closed with an internal error.
func NewAuth(tokens service.TokenService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw, err := bearerToken(c.Get(fiber.HeaderAuthorization))
		if err != nil {
			return unauthorized(err)
		}

		claims, err := tokens.ValidateAccessToken(raw)
		if err != nil {
			return unauthorized(err)
		}
		blacklisted, err := tokens.IsBlacklisted(c.UserContext(), claims.TokenID)
		if err != nil {
			return fmt.Errorf("check access token blacklist: %w", err)
		}
		if blacklisted {
			return unauthorized(errors.New("access token is blacklisted"))
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
