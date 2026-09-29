// Package handler exposes the auth HTTP endpoints.
package handler

import (
	"errors"

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

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	access, ok := middleware.AccessToken(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	var req dto.LogoutRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	if err := h.auth.Logout(c.UserContext(), access, req.RefreshToken); err != nil {
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, fiber.Map{"message": "logged out"})
}

// Me handles GET /api/v1/auth/me.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID, ok := middleware.UserID(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	res, err := h.auth.Me(c.UserContext(), userID)
	if err != nil {
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}

func decode(c *fiber.Ctx, validate *validator.Validate, dst any) error {
	if err := c.BodyParser(dst); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid json")
	}
	if err := validate.Struct(dst); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}
	return nil
}

func writeData(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"data":  data,
		"error": nil,
		"meta":  nil,
	})
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, service.ErrEmailTaken):
		return fiber.NewError(fiber.StatusConflict, "email already registered")
	case errors.Is(err, service.ErrAccountBlocked):
		return fiber.NewError(fiber.StatusForbidden, "account blocked")
	case errors.Is(err, service.ErrAccountNotFound):
		return fiber.NewError(fiber.StatusNotFound, "account not found")
	case errors.Is(err, service.ErrOTPTooManyAttempts):
		return fiber.NewError(fiber.StatusTooManyRequests, "too many requests")
	case errors.Is(err, service.ErrInvalidCredentials),
		errors.Is(err, service.ErrRefreshTokenInvalid),
		errors.Is(err, service.ErrOTPInvalid),
		errors.Is(err, service.ErrOTPExpired),
		errors.Is(err, service.ErrInvalidToken):
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	default:
		return err
	}
}
