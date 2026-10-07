// Package middleware holds the Redis-backed rate limiters of the social API.
package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	redisstore "github.com/gofiber/storage/redis/v3"

	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/social/internal/config"
)

// Per-user limits per minute. Reads share the general limit.
const (
	likeLimit    = 60
	commentLimit = 10
	followLimit  = 30
	shareLimit   = 30
	generalLimit = 120
)

// NewRedisStorage opens the Redis backend used by the rate limiters.
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
		PoolSize: 10,
	}), nil
}

// LikeLimit covers video and comment likes.
func LikeLimit(store fiber.Storage) fiber.Handler {
	return newLimit(likeLimit, "social:rl:like:", store)
}

// CommentLimit covers posting and editing comments.
func CommentLimit(store fiber.Storage) fiber.Handler {
	return newLimit(commentLimit, "social:rl:comment:", store)
}

// FollowLimit covers follows and unfollows.
func FollowLimit(store fiber.Storage) fiber.Handler {
	return newLimit(followLimit, "social:rl:follow:", store)
}

// ShareLimit keeps share counters from being inflated.
func ShareLimit(store fiber.Storage) fiber.Handler {
	return newLimit(shareLimit, "social:rl:share:", store)
}

// GeneralLimit covers every API route, reads included.
func GeneralLimit(store fiber.Storage) fiber.Handler {
	return newLimit(generalLimit, "social:rl:general:", store)
}

// newLimit counts per user when a token was verified, per IP otherwise. A
// rejected request gets 429 rate_limited and a Retry-After header.
func newLimit(maxPerMinute int, prefix string, store fiber.Storage) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        maxPerMinute,
		Expiration: time.Minute,
		Storage:    store,
		KeyGenerator: func(c *fiber.Ctx) string {
			if claims, ok := jwtauth.ClaimsFrom(c); ok {
				return prefix + claims.UserID.String()
			}
			return prefix + c.IP()
		},
		LimitReached: func(*fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "too many requests")
		},
	})
}
