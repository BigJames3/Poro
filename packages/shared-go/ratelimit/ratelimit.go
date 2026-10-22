// Package ratelimit is the Redis fixed-window rate limiter shared by the Go services.
//
// It fails closed: when Redis cannot be reached the request is refused with 503.
// The Fiber limiter it replaces ignored storage errors and restarted from an empty
// counter, which silently removed every limit during a Redis outage.
package ratelimit

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"
)

const (
	// Window is the length of one counting window.
	Window = time.Minute
	// timeout bounds one Redis round trip: a slow Redis refuses fast.
	timeout = 500 * time.Millisecond
)

// fixedWindow increments the counter and sets its expiry atomically.
// It returns {count, ttl_ms}.
var fixedWindow = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
local ttl = redis.call('PTTL', KEYS[1])
if ttl < 0 then redis.call('PEXPIRE', KEYS[1], ARGV[1]); ttl = tonumber(ARGV[1]) end
return {n, ttl}
`)

// ErrUnavailable is returned when the counter cannot be read.
var ErrUnavailable = errors.New("rate limiter unavailable")

// KeyFunc names whom a request is counted against.
type KeyFunc func(*fiber.Ctx) string

// ByIP counts every request against the client IP.
func ByIP(c *fiber.Ctx) string { return "ip:" + c.IP() }

// ByUserOrIP counts against the account when a token was verified earlier in the
// chain, so many users behind one carrier NAT do not share a counter, and
// against the IP otherwise.
func ByUserOrIP(c *fiber.Ctx) string {
	if claims, ok := jwtauth.ClaimsFrom(c); ok {
		return "user:" + claims.UserID.String()
	}
	return ByIP(c)
}

// Limiter counts requests in Redis, shared by every replica.
type Limiter struct {
	rdb    redis.UniversalClient
	prefix string
}

// New returns a limiter whose keys start with prefix. A nil client refuses
// every limited request.
func New(rdb redis.UniversalClient, prefix string) *Limiter {
	return &Limiter{rdb: rdb, prefix: prefix}
}

// Handler allows max requests per Window for each key under name. A request
// over the limit gets 429 with Retry-After; any Redis failure gets 503.
func (l *Limiter) Handler(name string, max int, key KeyFunc) fiber.Handler {
	unavailable := httpx.NewAPIError(fiber.StatusServiceUnavailable, "unavailable", "service temporarily unavailable")
	return func(c *fiber.Ctx) error {
		count, ttl, err := l.hit(c.UserContext(), name+":"+key(c))
		if err != nil {
			return unavailable.WithCause(err)
		}
		if count > int64(max) {
			c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(ttl.Seconds())+1))
			return fiber.NewError(fiber.StatusTooManyRequests, "too many requests")
		}
		return c.Next()
	}
}

func (l *Limiter) hit(parent context.Context, key string) (int64, time.Duration, error) {
	if l == nil || l.rdb == nil {
		return 0, 0, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	res, err := fixedWindow.Run(ctx, l.rdb, []string{l.prefix + key}, Window.Milliseconds()).Int64Slice()
	if err != nil {
		return 0, 0, errors.Join(ErrUnavailable, err)
	}
	if len(res) != 2 {
		return 0, 0, ErrUnavailable
	}
	return res[0], time.Duration(res[1]) * time.Millisecond, nil
}
