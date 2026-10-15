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

	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/service"
)

func accountApp(t *testing.T, accounts service.AccountService, claims *service.TokenClaims) *fiber.App {
	t.Helper()
	h := NewAccountHandler(accounts)
	app := newTestApp()
	if claims != nil {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals(middleware.LocalClaims, claims)
			return c.Next()
		})
	}
	app.Get("/sessions", h.Sessions)
	app.Post("/sessions/revoke-others", h.RevokeOtherSessions)
	app.Delete("/sessions/:id", h.RevokeSession)
	app.Post("/me/delete", h.DeleteAccount)
	app.Post("/me/delete/cancel", h.CancelDeletion)
	return app
}

func TestAccountHandlerSuccess(t *testing.T) {
	claims := &service.TokenClaims{UserID: mustUUID(t), SessionID: mustUUID(t), TokenID: "jti-1", Exp: time.Now().Add(time.Minute)}
	device := "Tecno Spark 20"
	scheduled := "2026-11-11T09:00:00Z"
	var revoked uuid.UUID
	var deleteReq dto.DeleteAccountRequest
	accounts := &stubAccounts{
		sessions: func(_ context.Context, c service.TokenClaims) ([]dto.SessionResponse, error) {
			require.Equal(t, *claims, c)
			return []dto.SessionResponse{{ID: c.SessionID.String(), Device: &device, Current: true}}, nil
		},
		revoke: func(_ context.Context, _ service.TokenClaims, id uuid.UUID) error {
			revoked = id
			return nil
		},
		revokeOthers: func(context.Context, service.TokenClaims) (*dto.RevokeSessionsResponse, error) {
			return &dto.RevokeSessionsResponse{Revoked: 2}, nil
		},
		deleteAccount: func(_ context.Context, _ service.TokenClaims, req dto.DeleteAccountRequest) (*dto.DeletionResponse, error) {
			deleteReq = req
			return &dto.DeletionResponse{DeletionScheduledAt: scheduled}, nil
		},
		cancel: func(context.Context, service.TokenClaims) (*dto.UserResponse, error) {
			return &dto.UserResponse{ID: claims.UserID.String()}, nil
		},
	}
	app := accountApp(t, accounts, claims)

	resp := do(t, app, http.MethodGet, "/sessions", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	sessions := decodeBody(t, resp)["data"].([]any)
	require.Equal(t, "Tecno Spark 20", sessions[0].(map[string]any)["device"])
	require.Equal(t, true, sessions[0].(map[string]any)["current"])

	target := mustUUID(t)
	resp = do(t, app, http.MethodDelete, "/sessions/"+target.String(), "", nil)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, target, revoked)

	resp = do(t, app, http.MethodPost, "/sessions/revoke-others", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.EqualValues(t, 2, decodeBody(t, resp)["data"].(map[string]any)["revoked"])

	resp = do(t, app, http.MethodPost, "/me/delete", `{"password":"password1"}`, nil)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.Equal(t, scheduled, decodeBody(t, resp)["data"].(map[string]any)["deletion_scheduled_at"])
	require.Equal(t, "password1", deleteReq.Password)

	resp = do(t, app, http.MethodPost, "/me/delete/cancel", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := decodeBody(t, resp)["data"].(map[string]any)
	require.Contains(t, data, "deletion_scheduled_at")
	require.Nil(t, data["deletion_scheduled_at"])
}

func TestAccountHandlerErrors(t *testing.T) {
	claims := &service.TokenClaims{UserID: mustUUID(t), SessionID: mustUUID(t)}
	fail := func(err error) *stubAccounts {
		return &stubAccounts{
			sessions: func(context.Context, service.TokenClaims) ([]dto.SessionResponse, error) { return nil, err },
			revoke:   func(context.Context, service.TokenClaims, uuid.UUID) error { return err },
			revokeOthers: func(context.Context, service.TokenClaims) (*dto.RevokeSessionsResponse, error) {
				return nil, err
			},
			deleteAccount: func(context.Context, service.TokenClaims, dto.DeleteAccountRequest) (*dto.DeletionResponse, error) {
				return nil, err
			},
			cancel: func(context.Context, service.TokenClaims) (*dto.UserResponse, error) { return nil, err },
		}
	}

	app := accountApp(t, fail(service.ErrSessionNotFound), claims)
	resp := do(t, app, http.MethodDelete, "/sessions/"+mustUUID(t).String(), "", nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, "session_not_found", errorCode(t, resp))
	resp = do(t, app, http.MethodDelete, "/sessions/not-a-uuid", "", nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, "session_not_found", errorCode(t, resp))

	app = accountApp(t, fail(service.ErrReauthRequired), claims)
	resp = do(t, app, http.MethodPost, "/me/delete", `{}`, nil)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	require.Equal(t, "reauth_required", errorCode(t, resp))
	resp = do(t, app, http.MethodPost, "/me/delete", `{"otp_code":"12ab"}`, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp = do(t, app, http.MethodPost, "/me/delete", `{`, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	app = accountApp(t, fail(service.ErrInvalidCredentials), claims)
	resp = do(t, app, http.MethodPost, "/me/delete", `{"password":"nope"}`, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, "invalid_credentials", errorCode(t, resp))

	app = accountApp(t, fail(&service.LoginLockedError{RetryAfter: 4 * time.Minute}), claims)
	resp = do(t, app, http.MethodPost, "/me/delete", `{"password":"nope"}`, nil)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	require.Equal(t, "login_throttled", errorCode(t, resp))
	require.Equal(t, "240", resp.Header.Get(fiber.HeaderRetryAfter))

	app = accountApp(t, fail(service.ErrAccountNotFound), claims)
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/sessions"},
		{http.MethodPost, "/sessions/revoke-others"},
		{http.MethodPost, "/me/delete/cancel"},
	} {
		resp = do(t, app, call.method, call.path, "", nil)
		require.Equal(t, http.StatusNotFound, resp.StatusCode, call.path)
	}

	app = accountApp(t, fail(errors.New("db down")), claims)
	resp = do(t, app, http.MethodGet, "/sessions", "", nil)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	anonymous := accountApp(t, fail(nil), nil)
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/sessions"},
		{http.MethodPost, "/sessions/revoke-others"},
		{http.MethodDelete, "/sessions/" + mustUUID(t).String()},
		{http.MethodPost, "/me/delete"},
		{http.MethodPost, "/me/delete/cancel"},
	} {
		resp = do(t, anonymous, call.method, call.path, "", nil)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, call.path)
	}
}

type stubAccounts struct {
	sessions      func(context.Context, service.TokenClaims) ([]dto.SessionResponse, error)
	revoke        func(context.Context, service.TokenClaims, uuid.UUID) error
	revokeOthers  func(context.Context, service.TokenClaims) (*dto.RevokeSessionsResponse, error)
	deleteAccount func(context.Context, service.TokenClaims, dto.DeleteAccountRequest) (*dto.DeletionResponse, error)
	cancel        func(context.Context, service.TokenClaims) (*dto.UserResponse, error)
}

func (s *stubAccounts) Sessions(ctx context.Context, c service.TokenClaims) ([]dto.SessionResponse, error) {
	return s.sessions(ctx, c)
}
func (s *stubAccounts) RevokeSession(ctx context.Context, c service.TokenClaims, id uuid.UUID) error {
	return s.revoke(ctx, c, id)
}
func (s *stubAccounts) RevokeOtherSessions(ctx context.Context, c service.TokenClaims) (*dto.RevokeSessionsResponse, error) {
	return s.revokeOthers(ctx, c)
}
func (s *stubAccounts) DeleteAccount(ctx context.Context, c service.TokenClaims, req dto.DeleteAccountRequest) (*dto.DeletionResponse, error) {
	return s.deleteAccount(ctx, c, req)
}
func (s *stubAccounts) CancelDeletion(ctx context.Context, c service.TokenClaims) (*dto.UserResponse, error) {
	return s.cancel(ctx, c)
}
