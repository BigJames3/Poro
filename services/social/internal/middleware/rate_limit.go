// Package middleware holds the Redis-backed rate limiters of the social API.
package middleware

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	redisstore "github.com/gofiber/storage/redis/v3"

	"github.com/poro/shared-go/ratelimit"

	"github.com/poro/social/internal/config"
)

// keyPrefix changed from social:rl: so old Fiber msgpack counters are never INCR'd.
const keyPrefix = "social:rl:v2:"

// Per-user limits per minute. Reads share the general limit.
const (
	likeLimit    = 60
	commentLimit = 10
	followLimit  = 30
	shareLimit   = 30
	generalLimit = 120
)

// NewLimiter returns the fail-closed limiter of the social API.
func NewLimiter(store *redisstore.Storage) *ratelimit.Limiter {
	return ratelimit.New(store.Conn(), keyPrefix)
}

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
func LikeLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("like", likeLimit, ratelimit.ByUserOrIP)
}

// CommentLimit covers posting and editing comments.
func CommentLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("comment", commentLimit, ratelimit.ByUserOrIP)
}

// FollowLimit covers follows and unfollows.
func FollowLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("follow", followLimit, ratelimit.ByUserOrIP)
}

// ShareLimit keeps share counters from being inflated.
func ShareLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("share", shareLimit, ratelimit.ByUserOrIP)
}

// GeneralLimit covers every API route, reads included.
func GeneralLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("general", generalLimit, ratelimit.ByUserOrIP)
}
