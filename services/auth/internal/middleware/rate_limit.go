package middleware

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/poro/shared-go/httpx"
)

const (
	otpRequestLimit = 5
	otpVerifyLimit  = 20
	loginLimit      = 10
	generalLimit    = 100

	limitWindow = time.Minute
	// limiterTimeout bounds one Redis round trip: a slow Redis refuses fast.
	limiterTimeout = 500 * time.Millisecond
	// keyPrefix changed from auth:rl: so old Fiber msgpack counters are never INCR'd.
	keyPrefix = "auth:rl:v2:"
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

// Limiter counts requests per IP in Redis, shared by every replica.
//
// It fails closed: when Redis cannot be reached the request is refused with 503.
// The Fiber limiter it replaces ignored storage errors and reset its counters,
// which silently disabled brute-force protection during a Redis outage.
type Limiter struct {
	rdb redis.UniversalClient
}

// NewLimiter returns a Redis-backed limiter. A nil client refuses every limited request.
func NewLimiter(rdb redis.UniversalClient) *Limiter {
	return &Limiter{rdb: rdb}
}

// OTPRequestLimit allows 5 requests per minute per IP on OTP issuance.
func OTPRequestLimit(l *Limiter) fiber.Handler { return l.limit("otp:", otpRequestLimit) }

// OTPVerifyLimit allows 20 requests per minute per IP on OTP verification.
func OTPVerifyLimit(l *Limiter) fiber.Handler { return l.limit("verify:", otpVerifyLimit) }

// LoginLimit allows 10 requests per minute per IP on email login.
func LoginLimit(l *Limiter) fiber.Handler { return l.limit("login:", loginLimit) }

// GeneralLimit allows 100 requests per minute per IP on the other auth routes.
func GeneralLimit(l *Limiter) fiber.Handler { return l.limit("general:", generalLimit) }

var errLimiterUnavailable = errors.New("rate limiter unavailable")

func (l *Limiter) limit(prefix string, max int) fiber.Handler {
	unavailable := httpx.NewAPIError(fiber.StatusServiceUnavailable, "unavailable", "service temporarily unavailable")
	return func(c *fiber.Ctx) error {
		count, ttl, err := l.hit(c.UserContext(), keyPrefix+prefix+c.IP())
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
		return 0, 0, errLimiterUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, limiterTimeout)
	defer cancel()
	res, err := fixedWindow.Run(ctx, l.rdb, []string{key}, limitWindow.Milliseconds()).Int64Slice()
	if err != nil {
		return 0, 0, errors.Join(errLimiterUnavailable, err)
	}
	if len(res) != 2 {
		return 0, 0, errLimiterUnavailable
	}
	return res[0], time.Duration(res[1]) * time.Millisecond, nil
}
