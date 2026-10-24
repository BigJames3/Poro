package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CapabilitiesHeader lists what a client can display (ADR-0020), as
// comma-separated case-insensitive tokens; unknown tokens are ignored.
const CapabilitiesHeader = "Poro-Capabilities"

// CapabilityPhotoPosts is declared by clients that display photo posts.
const CapabilityPhotoPosts = "photo-posts"

// PhotoPosts reports whether the request declared photo-posts. Without the
// header, as from every app released before photo posts, it is false: such a
// client never receives a photo post.
func PhotoPosts(c *fiber.Ctx) bool {
	return hasCapability(c.Get(CapabilitiesHeader), CapabilityPhotoPosts)
}

func hasCapability(header, capability string) bool {
	for _, token := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(token), capability) {
			return true
		}
	}
	return false
}
