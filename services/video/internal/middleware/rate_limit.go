package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	redisstore "github.com/gofiber/storage/redis/v3"

	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/video/internal/config"
)

const (
	initLimit     = 10
	completeLimit = 20
	generalLimit  = 60
)

// NewRedisStorage opens the Redis backend used by the rate limiters.
func NewRedisStorage(cfg *config.Config) (fiber.Storage, error) {
	port, err := strconv.Atoi(cfg.RedisPort)
	if err != nil {
		return nil, fmt.Errorf("parse redis port: %w", err)
	}
	return redisstore.New(redisstore.Config{
		Host:     cfg.RedisHost,
		Port:     port,
		Password: cfg.RedisPassword,
		Database: cfg.RedisDB,
		PoolSize: 10,
	}), nil
}

func InitLimit(store fiber.Storage) fiber.Handler {
	return newLimit(initLimit, "video:rl:init:", store)
}

func CompleteLimit(store fiber.Storage) fiber.Handler {
	return newLimit(completeLimit, "video:rl:complete:", store)
}

func GeneralLimit(store fiber.Storage) fiber.Handler {
	return newLimit(generalLimit, "video:rl:general:", store)
}

func newLimit(max int, prefix string, store fiber.Storage) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: time.Minute,
		Storage:    store,
		KeyGenerator: func(c *fiber.Ctx) string {
			if claims, ok := jwtauth.ClaimsFrom(c); ok {
				return prefix + claims.UserID.String()
			}
			return prefix + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "too many requests")
		},
	})
}
