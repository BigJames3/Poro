package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/poro/auth/internal/service"
)

func TestNewAuth(t *testing.T) {
	claims := &service.TokenClaims{UserID: uuid.Must(uuid.NewV7()), SessionID: uuid.Must(uuid.NewV7()), TokenID: "jti", Exp: time.Now().Add(time.Minute)}
	tokens := &stubTokens{claims: claims}
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
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
		})
	}
	require.Equal(t, "jti", tokens.checkedID, "the blacklist is keyed by jti")
}

func TestRequestID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(RequestIDFrom(c)) })

	cases := []struct {
		name     string
		incoming string
		keep     bool
	}{
		{name: "kept", incoming: "abc-123_X.9", keep: true},
		{name: "generated when missing", incoming: ""},
		{name: "replaced when unsafe", incoming: "evil\"id<script>"},
		{name: "replaced when too long", incoming: strings.Repeat("a", 129)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.incoming != "" {
				req.Header.Set(HeaderRequestID, tc.incoming)
			}
			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			got := resp.Header.Get(HeaderRequestID)
			if tc.keep {
				require.Equal(t, tc.incoming, got)
				return
			}
			_, err = uuid.Parse(got)
			require.NoError(t, err)
		})
	}
}

func TestAccessLogRecordsResolvedStatus(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core)
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(log)})
	app.Use(RequestID(), AccessLog(log))
	app.Get("/conflict", func(*fiber.Ctx) error { return NewAPIError(fiber.StatusConflict, "email_taken", "taken") })
	app.Get("/boom", func(*fiber.Ctx) error { return errors.New("db down") })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/conflict", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/boom", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var statuses []int64
	for _, entry := range logs.FilterMessage("http request").All() {
		statuses = append(statuses, entry.ContextMap()["status"].(int64))
		require.NotEmpty(t, entry.ContextMap()["request_id"])
	}
	require.Equal(t, []int64{409, 500}, statuses)
	require.Equal(t, 1, logs.FilterMessage("request failed").Len(), "only 5xx errors are logged with detail")
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
