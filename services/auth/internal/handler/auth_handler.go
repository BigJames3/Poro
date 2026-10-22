// Package handler exposes the auth HTTP endpoints.
package handler

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/service"
)

// signupEmailTimeout bounds the verification email sent after a sign-up.
const signupEmailTimeout = 30 * time.Second

// AuthHandler adapts the auth service to HTTP.
type AuthHandler struct {
	auth     service.AuthService
	validate *validator.Validate
	// afterSignup runs in the background once an email account is created.
	afterSignup func(ctx context.Context, userID uuid.UUID)
}

// NewAuthHandler returns an auth handler with a shared validator.
func NewAuthHandler(auth service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth, validate: validator.New()}
}

// SendVerificationOnSignup emails a verification code after each email
// sign-up, in the background: a slow or failing relay never delays or fails
// the sign-up itself.
func (h *AuthHandler) SendVerificationOnSignup(emails service.EmailService, log *zap.Logger) *AuthHandler {
	h.afterSignup = func(ctx context.Context, userID uuid.UUID) {
		service.SendVerificationAfterSignup(ctx, emails, userID, log)
	}
	return h
}

// RequestOTP handles POST /api/v1/auth/otp/request.
func (h *AuthHandler) RequestOTP(c *fiber.Ctx) error {
	var req dto.RequestOTPRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	res, err := h.auth.RequestOTP(c.UserContext(), req)
	if err != nil {
		setRetryAfter(c, err)
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}

// VerifyOTP handles POST /api/v1/auth/otp/verify.
func (h *AuthHandler) VerifyOTP(c *fiber.Ctx) error {
	var req dto.VerifyOTPRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	res, err := h.auth.VerifyOTP(c.UserContext(), req, c.Get(fiber.HeaderUserAgent), c.IP())
	if err != nil {
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}

// RegisterEmail handles POST /api/v1/auth/email/register.
func (h *AuthHandler) RegisterEmail(c *fiber.Ctx) error {
	var req dto.RegisterEmailRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	res, err := h.auth.RegisterEmail(c.UserContext(), req, c.Get(fiber.HeaderUserAgent), c.IP())
	if err != nil {
		return mapAuthError(err)
	}
	if h.afterSignup != nil {
		if userID, err := uuid.Parse(res.User.ID); err == nil {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(c.UserContext()), signupEmailTimeout)
			go func() {
				defer cancel()
				h.afterSignup(ctx, userID)
			}()
		}
	}
	return writeData(c, fiber.StatusCreated, res)
}

// LoginEmail handles POST /api/v1/auth/email/login.
func (h *AuthHandler) LoginEmail(c *fiber.Ctx) error {
	var req dto.LoginEmailRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	res, err := h.auth.LoginEmail(c.UserContext(), req, c.Get(fiber.HeaderUserAgent), c.IP())
	if err != nil {
		setRetryAfter(c, err)
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req dto.RefreshRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	res, err := h.auth.Refresh(c.UserContext(), req)
	if err != nil {
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}

// Logout handles POST /api/v1/auth/logout. It ends the caller's device session.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	claims, ok := middleware.Claims(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	if err := h.auth.Logout(c.UserContext(), *claims); err != nil {
		return mapAuthError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Me handles GET /api/v1/auth/me.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	claims, ok := middleware.Claims(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	res, err := h.auth.Me(c.UserContext(), claims.UserID)
	if err != nil {
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}

func decode(c *fiber.Ctx, validate *validator.Validate, dst any) error {
	if err := c.BodyParser(dst); err != nil {
		return httpx.NewAPIError(fiber.StatusBadRequest, "invalid_json", "invalid json")
	}
	if err := validate.Struct(dst); err != nil {
		return httpx.NewAPIError(fiber.StatusBadRequest, "invalid_request", "invalid request")
	}
	return nil
}

func writeData(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"data":  data,
		"error": nil,
		"meta":  fiber.Map{"request_id": httpx.RequestIDFrom(c)},
	})
}

// setRetryAfter tells a throttled or locked client when to try again.
func setRetryAfter(c *fiber.Ctx, err error) {
	var wait time.Duration
	var throttled *service.ThrottledError
	var locked *service.LoginLockedError
	switch {
	case errors.As(err, &throttled):
		wait = throttled.RetryAfter
	case errors.As(err, &locked):
		wait = locked.RetryAfter
	default:
		return
	}
	c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(math.Ceil(wait.Seconds()))))
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, service.ErrLoginLocked):
		return httpx.NewAPIError(fiber.StatusTooManyRequests, "login_throttled", "too many failed sign-ins, try again later")
	case errors.Is(err, service.ErrEmailTaken):
		return httpx.NewAPIError(fiber.StatusConflict, "email_taken", "email already registered")
	case errors.Is(err, service.ErrSuspensionForbidden):
		return httpx.NewAPIError(fiber.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, service.ErrSelfSuspension):
		return httpx.NewAPIError(fiber.StatusUnprocessableEntity, "self_suspension", "an administrator cannot suspend their own account")
	case errors.Is(err, service.ErrAccountBlocked):
		return httpx.NewAPIError(fiber.StatusForbidden, "account_blocked", "account blocked")
	case errors.Is(err, service.ErrAccountNotFound):
		return httpx.NewAPIError(fiber.StatusNotFound, "account_not_found", "account not found")
	case errors.Is(err, service.ErrOTPTooManyAttempts):
		return httpx.NewAPIError(fiber.StatusTooManyRequests, "otp_too_many_attempts", "too many attempts, request a new code")
	case errors.Is(err, service.ErrOTPThrottled):
		return httpx.NewAPIError(fiber.StatusTooManyRequests, "otp_throttled", "too many code requests")
	case errors.Is(err, service.ErrPhoneCountryNotSupported):
		return httpx.NewAPIError(fiber.StatusUnprocessableEntity, "phone_country_not_supported", "phone country not supported")
	case errors.Is(err, service.ErrPhoneUnreachable):
		return httpx.NewAPIError(fiber.StatusUnprocessableEntity, "phone_unreachable", "this number cannot receive sms")
	case errors.Is(err, service.ErrSMSUnavailable):
		return httpx.NewAPIError(fiber.StatusServiceUnavailable, "sms_unavailable", "sms delivery unavailable, try again").WithCause(err)
	case errors.Is(err, service.ErrOTPExpired):
		return httpx.NewAPIError(fiber.StatusUnauthorized, "otp_expired", "code expired")
	case errors.Is(err, service.ErrOTPInvalid):
		return httpx.NewAPIError(fiber.StatusUnauthorized, "otp_invalid", "invalid code")
	case errors.Is(err, service.ErrInvalidCredentials):
		return httpx.NewAPIError(fiber.StatusUnauthorized, "invalid_credentials", "invalid credentials")
	case errors.Is(err, service.ErrRefreshTokenReused):
		return httpx.NewAPIError(fiber.StatusUnauthorized, "session_revoked", "session revoked, sign in again")
	case errors.Is(err, service.ErrRefreshTokenInvalid):
		return httpx.NewAPIError(fiber.StatusUnauthorized, "refresh_token_invalid", "invalid refresh token")
	case errors.Is(err, service.ErrSessionNotFound):
		return httpx.NewAPIError(fiber.StatusNotFound, "session_not_found", "session not found")
	case errors.Is(err, service.ErrReauthRequired):
		return httpx.NewAPIError(fiber.StatusUnprocessableEntity, "reauth_required", "confirm with your password or a new code")
	case errors.Is(err, service.ErrInvalidToken):
		return httpx.NewAPIError(fiber.StatusUnauthorized, "unauthorized", "unauthorized")
	default:
		return err
	}
}
