package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/poro/social/internal/config"
)

func TestNewRedisStorageRejectsBadPort(t *testing.T) {
	_, err := NewRedisStorage(&config.Config{RedisHost: "localhost", RedisPort: "not-a-port"})
	require.Error(t, err)
}

func TestLimitsCountPerIPForAnonymousCallers(t *testing.T) {
	cases := []struct {
		name  string
		limit func(fiber.Storage) fiber.Handler
		max   int
	}{
		{"like", LikeLimit, likeLimit},
		{"comment", CommentLimit, commentLimit},
		{"follow", FollowLimit, followLimit},
		{"share", ShareLimit, shareLimit},
		{"general", GeneralLimit, generalLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/", tc.limit(nil), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
			for range tc.max {
				res, err := app.Test(httptest.NewRequest("GET", "/", nil))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusNoContent, res.StatusCode)
			}
			res, err := app.Test(httptest.NewRequest("GET", "/", nil))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooManyRequests, res.StatusCode)
		})
	}
}
