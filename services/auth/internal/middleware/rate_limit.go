package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/poro/shared-go/ratelimit"
)

const (
	otpRequestLimit = 5
	otpVerifyLimit  = 20
	loginLimit      = 10
	generalLimit    = 100
	// refreshLimit is higher than the general limit: every signed-in phone
	// behind one carrier NAT address refreshes through it, and a refresh token
	// cannot be guessed.
	refreshLimit = 300
	accountLimit = 100

	limitWindow = ratelimit.Window
	// keyPrefix changed from auth:rl: so old Fiber msgpack counters are never INCR'd.
	keyPrefix = "auth:rl:v2:"
)

// Limiter counts requests in Redis, shared by every replica. It fails closed:
// when Redis cannot be reached the request is refused with 503.
type Limiter = ratelimit.Limiter

// NewLimiter returns a Redis-backed limiter. A nil client refuses every limited request.
func NewLimiter(rdb redis.UniversalClient) *Limiter {
	return ratelimit.New(rdb, keyPrefix)
}

// OTPRequestLimit allows 5 requests per minute per IP on OTP issuance.
func OTPRequestLimit(l *Limiter) fiber.Handler {
	return l.Handler("otp", otpRequestLimit, ratelimit.ByIP)
}

// OTPVerifyLimit allows 20 requests per minute per IP on OTP verification.
func OTPVerifyLimit(l *Limiter) fiber.Handler {
	return l.Handler("verify", otpVerifyLimit, ratelimit.ByIP)
}

// LoginLimit allows 10 requests per minute per IP on email login.
func LoginLimit(l *Limiter) fiber.Handler { return l.Handler("login", loginLimit, ratelimit.ByIP) }

// GeneralLimit allows 100 requests per minute per IP on the public auth routes.
func GeneralLimit(l *Limiter) fiber.Handler {
	return l.Handler("general", generalLimit, ratelimit.ByIP)
}

// RefreshLimit allows 300 refreshes per minute per IP.
func RefreshLimit(l *Limiter) fiber.Handler {
	return l.Handler("refresh", refreshLimit, ratelimit.ByIP)
}

// AccountLimit allows 100 requests per minute per account. Mount it after
// NewAuth, so users sharing a carrier NAT address keep separate counters.
func AccountLimit(l *Limiter) fiber.Handler { return l.Handler("account", accountLimit, byAccount) }

// AccountLoginLimit allows 10 requests per minute per account on signed-in
// routes that check a password or a code. Mount it after NewAuth.
func AccountLoginLimit(l *Limiter) fiber.Handler {
	return l.Handler("account-login", loginLimit, byAccount)
}

func byAccount(c *fiber.Ctx) string {
	if claims, ok := Claims(c); ok {
		return "user:" + claims.UserID.String()
	}
	return ratelimit.ByIP(c)
}
