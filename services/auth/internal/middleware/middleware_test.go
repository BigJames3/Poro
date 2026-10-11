package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/auth/internal/service"
)

func TestNewAuth(t *testing.T) {
	claims := &service.TokenClaims{UserID: uuid.Must(uuid.NewV7()), SessionID: uuid.Must(uuid.NewV7()), TokenID: "jti", Exp: time.Now().Add(time.Minute)}
	tokens := &stubTokens{claims: claims}
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Get("/me", NewAuth(tokens), func(c *fiber.Ctx) error {
		got, ok := Claims(c)
		require.True(t, ok)
		require.Equal(t, claims, got)
		return c.SendStatus(fiber.StatusOK)
	})

	cases := []struct {
		name   string
		header string
		setup  func()
		status int
	}{
		{name: "valid", header: "Bearer good", status: http.StatusOK},
		{name: "lowercase scheme", header: "bearer good", status: http.StatusOK},
		{name: "missing", header: "", status: http.StatusUnauthorized},
		{name: "wrong scheme", header: "Basic good", status: http.StatusUnauthorized},
		{name: "invalid token", header: "Bearer bad", status: http.StatusUnauthorized},
		{name: "blacklisted", header: "Bearer good", setup: func() { tokens.blacklisted = true }, status: http.StatusUnauthorized},
		{name: "redis down fails closed", header: "Bearer good", setup: func() { tokens.blacklistErr = errors.New("redis") }, status: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens.blacklisted, tokens.blacklistErr = false, nil
			if tc.setup != nil {
				tc.setup()
			}
			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			if tc.header != "" {
				req.Header.Set(fiber.HeaderAuthorization, tc.header)
			}
			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			require.Equal(t, tc.status, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})
	}
	require.Equal(t, "jti", tokens.checkedID, "the blacklist is keyed by jti")
}

type stubTokens struct {
	service.TokenService
	claims       *service.TokenClaims
	blacklisted  bool
	blacklistErr error
	checkedID    string
}

func (s *stubTokens) ValidateAccessToken(token string) (*service.TokenClaims, error) {
	if token != "good" {
		return nil, service.ErrInvalidToken
	}
	return s.claims, nil
}

func (s *stubTokens) IsBlacklisted(_ context.Context, tokenID string) (bool, error) {
	s.checkedID = tokenID
	return s.blacklisted, s.blacklistErr
}
