package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/service"
)

func TestAuthHandlerSuccess(t *testing.T) {
	phone := "+14155552671"
	auth := &stubAuth{
		requestOTP: func(context.Context, dto.RequestOTPRequest) (*dto.RequestOTPResponse, error) {
			return &dto.RequestOTPResponse{Message: "otp sent", ExpiresIn: 300}, nil
		},
		verifyOTP: func(_ context.Context, _ dto.VerifyOTPRequest, userAgent, ip string) (*dto.AuthResponse, error) {
			require.Equal(t, "PoroTest", userAgent)
			require.NotEmpty(t, ip)
			return &dto.AuthResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 900}, nil
		},
		register: func(context.Context, dto.RegisterEmailRequest, string, string) (*dto.AuthResponse, error) {
			return &dto.AuthResponse{AccessToken: "access", TokenType: "Bearer"}, nil
		},
		login: func(context.Context, dto.LoginEmailRequest, string, string) (*dto.AuthResponse, error) {
			return &dto.AuthResponse{AccessToken: "access", TokenType: "Bearer"}, nil
		},
		refresh: func(context.Context, dto.RefreshRequest) (*dto.RefreshResponse, error) {
			return &dto.RefreshResponse{AccessToken: "access", RefreshToken: "next", ExpiresIn: 900, TokenType: "Bearer"}, nil
		},
		me: func(context.Context, uuid.UUID) (*dto.UserResponse, error) {
			return &dto.UserResponse{ID: "user-id", Phone: &phone}, nil
		},
	}
	app := authApp(auth)

	resp := do(t, app, http.MethodPost, "/otp/request", `{"phone":"+14155552671"}`, map[string]string{
		middleware.HeaderRequestID: "client-req-1",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "client-req-1", resp.Header.Get(middleware.HeaderRequestID))
	body := decodeBody(t, resp)
	require.Nil(t, body["error"])
	require.Equal(t, "otp sent", body["data"].(map[string]any)["message"])
	require.Equal(t, "client-req-1", body["meta"].(map[string]any)["request_id"])

	resp = do(t, app, http.MethodPost, "/otp/verify", `{"phone":"+14155552671","code":"123456"}`, map[string]string{
		"User-Agent": "PoroTest",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = do(t, app, http.MethodPost, "/email/register", `{"email":"ada@poro.app","password":"password1","full_name":"Ada Lovelace"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = do(t, app, http.MethodPost, "/email/login", `{"email":"ada@poro.app","password":"password1"}`, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = do(t, app, http.MethodPost, "/refresh", `{"refresh_token":"refresh"}`, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuthHandlerValidation(t *testing.T) {
	app := authApp(&stubAuth{})

	resp := do(t, app, http.MethodPost, "/email/login", `{`, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "invalid_json", errorCode(t, resp))

	resp = do(t, app, http.MethodPost, "/email/register", `{"email":"not-an-email","password":"x","full_name":"A"}`, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "not-an-email")
	require.NotContains(t, string(raw), `"password"`)

	resp = do(t, app, http.MethodPost, "/refresh", `{"refresh_token":"`+strings.Repeat("a", 300)+`"}`, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandlerMapsErrors(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{err: service.ErrEmailTaken, status: http.StatusConflict, code: "email_taken"},
		{err: service.ErrAccountBlocked, status: http.StatusForbidden, code: "account_blocked"},
		{err: service.ErrAccountNotFound, status: http.StatusNotFound, code: "account_not_found"},
		{err: service.ErrOTPTooManyAttempts, status: http.StatusTooManyRequests, code: "otp_too_many_attempts"},
		{err: &service.ThrottledError{RetryAfter: time.Minute}, status: http.StatusTooManyRequests, code: "otp_throttled"},
		{err: service.ErrInvalidCredentials, status: http.StatusUnauthorized, code: "invalid_credentials"},
		{err: service.ErrRefreshTokenInvalid, status: http.StatusUnauthorized, code: "refresh_token_invalid"},
		{err: service.ErrRefreshTokenReused, status: http.StatusUnauthorized, code: "session_revoked"},
		{err: service.ErrOTPInvalid, status: http.StatusUnauthorized, code: "otp_invalid"},
		{err: service.ErrOTPExpired, status: http.StatusUnauthorized, code: "otp_expired"},
		{err: service.ErrInvalidToken, status: http.StatusUnauthorized, code: "unauthorized"},
		{err: errors.New("db password=secret down"), status: http.StatusInternalServerError, code: "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			app := authApp(&stubAuth{
				login: func(context.Context, dto.LoginEmailRequest, string, string) (*dto.AuthResponse, error) {
					return nil, tc.err
				},
			})
			resp := do(t, app, http.MethodPost, "/email/login", `{"email":"ada@poro.app","password":"password1"}`, nil)
			require.Equal(t, tc.status, resp.StatusCode)
			body := decodeBody(t, resp)
			require.Nil(t, body["data"])
			apiErr := body["error"].(map[string]any)
			require.Equal(t, tc.code, apiErr["code"])
			require.NotContains(t, apiErr["message"], "secret", "internal details never reach the client")
			require.NotEmpty(t, body["meta"].(map[string]any)["request_id"])
		})
	}
}

func TestRequestOTPSetsRetryAfter(t *testing.T) {
	app := authApp(&stubAuth{
		requestOTP: func(context.Context, dto.RequestOTPRequest) (*dto.RequestOTPResponse, error) {
			return nil, &service.ThrottledError{RetryAfter: 42500 * time.Millisecond}
		},
	})
	resp := do(t, app, http.MethodPost, "/otp/request", `{"phone":"+14155552671"}`, nil)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	require.Equal(t, "43", resp.Header.Get(fiber.HeaderRetryAfter))
}

func TestLogoutAndMe(t *testing.T) {
	claims := &service.TokenClaims{UserID: mustUUID(t), SessionID: mustUUID(t), TokenID: "jti-1", Exp: time.Now().Add(time.Minute)}
	var gotClaims service.TokenClaims
	auth := &stubAuth{
		logout: func(_ context.Context, c service.TokenClaims) error {
			gotClaims = c
			return nil
		},
		me: func(_ context.Context, id uuid.UUID) (*dto.UserResponse, error) {
			require.Equal(t, claims.UserID, id)
			return &dto.UserResponse{ID: id.String(), Roles: []string{"PERSONAL", "CREATOR"}}, nil
		},
	}
	h := NewAuthHandler(auth)
	app := newTestApp()
	withClaims := func(c *fiber.Ctx) error {
		c.Locals(middleware.LocalClaims, claims)
		return c.Next()
	}
	app.Post("/logout", withClaims, h.Logout)
	app.Post("/logout-anon", h.Logout)
	app.Get("/me", withClaims, h.Me)
	app.Get("/me-anon", h.Me)

	resp := do(t, app, http.MethodPost, "/logout", "", nil)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, *claims, gotClaims)

	resp = do(t, app, http.MethodPost, "/logout-anon", "", nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp = do(t, app, http.MethodGet, "/me", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := decodeBody(t, resp)["data"].(map[string]any)
	require.Equal(t, []any{"PERSONAL", "CREATOR"}, data["roles"])

	resp = do(t, app, http.MethodGet, "/me-anon", "", nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHealthProbes(t *testing.T) {
	app := newTestApp()
	h := NewHealthHandler(nil, nil, "1.1.0")
	app.Get("/health/live", h.Live)
	app.Get("/health/ready", h.Ready)

	resp := do(t, app, http.MethodGet, "/health/live", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = do(t, app, http.MethodGet, "/health/ready", "", nil)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	body := decodeBody(t, resp)
	require.Equal(t, "degraded", body["status"])
	require.Equal(t, "auth", body["service"])
	require.Equal(t, "1.1.0", body["version"])
	checks := body["checks"].(map[string]any)
	require.Equal(t, "down", checks["postgres"])
	require.Equal(t, "down", checks["redis"])
}

func TestJWKSHandler(t *testing.T) {
	app := newTestApp()
	app.Get("/jwks", NewJWKSHandler(stubTokens{}).JWKS)
	resp := do(t, app, http.MethodGet, "/jwks", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "public, max-age=300", resp.Header.Get(fiber.HeaderCacheControl))
	body := decodeBody(t, resp)
	keys := body["keys"].([]any)
	require.Equal(t, "test-kid", keys[0].(map[string]any)["kid"])
}

func newTestApp() *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	app.Use(middleware.RequestID())
	return app
}

func authApp(auth service.AuthService) *fiber.App {
	h := NewAuthHandler(auth)
	app := newTestApp()
	app.Post("/otp/request", h.RequestOTP)
	app.Post("/otp/verify", h.VerifyOTP)
	app.Post("/email/register", h.RegisterEmail)
	app.Post("/email/login", h.LoginEmail)
	app.Post("/refresh", h.Refresh)
	return app
}

func do(t *testing.T, app *fiber.App, method, path, payload string, headers map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(payload))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	return resp
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}

func errorCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	return decodeBody(t, resp)["error"].(map[string]any)["code"].(string)
}

func mustUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err)
	return id
}

type stubAuth struct {
	requestOTP func(context.Context, dto.RequestOTPRequest) (*dto.RequestOTPResponse, error)
	verifyOTP  func(context.Context, dto.VerifyOTPRequest, string, string) (*dto.AuthResponse, error)
	register   func(context.Context, dto.RegisterEmailRequest, string, string) (*dto.AuthResponse, error)
	login      func(context.Context, dto.LoginEmailRequest, string, string) (*dto.AuthResponse, error)
	refresh    func(context.Context, dto.RefreshRequest) (*dto.RefreshResponse, error)
	logout     func(context.Context, service.TokenClaims) error
	me         func(context.Context, uuid.UUID) (*dto.UserResponse, error)
}

func (s *stubAuth) RequestOTP(ctx context.Context, req dto.RequestOTPRequest) (*dto.RequestOTPResponse, error) {
	return s.requestOTP(ctx, req)
}
func (s *stubAuth) VerifyOTP(ctx context.Context, req dto.VerifyOTPRequest, userAgent, ip string) (*dto.AuthResponse, error) {
	return s.verifyOTP(ctx, req, userAgent, ip)
}
func (s *stubAuth) RegisterEmail(ctx context.Context, req dto.RegisterEmailRequest, userAgent, ip string) (*dto.AuthResponse, error) {
	return s.register(ctx, req, userAgent, ip)
}
func (s *stubAuth) LoginEmail(ctx context.Context, req dto.LoginEmailRequest, userAgent, ip string) (*dto.AuthResponse, error) {
	return s.login(ctx, req, userAgent, ip)
}
func (s *stubAuth) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error) {
	return s.refresh(ctx, req)
}
func (s *stubAuth) Logout(ctx context.Context, claims service.TokenClaims) error {
	return s.logout(ctx, claims)
}
func (s *stubAuth) Me(ctx context.Context, userID uuid.UUID) (*dto.UserResponse, error) {
	return s.me(ctx, userID)
}

type stubTokens struct{ service.TokenService }

func (stubTokens) JWKS() service.JWKSet {
	return service.JWKSet{Keys: []service.JWK{{Kty: "RSA", Kid: "test-kid"}}}
}
