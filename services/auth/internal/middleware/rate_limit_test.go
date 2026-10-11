package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/auth/internal/config"
)

func TestLimitsPerRoute(t *testing.T) {
	cases := []struct {
		name  string
		limit func(fiber.Storage) fiber.Handler
		max   int
	}{
		{name: "otp request", limit: OTPRequestLimit, max: otpRequestLimit},
		{name: "otp verify", limit: OTPVerifyLimit, max: otpVerifyLimit},
		{name: "login", limit: LoginLimit, max: loginLimit},
		{name: "general", limit: GeneralLimit, max: generalLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
			app.Get("/", tc.limit(nil), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
			for i := 0; i < tc.max; i++ {
				require.Equal(t, http.StatusNoContent, get(t, app), "request %d is within the limit", i+1)
			}
			require.Equal(t, http.StatusTooManyRequests, get(t, app))
		})
	}
}

func TestRedisStorageSharesCounters(t *testing.T) {
	mr := miniredis.RunT(t)
	host, port, err := net.SplitHostPort(mr.Addr())
	require.NoError(t, err)
	store, err := NewRedisStorage(&config.Config{RedisHost: host, RedisPort: port})
	require.NoError(t, err)

	first := fiber.New()
	first.Get("/", OTPRequestLimit(store), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	second := fiber.New()
	second.Get("/", OTPRequestLimit(store), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	for i := 0; i < otpRequestLimit; i++ {
		require.Equal(t, http.StatusNoContent, get(t, first))
	}
	require.Equal(t, http.StatusTooManyRequests, get(t, second), "replicas share the Redis counter")

	_, err = NewRedisStorage(&config.Config{RedisHost: host, RedisPort: "not-a-port"})
	require.Error(t, err)
}

func get(t *testing.T, app *fiber.App) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil), -1)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode
}
