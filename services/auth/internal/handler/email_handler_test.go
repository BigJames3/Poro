package handler

import (
	"context"
	"errors"
	"net/http"
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

func emailApp(t *testing.T, emails service.EmailService, claims *service.TokenClaims) *fiber.App {
	t.Helper()
	h := NewEmailHandler(emails)
	app := newTestApp()
	withClaims := func(c *fiber.Ctx) error {
		if claims != nil {
			c.Locals(middleware.LocalClaims, claims)
		}
		return c.Next()
	}
	app.Post("/email/verification/request", withClaims, h.RequestVerification)
	app.Post("/email/verification/confirm", withClaims, h.ConfirmVerification)
	app.Post("/email/password/forgot", h.ForgotPassword)
	app.Post("/email/password/reset", h.ResetPassword)
	return app
}

func TestEmailHandlerSuccess(t *testing.T) {
	claims := &service.TokenClaims{UserID: mustUUID(t), SessionID: mustUUID(t)}
	var confirmed, forgot string
	var reset dto.ResetPasswordRequest
	emails := &stubEmails{
		request: func(_ context.Context, id uuid.UUID) (int, error) {
			require.Equal(t, claims.UserID, id)
			return 900, nil
		},
		confirm: func(_ context.Context, _ uuid.UUID, code string) (*dto.UserResponse, error) {
			confirmed = code
			return &dto.UserResponse{ID: claims.UserID.String(), EmailVerified: true}, nil
		},
		forgot: func(_ context.Context, email string) (int, error) {
			forgot = email
			return 900, nil
		},
		reset: func(_ context.Context, req dto.ResetPasswordRequest) error {
			reset = req
			return nil
		},
	}
	app := emailApp(t, emails, claims)

	resp := do(t, app, http.MethodPost, "/email/verification/request", "", nil)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.EqualValues(t, 900, decodeBody(t, resp)["data"].(map[string]any)["expires_in"])

	resp = do(t, app, http.MethodPost, "/email/verification/confirm", `{"code":"123456"}`, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, decodeBody(t, resp)["data"].(map[string]any)["email_verified"])
	require.Equal(t, "123456", confirmed)

	resp = do(t, app, http.MethodPost, "/email/password/forgot", `{"email":"awa@poro.test"}`, nil)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.Equal(t, "awa@poro.test", forgot)

	resp = do(t, app, http.MethodPost, "/email/password/reset", `{"email":"awa@poro.test","code":"654321","new_password":"nouveau-mdp"}`, nil)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, dto.ResetPasswordRequest{Email: "awa@poro.test", Code: "654321", NewPassword: "nouveau-mdp"}, reset)
}

func TestEmailHandlerErrors(t *testing.T) {
	claims := &service.TokenClaims{UserID: mustUUID(t)}
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{&service.ThrottledError{RetryAfter: 30500 * time.Millisecond}, http.StatusTooManyRequests, "email_throttled"},
		{service.ErrEmailMissing, http.StatusUnprocessableEntity, "email_missing"},
		{service.ErrEmailAlreadyVerified, http.StatusConflict, "email_already_verified"},
		{service.ErrEmailCodeInvalid, http.StatusUnprocessableEntity, "code_invalid"},
		{service.ErrEmailCodeExpired, http.StatusUnprocessableEntity, "code_expired"},
		{service.ErrEmailCodeTooManyAttempts, http.StatusTooManyRequests, "code_too_many_attempts"},
		{service.ErrEmailUnavailable, http.StatusServiceUnavailable, "email_unavailable"},
		{service.ErrAccountBlocked, http.StatusForbidden, "account_blocked"},
		{errors.New("db down"), http.StatusInternalServerError, "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			emails := &stubEmails{
				request: func(context.Context, uuid.UUID) (int, error) { return 0, tc.err },
				confirm: func(context.Context, uuid.UUID, string) (*dto.UserResponse, error) { return nil, tc.err },
				forgot:  func(context.Context, string) (int, error) { return 0, tc.err },
				reset:   func(context.Context, dto.ResetPasswordRequest) error { return tc.err },
			}
			app := emailApp(t, emails, claims)
			for _, call := range []struct{ path, body string }{
				{"/email/verification/request", ""},
				{"/email/verification/confirm", `{"code":"123456"}`},
				{"/email/password/forgot", `{"email":"awa@poro.test"}`},
				{"/email/password/reset", `{"email":"awa@poro.test","code":"123456","new_password":"nouveau-mdp"}`},
			} {
				resp := do(t, app, http.MethodPost, call.path, call.body, nil)
				require.Equal(t, tc.status, resp.StatusCode, call.path)
				require.Equal(t, tc.code, errorCode(t, resp), call.path)
				if tc.code == "email_throttled" {
					require.Equal(t, "31", resp.Header.Get(fiber.HeaderRetryAfter))
				}
			}
		})
	}
}

func TestEmailHandlerValidation(t *testing.T) {
	app := emailApp(t, &stubEmails{}, &service.TokenClaims{UserID: mustUUID(t)})
	for _, call := range []struct{ path, body string }{
		{"/email/verification/confirm", `{"code":"12345a"}`},
		{"/email/password/forgot", `{"email":"not-an-email"}`},
		{"/email/password/reset", `{"email":"awa@poro.test","code":"123456","new_password":"short"}`},
		{"/email/password/reset", `{`},
	} {
		resp := do(t, app, http.MethodPost, call.path, call.body, nil)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, call.body)
	}
	anonymous := emailApp(t, &stubEmails{}, nil)
	for _, path := range []string{"/email/verification/request", "/email/verification/confirm"} {
		resp := do(t, anonymous, http.MethodPost, path, `{"code":"123456"}`, nil)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
	}
}

func TestRegisterSendsTheVerificationEmailInTheBackground(t *testing.T) {
	userID := mustUUID(t)
	sent := make(chan uuid.UUID, 1)
	auth := &stubAuth{
		register: func(context.Context, dto.RegisterEmailRequest, string, string) (*dto.AuthResponse, error) {
			return &dto.AuthResponse{User: dto.UserResponse{ID: userID.String()}}, nil
		},
	}
	emails := &stubEmails{
		request: func(ctx context.Context, id uuid.UUID) (int, error) {
			_, hasDeadline := ctx.Deadline()
			require.True(t, hasDeadline, "the background send is bounded")
			sent <- id
			return 0, service.ErrEmailUnavailable
		},
	}
	h := NewAuthHandler(auth).SendVerificationOnSignup(emails, zap.NewNop())
	app := newTestApp()
	app.Post("/email/register", h.RegisterEmail)

	resp := do(t, app, http.MethodPost, "/email/register", `{"email":"awa@poro.test","password":"password1","full_name":"Awa Koné"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "a failing relay does not fail the sign-up")
	select {
	case id := <-sent:
		require.Equal(t, userID, id)
	case <-time.After(5 * time.Second):
		t.Fatal("no verification email after sign-up")
	}
}

type stubEmails struct {
	request func(context.Context, uuid.UUID) (int, error)
	confirm func(context.Context, uuid.UUID, string) (*dto.UserResponse, error)
	forgot  func(context.Context, string) (int, error)
	reset   func(context.Context, dto.ResetPasswordRequest) error
}

func (s *stubEmails) RequestVerification(ctx context.Context, id uuid.UUID) (int, error) {
	return s.request(ctx, id)
}
func (s *stubEmails) ConfirmVerification(ctx context.Context, id uuid.UUID, code string) (*dto.UserResponse, error) {
	return s.confirm(ctx, id, code)
}
func (s *stubEmails) ForgotPassword(ctx context.Context, email string) (int, error) {
	return s.forgot(ctx, email)
}
func (s *stubEmails) ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) error {
	return s.reset(ctx, req)
}
