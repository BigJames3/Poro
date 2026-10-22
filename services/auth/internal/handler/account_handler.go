package handler

import (
	"context"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/service"
)

// AccountHandler exposes the caller's sessions and the deletion of their account.
type AccountHandler struct {
	accounts service.AccountService
	validate *validator.Validate
}

// NewAccountHandler returns an account handler with a shared validator.
func NewAccountHandler(accounts service.AccountService) *AccountHandler {
	return &AccountHandler{accounts: accounts, validate: validator.New()}
}

// Sessions handles GET /api/v1/auth/sessions.
func (h *AccountHandler) Sessions(c *fiber.Ctx) error {
	claims, err := requireClaims(c)
	if err != nil {
		return err
	}
	res, err := h.accounts.Sessions(c.UserContext(), *claims)
	if err != nil {
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}

// RevokeSession handles DELETE /api/v1/auth/sessions/:id.
func (h *AccountHandler) RevokeSession(c *fiber.Ctx) error {
	claims, err := requireClaims(c)
	if err != nil {
		return err
	}
	sessionID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return mapAuthError(service.ErrSessionNotFound)
	}
	if err := h.accounts.RevokeSession(c.UserContext(), *claims, sessionID); err != nil {
		return mapAuthError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// RevokeOtherSessions handles POST /api/v1/auth/sessions/revoke-others.
func (h *AccountHandler) RevokeOtherSessions(c *fiber.Ctx) error {
	claims, err := requireClaims(c)
	if err != nil {
		return err
	}
	res, err := h.accounts.RevokeOtherSessions(c.UserContext(), *claims)
	if err != nil {
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}

// DeleteAccount handles POST /api/v1/auth/me/delete.
func (h *AccountHandler) DeleteAccount(c *fiber.Ctx) error {
	claims, err := requireClaims(c)
	if err != nil {
		return err
	}
	var req dto.DeleteAccountRequest
	if err := decode(c, h.validate, &req); err != nil {
		return err
	}
	res, err := h.accounts.DeleteAccount(c.UserContext(), *claims, req)
	if err != nil {
		setRetryAfter(c, err)
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusAccepted, res)
}

// CancelDeletion handles POST /api/v1/auth/me/delete/cancel.
func (h *AccountHandler) CancelDeletion(c *fiber.Ctx) error {
	claims, err := requireClaims(c)
	if err != nil {
		return err
	}
	res, err := h.accounts.CancelDeletion(c.UserContext(), *claims)
	if err != nil {
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}

func requireClaims(c *fiber.Ctx) (*service.TokenClaims, error) {
	claims, ok := middleware.Claims(c)
	if !ok {
		return nil, httpx.NewAPIError(fiber.StatusUnauthorized, "unauthorized", "unauthorized")
	}
	return claims, nil
}

// SuspendAccount handles POST /api/v1/auth/admin/users/:id/suspend (ADMIN or MODERATOR).
func (h *AccountHandler) SuspendAccount(c *fiber.Ctx) error {
	return h.adminAction(c, h.accounts.Suspend)
}

// ReinstateAccount handles POST /api/v1/auth/admin/users/:id/reinstate (ADMIN or MODERATOR).
func (h *AccountHandler) ReinstateAccount(c *fiber.Ctx) error {
	return h.adminAction(c, h.accounts.Reinstate)
}

func (h *AccountHandler) adminAction(
	c *fiber.Ctx,
	action func(context.Context, service.TokenClaims, uuid.UUID) (*dto.UserResponse, error),
) error {
	claims, err := requireClaims(c)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return mapAuthError(service.ErrAccountNotFound)
	}
	res, err := action(c.UserContext(), *claims, userID)
	if err != nil {
		return mapAuthError(err)
	}
	return writeData(c, fiber.StatusOK, res)
}
