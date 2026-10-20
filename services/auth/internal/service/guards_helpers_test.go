package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/poro/auth/internal/config"
)

// guardConfig holds the production defaults of the abuse limits (ADR-0017).
func guardConfig() *config.Config {
	return &config.Config{
		SMSDailyLimitPerPhone: 10, SMSHourlyLimitPerPrefix: 20, SMSHourlyLimitPerCountry: 1000, SMSDailyLimit: 5000,
		LoginMaxFailures: 5, LoginFailureWindow: "15m", LoginLockBase: "1m", LoginLockMax: "15m",
	}
}

// newTestLoginGuard is the real Redis login guard on an in-memory Redis.
func newTestLoginGuard(t *testing.T) (LoginGuard, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	guard, err := NewRedisLoginGuard(guardConfig(), rdb)
	require.NoError(t, err)
	return guard, mr
}

// allowSMS is an SMS budget that never refuses and records refunds.
type allowSMS struct {
	refunds   int
	delivered []string
}

func (a *allowSMS) Spend(context.Context, string, string) error { return nil }
func (a *allowSMS) Refund(context.Context, string, string) error {
	a.refunds++
	return nil
}
func (a *allowSMS) Delivered(country string) { a.delivered = append(a.delivered, country) }

func mustLoginGuard(t *testing.T) LoginGuard {
	t.Helper()
	guard, _ := newTestLoginGuard(t)
	return guard
}
