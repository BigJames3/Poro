package ratelimit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"
)

func newTestLimiter(t *testing.T) (*Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb, "test:rl:"), mr
}

func newApp(handler fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Get("/", handler, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	return app
}

func do(t *testing.T, app *fiber.App, who string) (int, http.Header) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Who", who)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, resp.Header
}

func byHeader(c *fiber.Ctx) string { return c.Get("X-Who") }

func TestLimitAndRetryAfter(t *testing.T) {
	l, _ := newTestLimiter(t)
	app := newApp(l.Handler("x", 3, ByIP))
	for i := 0; i < 3; i++ {
		status, _ := do(t, app, "")
		require.Equal(t, http.StatusNoContent, status, "request %d is within the limit", i+1)
	}
	status, header := do(t, app, "")
	require.Equal(t, http.StatusTooManyRequests, status)
	require.NotEmpty(t, header.Get(fiber.HeaderRetryAfter))
}

func TestKeysAreCountedSeparately(t *testing.T) {
	l, _ := newTestLimiter(t)
	app := newApp(l.Handler("x", 1, byHeader))
	status, _ := do(t, app, "a")
	require.Equal(t, http.StatusNoContent, status)
	status, _ = do(t, app, "b")
	require.Equal(t, http.StatusNoContent, status, "another account behind the same IP keeps its own counter")
	status, _ = do(t, app, "a")
	require.Equal(t, http.StatusTooManyRequests, status)
}

func TestCountersAreSharedAndExpire(t *testing.T) {
	l, mr := newTestLimiter(t)
	first, second := newApp(l.Handler("x", 2, ByIP)), newApp(l.Handler("x", 2, ByIP))
	for i := 0; i < 2; i++ {
		status, _ := do(t, first, "")
		require.Equal(t, http.StatusNoContent, status)
	}
	status, _ := do(t, second, "")
	require.Equal(t, http.StatusTooManyRequests, status, "replicas share the Redis counter")

	mr.FastForward(Window)
	status, _ = do(t, second, "")
	require.Equal(t, http.StatusNoContent, status, "the window resets after a minute")
	require.True(t, mr.Exists("test:rl:x:ip:0.0.0.0"), "keys carry the limiter prefix")
}

func TestFailsClosed(t *testing.T) {
	t.Run("redis down", func(t *testing.T) {
		l, mr := newTestLimiter(t)
		app := newApp(l.Handler("x", 2, ByIP))
		status, _ := do(t, app, "")
		require.Equal(t, http.StatusNoContent, status)
		mr.Close()
		for i := 0; i < 4; i++ {
			status, _ = do(t, app, "")
			require.Equal(t, http.StatusServiceUnavailable, status, "no request passes while Redis is down")
		}
	})
	t.Run("no redis client", func(t *testing.T) {
		status, _ := do(t, newApp(New(nil, "p:").Handler("x", 2, ByIP)), "")
		require.Equal(t, http.StatusServiceUnavailable, status)
	})
}

func TestByUserOrIPWithoutClaimsUsesIP(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(ByUserOrIP(c)) })
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ip:0.0.0.0", string(body))
}
