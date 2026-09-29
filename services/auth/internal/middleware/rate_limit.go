package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	redisstore "github.com/gofiber/storage/redis/v3"

	"github.com/poro/auth/internal/config"
)

const (
	otpRequestLimit = 5
	loginLimit      = 10
	generalLimit    = 100
)

// NewRedisStorage opens the Redis backend used by the rate limiters.
// A nil storage makes the limiters keep counters in process memory.
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

// OTPRequestLimit allows 5 requests per minute per IP on OTP issuance.
func OTPRequestLimit(store fiber.Storage) fiber.Handler {
	return newLimit(otpRequestLimit, "auth:rl:otp:", store)
}

// LoginLimit allows 10 requests per minute per IP on email login.
func LoginLimit(store fiber.Storage) fiber.Handler {
	return newLimit(loginLimit, "auth:rl:login:", store)
}

// GeneralLimit allows 100 requests per minute per IP on the other auth routes.
func GeneralLimit(store fiber.Storage) fiber.Handler {
	return newLimit(generalLimit, "auth:rl:general:", store)
}

func newLimit(max int, prefix string, store fiber.Storage) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: time.Minute,
		Storage:    store,
		KeyGenerator: func(c *fiber.Ctx) string {
			return prefix + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "too many requests")
		},
	})
}
