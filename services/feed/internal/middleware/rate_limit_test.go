package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/poro/feed/internal/config"
)

func TestNewRedisStorageRejectsBadPort(t *testing.T) {
	_, err := NewRedisStorage(&config.Config{RedisHost: "localhost", RedisPort: "x"})
	require.Error(t, err)
}

func TestGeneralLimit(t *testing.T) {
	app := fiber.New()
	app.Get("/", GeneralLimit(nil), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	for range generalLimit {
		res, err := app.Test(httptest.NewRequest("GET", "/", nil))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusNoContent, res.StatusCode)
	}
	res, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, res.StatusCode)
}
