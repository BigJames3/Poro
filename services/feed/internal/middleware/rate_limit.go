// Package middleware holds the Redis-backed rate limiter of the feed API.
package middleware

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	redisstore "github.com/gofiber/storage/redis/v3"

	"github.com/poro/shared-go/ratelimit"

	"github.com/poro/feed/internal/config"
)

// generalLimit is the per-user number of feed requests per minute.
const generalLimit = 120

// publicLimit is the per-IP number of guest feed requests per minute. Lower
// than generalLimit: many guests can share one carrier IP, but an anonymous
// client must not page the whole catalogue quickly.
const publicLimit = 60

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

// keyPrefix changed from feed:rl: so old Fiber msgpack counters are never INCR'd.
const keyPrefix = "feed:rl:v2:"

// NewLimiter returns the fail-closed limiter of the feed API.
func NewLimiter(store *redisstore.Storage) *ratelimit.Limiter {
	return ratelimit.New(store.Conn(), keyPrefix)
}

// GeneralLimit counts per user when a token was verified, per IP otherwise.
// A rejected request gets 429 rate_limited and a Retry-After header; a Redis
// failure gets 503.
func GeneralLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("general", generalLimit, ratelimit.ByUserOrIP)
}

// PublicLimit counts guest requests per IP, apart from the authenticated
// counter.
func PublicLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("public", publicLimit, ratelimit.ByIP)
}
