// Package handler adapts HTTP requests to the feed service.
package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/feed/internal/dto"
	"github.com/poro/feed/internal/service"
)

var errUnauthorized = httpx.NewAPIError(fiber.StatusUnauthorized, "unauthorized", "unauthorized")

// Feed serves /api/v1/feed.
type Feed struct {
	svc *service.Feed
}

// New builds the HTTP handler.
func New(svc *service.Feed) *Feed {
	return &Feed{svc: svc}
}

func (h *Feed) ForYou(c *fiber.Ctx) error {
	return h.serve(c, h.svc.ForYou)
}

func (h *Feed) Following(c *fiber.Ctx) error {
	return h.serve(c, h.svc.Following)
}

func (h *Feed) Trending(c *fiber.Ctx) error {
	return h.serve(c, func(ctx context.Context, _ uuid.UUID, cursor string, limit int) (*dto.Page, error) {
		return h.svc.Trending(ctx, cursor, limit)
	})
}

type feedFunc func(ctx context.Context, viewer uuid.UUID, cursor string, limit int) (*dto.Page, error)

func (h *Feed) serve(c *fiber.Ctx, fn feedFunc) error {
	claims, ok := jwtauth.ClaimsFrom(c)
	if !ok {
		return errUnauthorized
	}
	page, err := fn(c.UserContext(), claims.UserID, c.Query("cursor"), c.QueryInt("limit"))
	if err != nil {
		return err
	}
	return httpx.WriteData(c, fiber.StatusOK, page)
}
