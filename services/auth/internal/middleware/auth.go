// Package middleware holds HTTP middleware for the auth service.
package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
	"github.com/poro/auth/internal/service"
)

// AccountLookup loads the account behind a token so its status is checked on every request.
type AccountLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

// LocalClaims is the Fiber local key for the verified *service.TokenClaims.
const LocalClaims = "authClaims"

// NewAuth requires a valid access token whose session was not revoked
// (ADR-0016), for an account that still exists and is neither suspended nor
// deleted: an access token issued before a suspension stops working at once.
// A registry or database that cannot be read fails closed with 503, as in
// every other service.
func NewAuth(tokens service.TokenService, accounts AccountLookup) fiber.Handler {
	if accounts == nil {
		panic("middleware.NewAuth: an account lookup is required")
	}
	unavailable := httpx.NewAPIError(fiber.StatusServiceUnavailable, "unavailable", "authentication temporarily unavailable")
	blockedAccount := httpx.NewAPIError(fiber.StatusForbidden, "account_blocked", "account blocked")
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
		user, err := accounts.GetByID(c.UserContext(), claims.UserID)
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
			return unauthorized(errors.New("account not found"))
		case err != nil:
			return unavailable.WithCause(err)
		case user.Status == model.StatusSuspended || user.Status == model.StatusDeleted:
			return blockedAccount
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
