package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/httpx"
)

func TestPhotoPostsGate(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		reached := false
		app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
			var apiErr *httpx.APIError
			require.True(t, errors.As(err, &apiErr))
			return c.Status(apiErr.Status).SendString(apiErr.Code)
		}})
		app.Post("/photo-uploads", PhotoPosts(enabled), func(c *fiber.Ctx) error {
			reached = true
			return c.SendStatus(fiber.StatusCreated)
		})
		resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/photo-uploads", nil))
		require.NoError(t, err)
		_ = resp.Body.Close()
		if enabled {
			require.Equal(t, fiber.StatusCreated, resp.StatusCode)
			require.True(t, reached)
		} else {
			require.Equal(t, fiber.StatusNotFound, resp.StatusCode, "disabled: refused")
			require.False(t, reached, "disabled: the handler never runs")
		}
	}
}
