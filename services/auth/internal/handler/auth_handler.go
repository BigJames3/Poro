// Package handler exposes the auth HTTP endpoints.
package handler

import (
	"errors"
	"math"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/service"
)

// AuthHandler adapts the auth service to HTTP.
type AuthHandler struct {
	auth     service.AuthService
	validate *validator.Validate
}

// NewAuthHandler returns an auth handler with a shared validator.
func NewAuthHandler(auth service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth, validate: validator.New()}
}

// RequestOTP handles POST /api/v1/auth/otp/request.
func (h *AuthHandler) RequestOTP(c *fiber.Ctx) error {
	var req dto.RequestOTPRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	res, err := h.auth.RequestOTP(c.UserContext(), req)
	if err != nil {
		var throttled *service.ThrottledError
		if errors.As(err, &throttled) {
			c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(math.Ceil(throttled.RetryAfter.Seconds()))))
		}
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
		return middleware.NewAPIError(fiber.StatusBadRequest, "invalid_json", "invalid json")
	}
	if err := validate.Struct(dst); err != nil {
		return middleware.NewAPIError(fiber.StatusBadRequest, "invalid_request", "invalid request")
	}
	return nil
}

func writeData(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"data":  data,
		"error": nil,
		"meta":  fiber.Map{"request_id": middleware.RequestIDFrom(c)},
	})
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, service.ErrEmailTaken):
		return middleware.NewAPIError(fiber.StatusConflict, "email_taken", "email already registered")
	case errors.Is(err, service.ErrAccountBlocked):
		return middleware.NewAPIError(fiber.StatusForbidden, "account_blocked", "account blocked")
	case errors.Is(err, service.ErrAccountNotFound):
		return middleware.NewAPIError(fiber.StatusNotFound, "account_not_found", "account not found")
	case errors.Is(err, service.ErrOTPTooManyAttempts):
		return middleware.NewAPIError(fiber.StatusTooManyRequests, "otp_too_many_attempts", "too many attempts, request a new code")
	case errors.Is(err, service.ErrOTPThrottled):
		return middleware.NewAPIError(fiber.StatusTooManyRequests, "otp_throttled", "too many code requests")
	case errors.Is(err, service.ErrOTPExpired):
		return middleware.NewAPIError(fiber.StatusUnauthorized, "otp_expired", "code expired")
	case errors.Is(err, service.ErrOTPInvalid):
		return middleware.NewAPIError(fiber.StatusUnauthorized, "otp_invalid", "invalid code")
	case errors.Is(err, service.ErrInvalidCredentials):
		return middleware.NewAPIError(fiber.StatusUnauthorized, "invalid_credentials", "invalid credentials")
	case errors.Is(err, service.ErrRefreshTokenReused):
		return middleware.NewAPIError(fiber.StatusUnauthorized, "session_revoked", "session revoked, sign in again")
	case errors.Is(err, service.ErrRefreshTokenInvalid):
		return middleware.NewAPIError(fiber.StatusUnauthorized, "refresh_token_invalid", "invalid refresh token")
	case errors.Is(err, service.ErrInvalidToken):
		return middleware.NewAPIError(fiber.StatusUnauthorized, "unauthorized", "unauthorized")
	default:
		return err
	}
}
