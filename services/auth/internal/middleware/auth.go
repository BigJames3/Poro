// Package middleware holds HTTP middleware for the auth service.
package middleware

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/poro/auth/internal/service"
)

const (
	// LocalUserID is the Fiber local key for the authenticated account ID.
	LocalUserID = "userID"
	// LocalRole is the Fiber local key for the authenticated role.
	LocalRole = "role"
	// LocalAccessToken is the Fiber local key for the raw bearer token.
	LocalAccessToken = "accessToken"
)

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
		blacklisted, err := tokens.IsBlacklisted(c.UserContext(), raw)
		if err != nil {
			return fmt.Errorf("check access token blacklist: %w", err)
		}
		if blacklisted {
			return unauthorized(errors.New("access token is blacklisted"))
		}

		c.Locals(LocalUserID, claims.UserID)
		c.Locals(LocalRole, claims.Role)
		c.Locals(LocalAccessToken, raw)
		return c.Next()
	}
}

// UserID reads the authenticated account ID stored by NewAuth.
func UserID(c *fiber.Ctx) (uuid.UUID, bool) {
	id, ok := c.Locals(LocalUserID).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// AccessToken reads the raw bearer token stored by NewAuth.
func AccessToken(c *fiber.Ctx) (string, bool) {
	token, ok := c.Locals(LocalAccessToken).(string)
	return token, ok && token != ""
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
