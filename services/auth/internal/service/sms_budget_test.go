package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type budgetFixture struct {
	budget *redisSMSBudget
	mr     *miniredis.Miniredis
	now    time.Time
}

func newBudgetFixture(t *testing.T, mutate func(*redisSMSBudget)) *budgetFixture {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	b, err := NewRedisSMSBudget(guardConfig(), rdb, prometheus.NewRegistry())
	require.NoError(t, err)
	f := &budgetFixture{budget: b.(*redisSMSBudget), mr: mr, now: time.Date(2026, 10, 15, 14, 20, 0, 0, time.UTC)}
	f.budget.now = func() time.Time { return f.now }
	if mutate != nil {
		mutate(f.budget)
	}
	return f
}

func TestSMSBudgetCapsPerPhoneEachDay(t *testing.T) {
	ctx := context.Background()
	f := newBudgetFixture(t, func(b *redisSMSBudget) { b.limits[capPhoneDaily] = 2 })
	phone := "+2250707070707"
	require.NoError(t, f.budget.Spend(ctx, phone, "+225"))
	require.NoError(t, f.budget.Spend(ctx, phone, "+225"))

	err := f.budget.Spend(ctx, phone, "+225")
	var throttled *ThrottledError
	require.ErrorAs(t, err, &throttled)
	require.ErrorIs(t, err, ErrOTPThrottled)
	require.Equal(t, 9*time.Hour+40*time.Minute, throttled.RetryAfter, "until the end of the UTC day")
	require.NoError(t, f.budget.Spend(ctx, "+2250707070708", "+225"), "another number has its own allowance")
	require.Equal(t, 1.0, testutil.ToFloat64(f.budget.blocked.WithLabelValues("phone_daily")))

	f.now = f.now.Add(10 * time.Hour)
	require.NoError(t, f.budget.Spend(ctx, phone, "+225"), "a new day, a new allowance")
}

func TestSMSBudgetCapsNumberRangesAndCountriesEachHour(t *testing.T) {
	ctx := context.Background()
	f := newBudgetFixture(t, func(b *redisSMSBudget) {
		b.limits[capPrefixHourly] = 2
		b.limits[capCountryHourly] = 3
	})
	// Consecutive numbers rented for SMS pumping share their range.
	require.NoError(t, f.budget.Spend(ctx, "+2250707070001", "+225"))
	require.NoError(t, f.budget.Spend(ctx, "+2250707070002", "+225"))
	err := f.budget.Spend(ctx, "+2250707070003", "+225")
	var throttled *ThrottledError
	require.ErrorAs(t, err, &throttled)
	require.Equal(t, 40*time.Minute, throttled.RetryAfter, "until the end of the hour")

	require.NoError(t, f.budget.Spend(ctx, "+2250505050001", "+225"))
	err = f.budget.Spend(ctx, "+2250101010001", "+225")
	require.ErrorAs(t, err, &throttled, "the country spent its hourly allowance")
	require.NoError(t, f.budget.Spend(ctx, "+221770000001", "+221"), "another country is not affected")
	require.Equal(t, 1.0, testutil.ToFloat64(f.budget.blocked.WithLabelValues("prefix_hourly")))
	require.Equal(t, 1.0, testutil.ToFloat64(f.budget.blocked.WithLabelValues("country_hourly")))

	f.now = f.now.Add(40 * time.Minute)
	require.NoError(t, f.budget.Spend(ctx, "+2250707070003", "+225"), "a new hour, a new allowance")
}

func TestSMSBudgetStopsEverySMSOnceTheDailyBudgetIsSpent(t *testing.T) {
	ctx := context.Background()
	f := newBudgetFixture(t, func(b *redisSMSBudget) { b.limits[capGlobalDaily] = 2 })
	require.NoError(t, f.budget.Spend(ctx, "+2250707070707", "+225"))
	require.NoError(t, f.budget.Spend(ctx, "+221770000001", "+221"))
	err := f.budget.Spend(ctx, "+237650000001", "+237")
	require.ErrorIs(t, err, ErrSMSBudgetExhausted)
	require.ErrorIs(t, err, ErrSMSUnavailable, "answered like a provider outage")
	require.Equal(t, 1.0, testutil.ToFloat64(f.budget.blocked.WithLabelValues("global_daily")))
}

func TestSMSBudgetCountsNothingWhenItRefuses(t *testing.T) {
	ctx := context.Background()
	f := newBudgetFixture(t, func(b *redisSMSBudget) { b.limits[capCountryHourly] = 1 })
	require.NoError(t, f.budget.Spend(ctx, "+2250707070707", "+225"))
	require.Error(t, f.budget.Spend(ctx, "+2250505050505", "+225"))
	day := f.now.Format("20060102")
	got, err := f.mr.Get("auth:sms:phone:+2250505050505:" + day)
	if err == nil {
		require.Equal(t, "0", got, "a refused SMS is not counted against the other caps")
	}
	global, err := f.mr.Get("auth:sms:global:" + day)
	require.NoError(t, err)
	require.Equal(t, "1", global)
}

func TestSMSBudgetRefund(t *testing.T) {
	ctx := context.Background()
	f := newBudgetFixture(t, func(b *redisSMSBudget) { b.limits[capPhoneDaily] = 1 })
	phone := "+2250707070707"
	require.NoError(t, f.budget.Spend(ctx, phone, "+225"))
	require.NoError(t, f.budget.Refund(ctx, phone, "+225"))
	require.NoError(t, f.budget.Spend(ctx, phone, "+225"), "an SMS that was not sent does not count")

	other := newBudgetFixture(t, nil)
	require.NoError(t, other.budget.Refund(ctx, phone, "+225"), "refunding before any spend is harmless")
	require.False(t, other.mr.Exists("auth:sms:global:"+other.now.Format("20060102")), "counters never go below zero")

	f.budget.Delivered("+225")
	require.Equal(t, 1.0, testutil.ToFloat64(f.budget.sent.WithLabelValues("+225")))
}

func TestSMSBudgetKeysExpireWithTheirWindow(t *testing.T) {
	ctx := context.Background()
	f := newBudgetFixture(t, nil)
	require.NoError(t, f.budget.Spend(ctx, "+2250707070707", "+225"))
	require.Equal(t, 9*time.Hour+41*time.Minute, f.mr.TTL("auth:sms:global:20261015"))
	require.Equal(t, 41*time.Minute, f.mr.TTL("auth:sms:country:+225:2026101514"))
}

func TestSMSBudgetFailsWhenRedisDoes(t *testing.T) {
	ctx := context.Background()
	f := newBudgetFixture(t, nil)
	f.mr.Close()
	require.Error(t, f.budget.Spend(ctx, "+2250707070707", "+225"))
	require.Error(t, f.budget.Refund(ctx, "+2250707070707", "+225"))
}

func TestNewRedisSMSBudgetChecksItsSettings(t *testing.T) {
	_, err := NewRedisSMSBudget(guardConfig(), nil, nil)
	require.Error(t, err)

	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := guardConfig()
	cfg.SMSDailyLimit = 0
	_, err = NewRedisSMSBudget(cfg, rdb, nil)
	require.ErrorContains(t, err, "global_daily")

	registry := prometheus.NewRegistry()
	_, err = NewRedisSMSBudget(guardConfig(), rdb, registry)
	require.NoError(t, err)
	_, err = NewRedisSMSBudget(guardConfig(), rdb, registry)
	require.Error(t, err, "metrics are registered once")
}

func TestNumberRange(t *testing.T) {
	require.Equal(t, "+2250707070", numberRange("+2250707070707"))
	require.Equal(t, "+22507", numberRange("+22507"))
}
