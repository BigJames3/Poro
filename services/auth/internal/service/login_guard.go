package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/poro/auth/internal/config"
)

// ErrLoginLocked is returned while password sign-in is locked for an account
// after repeated failures (ADR-0017).
var ErrLoginLocked = errors.New("sign-in temporarily locked")

// LoginLockedError carries how long the lock lasts. It matches ErrLoginLocked.
type LoginLockedError struct {
	RetryAfter time.Duration
}

func (e *LoginLockedError) Error() string {
	return fmt.Sprintf("%s: retry after %s", ErrLoginLocked, e.RetryAfter)
}

// Is makes errors.Is(err, ErrLoginLocked) true.
func (e *LoginLockedError) Is(target error) bool { return target == ErrLoginLocked }

// LoginGuard locks password sign-in per account, whatever the IP: a
// distributed guessing attack on one account is stopped (ADR-0017). Accounts
// are named by their normalized email address, known or not, so a lock never
// reveals whether an account exists.
type LoginGuard interface {
	// Check returns a *LoginLockedError while the account is locked. The
	// password must not be checked then.
	Check(ctx context.Context, email string) error
	// Fail records a wrong password; enough of them lock the account.
	Fail(ctx context.Context, email string) error
	// Reset clears the failures after a successful sign-in or a password reset.
	Reset(ctx context.Context, email string) error
}

const (
	loginFailKeyPrefix = "auth:login:fail:"
	loginLockKeyPrefix = "auth:login:lock:"
)

// failScript counts a failure within the window and, from the max-th one on,
// locks the account: base for the first lock, doubled by each further failure,
// capped. A lock keeps the failures for its own length plus the window, so
// waiting a lock out never earns a fresh set of attempts: once at the cap,
// each further failure locks again at once.
// KEYS: failures, lock. ARGV: window ms, max failures, base ms, cap ms.
var failScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
local window = tonumber(ARGV[1])
local max = tonumber(ARGV[2])
if n < max then
  redis.call('PEXPIRE', KEYS[1], window)
  return 0
end
local lock = tonumber(ARGV[3]) * 2 ^ (n - max)
if lock > tonumber(ARGV[4]) then
  lock = tonumber(ARGV[4])
end
lock = math.floor(lock)
redis.call('SET', KEYS[2], '1', 'PX', lock)
redis.call('PEXPIRE', KEYS[1], lock + window)
return lock
`)

type redisLoginGuard struct {
	rdb         *redis.Client
	maxFailures int
	window      time.Duration
	lockBase    time.Duration
	lockMax     time.Duration
}

// NewRedisLoginGuard reads the lockout settings Config.Validate checked.
func NewRedisLoginGuard(cfg *config.Config, rdb *redis.Client) (LoginGuard, error) {
	if rdb == nil {
		return nil, errors.New("login guard: redis is required")
	}
	window, err := time.ParseDuration(cfg.LoginFailureWindow)
	if err != nil || window <= 0 {
		return nil, fmt.Errorf("login guard: LOGIN_FAILURE_WINDOW must be a positive duration, got %q", cfg.LoginFailureWindow)
	}
	base, err := time.ParseDuration(cfg.LoginLockBase)
	if err != nil || base <= 0 {
		return nil, fmt.Errorf("login guard: LOGIN_LOCK_BASE must be a positive duration, got %q", cfg.LoginLockBase)
	}
	maxLock, err := time.ParseDuration(cfg.LoginLockMax)
	if err != nil || maxLock < base {
		return nil, fmt.Errorf("login guard: LOGIN_LOCK_MAX must be a duration of at least LOGIN_LOCK_BASE, got %q", cfg.LoginLockMax)
	}
	if cfg.LoginMaxFailures <= 0 {
		return nil, fmt.Errorf("login guard: LOGIN_MAX_FAILURES must be positive, got %d", cfg.LoginMaxFailures)
	}
	return &redisLoginGuard{rdb: rdb, maxFailures: cfg.LoginMaxFailures, window: window, lockBase: base, lockMax: maxLock}, nil
}

func (g *redisLoginGuard) Check(ctx context.Context, email string) error {
	ttl, err := g.rdb.PTTL(ctx, loginLockKeyPrefix+accountKey(email)).Result()
	if err != nil {
		return fmt.Errorf("login guard: %w", err)
	}
	if ttl > 0 {
		return &LoginLockedError{RetryAfter: ttl}
	}
	return nil
}

func (g *redisLoginGuard) Fail(ctx context.Context, email string) error {
	key := accountKey(email)
	err := failScript.Run(ctx, g.rdb, []string{loginFailKeyPrefix + key, loginLockKeyPrefix + key},
		g.window.Milliseconds(), g.maxFailures, g.lockBase.Milliseconds(), g.lockMax.Milliseconds()).Err()
	if err != nil {
		return fmt.Errorf("login guard: %w", err)
	}
	return nil
}

func (g *redisLoginGuard) Reset(ctx context.Context, email string) error {
	key := accountKey(email)
	if err := g.rdb.Del(ctx, loginFailKeyPrefix+key, loginLockKeyPrefix+key).Err(); err != nil {
		return fmt.Errorf("login guard: %w", err)
	}
	return nil
}

// accountKey names an account in Redis without storing its address.
func accountKey(email string) string {
	sum := sha256.Sum256([]byte(NormalizeEmail(email)))
	return hex.EncodeToString(sum[:])
}
