package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/poro/auth/internal/config"
)

func TestLoginGuardLocksProgressively(t *testing.T) {
	ctx := context.Background()
	guard, mr := newTestLoginGuard(t)
	email := "awa@poro.test"

	for range 4 {
		require.NoError(t, guard.Check(ctx, email))
		require.NoError(t, guard.Fail(ctx, email))
	}
	require.NoError(t, guard.Check(ctx, email), "4 failures do not lock")

	require.NoError(t, guard.Fail(ctx, email))
	err := guard.Check(ctx, email)
	var locked *LoginLockedError
	require.ErrorAs(t, err, &locked)
	require.ErrorIs(t, err, ErrLoginLocked)
	require.Equal(t, time.Minute, locked.RetryAfter, "the 5th failure locks for a minute")
	require.ErrorIs(t, guard.Check(ctx, "AWA@Poro.Test "), ErrLoginLocked, "the address is normalized")
	require.NoError(t, guard.Check(ctx, "other@poro.test"), "other accounts are not affected")

	mr.FastForward(time.Minute)
	require.NoError(t, guard.Check(ctx, email))
	for _, want := range []time.Duration{2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 15 * time.Minute, 15 * time.Minute} {
		require.NoError(t, guard.Fail(ctx, email))
		require.ErrorAs(t, guard.Check(ctx, email), &locked)
		require.Equal(t, want, locked.RetryAfter)
		mr.FastForward(want)
	}

	require.NoError(t, guard.Fail(ctx, email))
	require.ErrorAs(t, guard.Check(ctx, email), &locked)
	require.Equal(t, 15*time.Minute, locked.RetryAfter,
		"waiting a lock out earns no fresh attempts: each failure locks again")

	require.NoError(t, guard.Reset(ctx, email))
	require.NoError(t, guard.Fail(ctx, email))
	require.NoError(t, guard.Check(ctx, email), "a reset starts the count again")

	for _, key := range mr.Keys() {
		require.NotContains(t, strings.ToLower(key), "awa", "no address is stored in Redis")
	}
}

func TestLoginGuardForgetsOldFailures(t *testing.T) {
	ctx := context.Background()
	guard, mr := newTestLoginGuard(t)
	for range 4 {
		require.NoError(t, guard.Fail(ctx, "awa@poro.test"))
	}
	mr.FastForward(16 * time.Minute)
	require.NoError(t, guard.Fail(ctx, "awa@poro.test"))
	require.NoError(t, guard.Check(ctx, "awa@poro.test"), "failures older than the window are forgotten")

	// After a lock, failures are kept for the lock plus the window.
	for range 4 {
		require.NoError(t, guard.Fail(ctx, "awa@poro.test"))
	}
	require.Error(t, guard.Check(ctx, "awa@poro.test"))
	mr.FastForward(time.Minute + 15*time.Minute + time.Second)
	require.NoError(t, guard.Fail(ctx, "awa@poro.test"))
	require.NoError(t, guard.Check(ctx, "awa@poro.test"), "a quiet window after the lock forgets them")
}

func TestLoginGuardFailsWhenRedisDoes(t *testing.T) {
	ctx := context.Background()
	guard, mr := newTestLoginGuard(t)
	mr.Close()
	require.Error(t, guard.Check(ctx, "awa@poro.test"))
	require.Error(t, guard.Fail(ctx, "awa@poro.test"))
	require.Error(t, guard.Reset(ctx, "awa@poro.test"))
}

func TestNewRedisLoginGuardChecksItsSettings(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	_, err := NewRedisLoginGuard(guardConfig(), nil)
	require.Error(t, err)
	for name, mutate := range map[string]func(c *config.Config){
		"window":   func(c *config.Config) { c.LoginFailureWindow = "soon" },
		"base":     func(c *config.Config) { c.LoginLockBase = "0s" },
		"max":      func(c *config.Config) { c.LoginLockMax = "30s" },
		"failures": func(c *config.Config) { c.LoginMaxFailures = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := guardConfig()
			mutate(cfg)
			_, err := NewRedisLoginGuard(cfg, rdb)
			require.Error(t, err)
		})
	}
}
