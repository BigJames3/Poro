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
		{name: "revoked session", header: "Bearer good", setup: func() { tokens.revoked = true }, status: http.StatusUnauthorized},
		{name: "registry down fails closed", header: "Bearer good", setup: func() { tokens.revokedErr = errors.New("redis") }, status: http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens.revoked, tokens.revokedErr = false, nil
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
	require.Equal(t, claims.SessionID, tokens.checkedSession, "revocation is checked per session (sid)")
}

type stubTokens struct {
	service.TokenService
	claims         *service.TokenClaims
	revoked        bool
	revokedErr     error
	checkedSession uuid.UUID
}

func (s *stubTokens) ValidateAccessToken(token string) (*service.TokenClaims, error) {
	if token != "good" {
		return nil, service.ErrInvalidToken
	}
	return s.claims, nil
}

func (s *stubTokens) IsSessionRevoked(_ context.Context, sessionID uuid.UUID) (bool, error) {
	s.checkedSession = sessionID
	return s.revoked, s.revokedErr
}
