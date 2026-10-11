package handler

import (
	"errors"
	"math"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/service"
)

// EmailHandler exposes email verification and password reset.
type EmailHandler struct {
	emails   service.EmailService
	validate *validator.Validate
}

// NewEmailHandler returns an email handler with a shared validator.
func NewEmailHandler(emails service.EmailService) *EmailHandler {
	return &EmailHandler{emails: emails, validate: validator.New()}
}

// RequestVerification handles POST /api/v1/auth/email/verification/request.
func (h *EmailHandler) RequestVerification(c *fiber.Ctx) error {
	claims, err := requireClaims(c)
	if err != nil {
		return err
	}
	expiresIn, err := h.emails.RequestVerification(c.UserContext(), claims.UserID)
	if err != nil {
		return mapEmailError(c, err)
	}
	return writeData(c, fiber.StatusAccepted, dto.CodeSentResponse{ExpiresIn: expiresIn})
}

// ConfirmVerification handles POST /api/v1/auth/email/verification/confirm.
func (h *EmailHandler) ConfirmVerification(c *fiber.Ctx) error {
	claims, err := requireClaims(c)
	if err != nil {
		return err
	}
	var req dto.ConfirmEmailRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	res, err := h.emails.ConfirmVerification(c.UserContext(), claims.UserID, req.Code)
	if err != nil {
		return mapEmailError(c, err)
	}
	return writeData(c, fiber.StatusOK, res)
}

// ForgotPassword handles POST /api/v1/auth/email/password/forgot. It answers
// 202 whether or not the address has an account.
func (h *EmailHandler) ForgotPassword(c *fiber.Ctx) error {
	var req dto.ForgotPasswordRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	expiresIn, err := h.emails.ForgotPassword(c.UserContext(), req.Email)
	if err != nil {
		return mapEmailError(c, err)
	}
	return writeData(c, fiber.StatusAccepted, dto.CodeSentResponse{ExpiresIn: expiresIn})
}

// ResetPassword handles POST /api/v1/auth/email/password/reset.
func (h *EmailHandler) ResetPassword(c *fiber.Ctx) error {
	var req dto.ResetPasswordRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	if err := h.emails.ResetPassword(c.UserContext(), req); err != nil {
		return mapEmailError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func mapEmailError(c *fiber.Ctx, err error) error {
	var throttled *service.ThrottledError
	switch {
	case errors.As(err, &throttled):
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(math.Ceil(throttled.RetryAfter.Seconds()))))
		return httpx.NewAPIError(fiber.StatusTooManyRequests, "email_throttled", "too many emails, wait before asking again")
	case errors.Is(err, service.ErrEmailMissing):
		return httpx.NewAPIError(fiber.StatusUnprocessableEntity, "email_missing", "this account has no email address")
	case errors.Is(err, service.ErrEmailAlreadyVerified):
		return httpx.NewAPIError(fiber.StatusConflict, "email_already_verified", "email already verified")
	case errors.Is(err, service.ErrEmailCodeInvalid):
		return httpx.NewAPIError(fiber.StatusUnprocessableEntity, "code_invalid", "invalid code")
	case errors.Is(err, service.ErrEmailCodeExpired):
		return httpx.NewAPIError(fiber.StatusUnprocessableEntity, "code_expired", "code expired, ask for a new one")
	case errors.Is(err, service.ErrEmailCodeTooManyAttempts):
		return httpx.NewAPIError(fiber.StatusTooManyRequests, "code_too_many_attempts", "too many attempts, ask for a new code")
	case errors.Is(err, service.ErrEmailUnavailable):
		return httpx.NewAPIError(fiber.StatusServiceUnavailable, "email_unavailable", "email delivery unavailable, try again").WithCause(err)
	default:
		return mapAuthError(err)
	}
}
