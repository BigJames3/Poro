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

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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
			return &dto.RefreshResponse{AccessToken: "access", RefreshToken: "next", ExpiresIn: 900}, nil
		},
		logout: func(context.Context, string, string) error { return nil },
		me: func(context.Context, uuid.UUID) (*dto.UserResponse, error) {
			return &dto.UserResponse{ID: "user-id", Phone: &phone}, nil
		},
	}
	app := authApp(auth)

	resp := do(t, app, http.MethodPost, "/otp/request", `{"phone":"+14155552671"}`, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeBody(t, resp)
	require.Nil(t, body["error"])
	require.Equal(t, "otp sent", body["data"].(map[string]any)["message"])

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

	resp = do(t, app, http.MethodPost, "/email/register", `{"email":"not-an-email","password":"x","full_name":"A"}`, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "not-an-email")
	require.NotContains(t, string(raw), `"password"`)
}

func TestAuthHandlerMapsErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "taken", err: service.ErrEmailTaken, status: http.StatusConflict},
		{name: "blocked", err: service.ErrAccountBlocked, status: http.StatusForbidden},
		{name: "missing", err: service.ErrAccountNotFound, status: http.StatusNotFound},
		{name: "otp attempts", err: service.ErrOTPTooManyAttempts, status: http.StatusTooManyRequests},
		{name: "credentials", err: service.ErrInvalidCredentials, status: http.StatusUnauthorized},
		{name: "refresh", err: service.ErrRefreshTokenInvalid, status: http.StatusUnauthorized},
		{name: "otp", err: service.ErrOTPInvalid, status: http.StatusUnauthorized},
		{name: "expired", err: service.ErrOTPExpired, status: http.StatusUnauthorized},
		{name: "token", err: service.ErrInvalidToken, status: http.StatusUnauthorized},
		{name: "internal", err: errors.New("db down"), status: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := authApp(&stubAuth{
				login: func(context.Context, dto.LoginEmailRequest, string, string) (*dto.AuthResponse, error) {
					return nil, tc.err
				},
			})
			resp := do(t, app, http.MethodPost, "/email/login", `{"email":"ada@poro.app","password":"password1"}`, nil)
			require.Equal(t, tc.status, resp.StatusCode)
		})
	}
}

func TestLogoutAndMe(t *testing.T) {
	userID := mustUUID(t)
	var gotAccess string
	var gotRefresh string
	auth := &stubAuth{
		logout: func(_ context.Context, access, refresh string) error {
			gotAccess, gotRefresh = access, refresh
			return nil
		},
		me: func(_ context.Context, id uuid.UUID) (*dto.UserResponse, error) {
			require.Equal(t, userID, id)
			return &dto.UserResponse{ID: id.String()}, nil
		},
	}
	h := NewAuthHandler(auth)
	app := fiber.New()
	app.Post("/logout", func(c *fiber.Ctx) error {
		c.Locals(middleware.LocalAccessToken, "access-token")
		return c.Next()
	}, h.Logout)
	app.Post("/logout-anon", h.Logout)
	app.Get("/me", func(c *fiber.Ctx) error {
		c.Locals(middleware.LocalUserID, userID)
		return c.Next()
	}, h.Me)
	app.Get("/me-anon", h.Me)

	resp := do(t, app, http.MethodPost, "/logout", `{"refresh_token":"refresh-token"}`, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "access-token", gotAccess)
	require.Equal(t, "refresh-token", gotRefresh)

	resp = do(t, app, http.MethodPost, "/logout-anon", `{"refresh_token":"refresh-token"}`, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp = do(t, app, http.MethodGet, "/me", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = do(t, app, http.MethodGet, "/me-anon", "", nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHealthDegraded(t *testing.T) {
	app := fiber.New()
	h := NewHealthHandler(nil, nil, "1.0.0")
	app.Get("/health", h.Health)
	resp := do(t, app, http.MethodGet, "/health", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeBody(t, resp)
	require.Equal(t, "degraded", body["status"])
	require.Equal(t, "auth", body["service"])
	require.Equal(t, "1.0.0", body["version"])
	checks := body["checks"].(map[string]any)
	require.Equal(t, "down", checks["postgres"])
	require.Equal(t, "down", checks["redis"])
}

func authApp(auth service.AuthService) *fiber.App {
	h := NewAuthHandler(auth)
	app := fiber.New()
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
	logout     func(context.Context, string, string) error
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
func (s *stubAuth) Logout(ctx context.Context, accessToken, refreshToken string) error {
	return s.logout(ctx, accessToken, refreshToken)
}
func (s *stubAuth) Me(ctx context.Context, userID uuid.UUID) (*dto.UserResponse, error) {
	return s.me(ctx, userID)
}
