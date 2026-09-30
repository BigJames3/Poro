package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
)

func TestRedisOTPThrottleCooldownAndHourlyCap(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	throttle, err := NewRedisOTPThrottle(&config.Config{OtpRequestCooldown: "60s", OtpMaxRequestsPerHour: 2}, rdb)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, throttle.Allow(ctx, testPhone))

	err = throttle.Allow(ctx, testPhone)
	var throttled *ThrottledError
	require.ErrorAs(t, err, &throttled)
	require.ErrorIs(t, err, ErrOTPThrottled)
	require.InDelta(t, 60, throttled.RetryAfter.Seconds(), 1)

	require.NoError(t, throttle.Allow(ctx, "+2250101010101"), "phones are throttled independently")

	mr.FastForward(61 * time.Second)
	require.NoError(t, throttle.Allow(ctx, testPhone))

	mr.FastForward(61 * time.Second)
	err = throttle.Allow(ctx, testPhone)
	require.ErrorAs(t, err, &throttled, "the hourly cap applies after the cooldown")
	require.Greater(t, throttled.RetryAfter, time.Minute)

	mr.FastForward(time.Hour)
	require.NoError(t, throttle.Allow(ctx, testPhone), "the hourly window expires")
}

func TestRedisOTPThrottleErrors(t *testing.T) {
	_, err := NewRedisOTPThrottle(&config.Config{OtpRequestCooldown: "60s", OtpMaxRequestsPerHour: 5}, nil)
	require.Error(t, err)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	_, err = NewRedisOTPThrottle(&config.Config{OtpRequestCooldown: "nope", OtpMaxRequestsPerHour: 5}, rdb)
	require.Error(t, err)
	_, err = NewRedisOTPThrottle(&config.Config{OtpRequestCooldown: "60s", OtpMaxRequestsPerHour: 0}, rdb)
	require.Error(t, err)

	throttle, err := NewRedisOTPThrottle(&config.Config{OtpRequestCooldown: "60s", OtpMaxRequestsPerHour: 5}, rdb)
	require.NoError(t, err)
	mr.Close()
	err = throttle.Allow(context.Background(), testPhone)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrOTPThrottled), "a Redis outage is an internal error, not a throttle")
}

func TestNewSMSSender(t *testing.T) {
	sender, err := NewSMSSender(&config.Config{AppEnv: config.EnvDev, SMSProvider: config.SMSProviderLog}, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, sender.SendOTP(context.Background(), testPhone, "123456", time.Minute))

	_, err = NewSMSSender(&config.Config{AppEnv: config.EnvProd, SMSProvider: config.SMSProviderLog}, zap.NewNop())
	require.Error(t, err)
	_, err = NewSMSSender(&config.Config{AppEnv: config.EnvDev, SMSProvider: "carrier-pigeon"}, zap.NewNop())
	require.Error(t, err)
}
