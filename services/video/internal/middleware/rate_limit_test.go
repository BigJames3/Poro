package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"
)

func TestInitLimit(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Post("/", InitLimit(nil), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	for i := 0; i < 10; i++ {
		resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/", nil), -1)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestCompleteAndGeneralLimit(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Get("/c", CompleteLimit(nil), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/g", GeneralLimit(nil), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/c", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/g", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
