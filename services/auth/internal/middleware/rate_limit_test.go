package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/auth/internal/service"
)

func newTestLimiter(t *testing.T) (*Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewLimiter(rdb), mr
}

func newLimitedApp(handler fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Get("/", handler, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	return app
}

func TestLimitsPerRoute(t *testing.T) {
	cases := []struct {
		name  string
		limit func(*Limiter) fiber.Handler
		max   int
	}{
		{name: "otp request", limit: OTPRequestLimit, max: otpRequestLimit},
		{name: "otp verify", limit: OTPVerifyLimit, max: otpVerifyLimit},
		{name: "login", limit: LoginLimit, max: loginLimit},
		{name: "general", limit: GeneralLimit, max: generalLimit},
		{name: "refresh", limit: RefreshLimit, max: refreshLimit},
		{name: "account without claims", limit: AccountLimit, max: accountLimit},
		{name: "account login without claims", limit: AccountLoginLimit, max: loginLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limiter, _ := newTestLimiter(t)
			app := newLimitedApp(tc.limit(limiter))
			for i := 0; i < tc.max; i++ {
				require.Equal(t, http.StatusNoContent, get(t, app), "request %d is within the limit", i+1)
			}
			status, header := do(t, app)
			require.Equal(t, http.StatusTooManyRequests, status)
			require.NotEmpty(t, header.Get(fiber.HeaderRetryAfter))
		})
	}
}

func TestRedisCountersAreSharedAndExpire(t *testing.T) {
	limiter, mr := newTestLimiter(t)
	first := newLimitedApp(OTPRequestLimit(limiter))
	second := newLimitedApp(OTPRequestLimit(limiter))
	for i := 0; i < otpRequestLimit; i++ {
		require.Equal(t, http.StatusNoContent, get(t, first))
	}
	require.Equal(t, http.StatusTooManyRequests, get(t, second), "replicas share the Redis counter")

	mr.FastForward(limitWindow)
	require.Equal(t, http.StatusNoContent, get(t, second), "the window resets after a minute")
}

func TestLimiterFailsClosed(t *testing.T) {
	t.Run("redis down", func(t *testing.T) {
		limiter, mr := newTestLimiter(t)
		app := newLimitedApp(LoginLimit(limiter))
		require.Equal(t, http.StatusNoContent, get(t, app))
		mr.Close()
		for i := 0; i < loginLimit+2; i++ {
			require.Equal(t, http.StatusServiceUnavailable, get(t, app), "no request passes while Redis is down")
		}
	})
	t.Run("no redis client", func(t *testing.T) {
		app := newLimitedApp(LoginLimit(NewLimiter(nil)))
		require.Equal(t, http.StatusServiceUnavailable, get(t, app))
	})
}

func do(t *testing.T, app *fiber.App) (int, http.Header) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil), -1)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, resp.Header
}

func get(t *testing.T, app *fiber.App) int {
	t.Helper()
	status, _ := do(t, app)
	return status
}

func TestAccountLimitsCountPerAccount(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Get("/", func(c *fiber.Ctx) error {
		c.Locals(LocalClaims, &service.TokenClaims{UserID: uuid.MustParse(c.Get("X-User"))})
		return c.Next()
	}, AccountLoginLimit(limiter), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	call := func(user string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-User", user)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		return resp.StatusCode
	}
	first, second := uuid.NewString(), uuid.NewString()
	for i := 0; i < loginLimit; i++ {
		require.Equal(t, http.StatusNoContent, call(first))
	}
	require.Equal(t, http.StatusTooManyRequests, call(first))
	require.Equal(t, http.StatusNoContent, call(second), "another account on the same IP keeps its own counter")
}
