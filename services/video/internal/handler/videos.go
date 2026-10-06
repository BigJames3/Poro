package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/video/internal/dto"
	"github.com/poro/video/internal/service"
)

// Videos serves /api/v1/videos.
type Videos struct {
	svc       *service.Videos
	principal func(*fiber.Ctx) (*jwtauth.Claims, bool)
}

// NewVideos builds the HTTP handler.
func NewVideos(svc *service.Videos) *Videos {
	return &Videos{svc: svc, principal: jwtauth.ClaimsFrom}
}

func (h *Videos) Init(c *fiber.Ctx) error {
	user, err := h.requireUser(c)
	if err != nil {
		return err
	}
	var req dto.InitRequest
	if err := c.BodyParser(&req); err != nil {
		return httpx.NewAPIError(fiber.StatusBadRequest, "invalid_request", "invalid request")
	}
	out, err := h.svc.InitUpload(c.UserContext(), user, req)
	if err != nil {
		return err
	}
	return httpx.WriteData(c, fiber.StatusCreated, out)
}

func (h *Videos) Complete(c *fiber.Ctx) error {
	user, err := h.requireUser(c)
	if err != nil {
		return err
	}
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var req dto.CompleteRequest
	if err := c.BodyParser(&req); err != nil {
		return httpx.NewAPIError(fiber.StatusBadRequest, "invalid_request", "invalid request")
	}
	out, err := h.svc.Complete(c.UserContext(), user, id, req)
	if err != nil {
		return err
	}
	return httpx.WriteData(c, fiber.StatusOK, out)
}

func (h *Videos) Abort(c *fiber.Ctx) error {
	user, err := h.requireUser(c)
	if err != nil {
		return err
	}
	id, err := parseID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Abort(c.UserContext(), user, id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Videos) Get(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var viewer *uuid.UUID
	if claims, ok := h.principal(c); ok {
		viewer = &claims.UserID
	}
	out, err := h.svc.Get(c.UserContext(), id, viewer)
	if err != nil {
		return err
	}
	return httpx.WriteData(c, fiber.StatusOK, out)
}

func (h *Videos) List(c *fiber.Ctx) error {
	user, err := h.requireUser(c)
	if err != nil {
		return err
	}
	limit := c.QueryInt("limit", 0)
	out, err := h.svc.List(c.UserContext(), user, c.Query("cursor"), limit)
	if err != nil {
		return err
	}
	return httpx.WriteData(c, fiber.StatusOK, out)
}

func (h *Videos) Delete(c *fiber.Ctx) error {
	user, err := h.requireUser(c)
	if err != nil {
		return err
	}
	id, err := parseID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(c.UserContext(), user, id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Videos) requireUser(c *fiber.Ctx) (uuid.UUID, error) {
	claims, ok := h.principal(c)
	if !ok {
		return uuid.Nil, httpx.NewAPIError(fiber.StatusUnauthorized, "unauthorized", "unauthorized")
	}
	return claims.UserID, nil
}

func parseID(c *fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, httpx.NewAPIError(fiber.StatusBadRequest, "invalid_request", "invalid video id")
	}
	return id, nil
}
