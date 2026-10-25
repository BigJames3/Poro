package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/httpx"
)

// errPhotoPostsDisabled answers the photo post upload routes when they are
// switched off (PHOTO_POSTS_ENABLED=false; on by default).
var errPhotoPostsDisabled = httpx.NewAPIError(fiber.StatusNotFound, "photo_posts_disabled",
	"photo posts are not available yet")

// PhotoPosts lets photo post creation and completion through only when
// enabled. Mounted before authentication and rate limits, a disabled route
// costs no quota and accepts nothing.
func PhotoPosts(enabled bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !enabled {
			return errPhotoPostsDisabled
		}
		return c.Next()
	}
}
