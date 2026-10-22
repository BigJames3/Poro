// Package middleware holds the Redis-backed rate limiter of the search API.
package middleware

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	redisstore "github.com/gofiber/storage/redis/v3"

	"github.com/poro/shared-go/ratelimit"

	"github.com/poro/search/internal/config"
)

// generalLimit is the number of search requests per minute, per account or IP.
const generalLimit = 60

// NewRedisStorage opens the Redis backend of the rate limiter.
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

// keyPrefix changed from search:rl: so old Fiber msgpack counters are never INCR'd.
const keyPrefix = "search:rl:v2:"

// NewLimiter returns the fail-closed limiter of the search API.
func NewLimiter(store *redisstore.Storage) *ratelimit.Limiter {
	return ratelimit.New(store.Conn(), keyPrefix)
}

// GeneralLimit counts per user when a token was verified, per IP otherwise.
// A rejected request gets 429 rate_limited and a Retry-After header; a Redis
// failure gets 503.
func GeneralLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("general", generalLimit, ratelimit.ByUserOrIP)
}
