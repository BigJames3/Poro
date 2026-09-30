package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
)

const (
	otpCooldownKeyPrefix = "auth:otp:cooldown:"
	otpHourlyKeyPrefix   = "auth:otp:hourly:"
)

type redisOTPThrottle struct {
	rdb        *redis.Client
	cooldown   time.Duration
	maxPerHour int
}

// NewRedisOTPThrottle enforces a cooldown between two codes and an hourly cap per phone.
func NewRedisOTPThrottle(cfg *config.Config, rdb *redis.Client) (OTPThrottle, error) {
	if rdb == nil {
		return nil, fmt.Errorf("otp throttle: redis is required")
	}
	cooldown, err := time.ParseDuration(cfg.OtpRequestCooldown)
	if err != nil {
		return nil, fmt.Errorf("parse otp request cooldown: %w", err)
	}
	if cooldown <= 0 || cfg.OtpMaxRequestsPerHour <= 0 {
		return nil, fmt.Errorf("otp throttle: cooldown and hourly cap must be positive")
	}
	return &redisOTPThrottle{rdb: rdb, cooldown: cooldown, maxPerHour: cfg.OtpMaxRequestsPerHour}, nil
}

func (t *redisOTPThrottle) Allow(ctx context.Context, phone string) error {
	cooldownKey := otpCooldownKeyPrefix + phone
	set, err := t.rdb.SetNX(ctx, cooldownKey, "1", t.cooldown).Result()
	if err != nil {
		return fmt.Errorf("otp throttle: %w", err)
	}
	if !set {
		return &ThrottledError{RetryAfter: t.ttl(ctx, cooldownKey, t.cooldown)}
	}

	hourlyKey := otpHourlyKeyPrefix + phone
	var count *redis.IntCmd
	_, err = t.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.SetNX(ctx, hourlyKey, 0, time.Hour)
		count = p.Incr(ctx, hourlyKey)
		return nil
	})
	if err != nil {
		return fmt.Errorf("otp throttle: %w", err)
	}
	if count.Val() > int64(t.maxPerHour) {
		return &ThrottledError{RetryAfter: t.ttl(ctx, hourlyKey, time.Hour)}
	}
	return nil
}

func (t *redisOTPThrottle) ttl(ctx context.Context, key string, fallback time.Duration) time.Duration {
	ttl, err := t.rdb.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		return fallback
	}
	return ttl
}

type logSMSSender struct {
	log *zap.Logger
}

// NewSMSSender returns the sender named by SMS_PROVIDER.
func NewSMSSender(cfg *config.Config, log *zap.Logger) (SMSSender, error) {
	switch cfg.SMSProvider {
	case config.SMSProviderLog:
		if !cfg.IsDev() {
			return nil, errors.New("sms provider log is only allowed in dev")
		}
		return &logSMSSender{log: log}, nil
	default:
		return nil, fmt.Errorf("sms provider %q is not supported", cfg.SMSProvider)
	}
}

// SendOTP writes the code to the log. Dev only.
func (s *logSMSSender) SendOTP(_ context.Context, phone, code string, ttl time.Duration) error {
	s.log.Info("dev sms otp", zap.String("phone", phone), zap.String("code", code), zap.Duration("ttl", ttl))
	return nil
}
