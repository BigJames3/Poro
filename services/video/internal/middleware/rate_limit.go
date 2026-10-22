package middleware

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	redisstore "github.com/gofiber/storage/redis/v3"

	"github.com/poro/shared-go/ratelimit"

	"github.com/poro/video/internal/config"
)

const (
	initLimit     = 10
	completeLimit = 20
	generalLimit  = 60
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

// keyPrefix changed from video:rl: so old Fiber msgpack counters are never INCR'd.
const keyPrefix = "video:rl:v2:"

// NewLimiter returns the fail-closed limiter of the video API.
func NewLimiter(store *redisstore.Storage) *ratelimit.Limiter {
	return ratelimit.New(store.Conn(), keyPrefix)
}

// InitLimit covers upload creation, per account.
func InitLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("init", initLimit, ratelimit.ByUserOrIP)
}

// CompleteLimit covers upload completion, per account.
func CompleteLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("complete", completeLimit, ratelimit.ByUserOrIP)
}

// GeneralLimit covers every video route, per account once a token was
// verified, per IP otherwise. Mount it after the JWT middleware.
func GeneralLimit(l *ratelimit.Limiter) fiber.Handler {
	return l.Handler("general", generalLimit, ratelimit.ByUserOrIP)
}
