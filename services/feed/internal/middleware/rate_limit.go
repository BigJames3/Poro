// Package middleware holds the Redis-backed rate limiter of the feed API.
package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	redisstore "github.com/gofiber/storage/redis/v3"

	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/feed/internal/config"
)

// generalLimit is the per-user number of feed requests per minute.
const generalLimit = 120

// NewRedisStorage opens the Redis backend shared by the rate limiter, the
// For You sessions and the page cache.
func NewRedisStorage(cfg *config.Config) (*redisstore.Storage, error) {
	port, err := strconv.Atoi(cfg.RedisPort)
	if err != nil {
		return nil, fmt.Errorf("parse redis port: %w", err)
	}
	return redisstore.New(redisstore.Config{
		Host:     cfg.RedisHost,
		Port:     port,
		Password: cfg.RedisPassword,
		Database: cfg.RedisDB,
		PoolSize: 20,
	}), nil
}

// GeneralLimit counts per user when a token was verified, per IP otherwise.
// A rejected request gets 429 rate_limited and a Retry-After header.
func GeneralLimit(store fiber.Storage) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        generalLimit,
		Expiration: time.Minute,
		Storage:    store,
		KeyGenerator: func(c *fiber.Ctx) string {
			if claims, ok := jwtauth.ClaimsFrom(c); ok {
				return "feed:rl:" + claims.UserID.String()
			}
			return "feed:rl:" + c.IP()
		},
		LimitReached: func(*fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "too many requests")
		},
	})
}
