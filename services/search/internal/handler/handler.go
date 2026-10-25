// Package handler adapts HTTP requests to the search service.
package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/capability"
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
	filters := service.Filters{Country: c.Query("country"), Currency: c.Query("currency"),
		MinPrice: c.Query("min_price"), MaxPrice: c.Query("max_price")}
	out, err := h.svc.Search(c.UserContext(), c.Query("q"), c.Query("type"), photoPosts(c), c.Query("cursor"), c.QueryInt("limit"), filters)
	if err != nil {
		return err
	}
	return httpx.WriteData(c, fiber.StatusOK, out)
}

func (h *Search) Suggest(c *fiber.Ctx) error {
	out, err := h.svc.Suggest(c.UserContext(), c.Query("q"), photoPosts(c))
	if err != nil {
		return err
	}
	return httpx.WriteData(c, fiber.StatusOK, out)
}

// photoPosts reports whether the client declared photo posts (ADR-0020).
func photoPosts(c *fiber.Ctx) bool {
	return capability.Has(c.Get(capability.Header), capability.PhotoPosts)
}
