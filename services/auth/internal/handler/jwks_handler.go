package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/auth/internal/service"
)

// JWKSHandler publishes the public signing key so other services verify tokens locally.
type JWKSHandler struct {
	tokens service.TokenService
}

// NewJWKSHandler returns a JWKS handler.
func NewJWKSHandler(tokens service.TokenService) *JWKSHandler {
	return &JWKSHandler{tokens: tokens}
}

// JWKS handles GET /.well-known/jwks.json.
func (h *JWKSHandler) JWKS(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	return c.JSON(h.tokens.JWKS())
}
