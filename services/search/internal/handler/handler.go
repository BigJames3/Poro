// Package handler adapts HTTP requests to the search service.
package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/search/internal/service"
)

// Search serves /api/v1/search. A token is optional: anyone may search.
type Search struct {
	svc *service.Search
}

// New builds the HTTP handler.
func New(svc *service.Search) *Search {
	return &Search{svc: svc}
}

func (h *Search) Search(c *fiber.Ctx) error {
	out, err := h.svc.Search(c.UserContext(), c.Query("q"), c.Query("type"), c.Query("cursor"), c.QueryInt("limit"))
	if err != nil {
		return err
	}
	return httpx.WriteData(c, fiber.StatusOK, out)
}

func (h *Search) Suggest(c *fiber.Ctx) error {
	out, err := h.svc.Suggest(c.UserContext(), c.Query("q"))
	if err != nil {
		return err
	}
	return httpx.WriteData(c, fiber.StatusOK, out)
}
