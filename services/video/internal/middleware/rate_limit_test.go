package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/ratelimit/ratelimittest"
)

func TestInitLimit(t *testing.T) {
	limiter, _ := ratelimittest.New(t)
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Post("/", InitLimit(limiter), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	for i := 0; i < 10; i++ {
		resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/", nil), -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestCompleteAndGeneralLimit(t *testing.T) {
	limiter, mr := ratelimittest.New(t)
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Get("/c", CompleteLimit(limiter), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/g", GeneralLimit(limiter), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/c", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/g", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	mr.Close()
	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/g", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "a Redis outage refuses instead of lifting the limit")
}
