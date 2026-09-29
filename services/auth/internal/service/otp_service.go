package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/alexedwards/argon2id"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

var (
	// ErrOTPInvalid is returned when the code is wrong, missing, or already used.
	ErrOTPInvalid = errors.New("invalid otp")
	// ErrOTPExpired is returned when the latest code is past its lifetime.
	ErrOTPExpired = errors.New("otp expired")
	// ErrOTPTooManyAttempts is returned when the attempt budget is exhausted.
	ErrOTPTooManyAttempts = errors.New("too many otp attempts")
)

// OTPService issues and checks SMS one-time codes.
type OTPService interface {
	RequestOTP(ctx context.Context, phone string) (expiresIn int, err error)
	VerifyOTP(ctx context.Context, phone, code string) (bool, error)
}

type otpService struct {
	repo        repository.OTPRepository
	log         *zap.Logger
	ttl         time.Duration
	maxAttempts int
	dev         bool
}

// NewOTPService builds an OTP service. The SMS provider is a log stub.
// The raw code is logged only when APP_ENV is dev.
func NewOTPService(cfg *config.Config, repo repository.OTPRepository, log *zap.Logger) (OTPService, error) {
	ttl, err := time.ParseDuration(cfg.OtpTTL)
	if err != nil {
		return nil, fmt.Errorf("parse otp ttl: %w", err)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("parse otp ttl: must be positive")
	}
	maxAttempts := cfg.OtpMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	return &otpService{
		repo:        repo,
		log:         log,
		ttl:         ttl,
		maxAttempts: maxAttempts,
		dev:         cfg.AppEnv == "dev",
	}, nil
}

func (s *otpService) RequestOTP(ctx context.Context, phone string) (int, error) {
	code, err := newOTPCode()
	if err != nil {
		return 0, fmt.Errorf("request otp: %w", err)
	}
	hash, err := argon2id.CreateHash(code, argon2id.DefaultParams)
	if err != nil {
		return 0, fmt.Errorf("request otp: hash code: %w", err)
	}

	now := time.Now().UTC()
	otp := &model.OTPCode{
		Phone:       phone,
		CodeHash:    hash,
		MaxAttempts: s.maxAttempts,
		ExpiresAt:   now.Add(s.ttl),
		CreatedAt:   now,
	}
	if err := s.repo.Create(ctx, otp); err != nil {
		return 0, fmt.Errorf("request otp: %w", err)
	}

	expiresIn := int(s.ttl.Seconds())
	fields := []zap.Field{
		zap.String("phone", phone),
		zap.Int("expires_in", expiresIn),
	}
	if s.dev {
		fields = append(fields, zap.String("code", code))
	}
	s.log.Info("otp issued", fields...)
	return expiresIn, nil
}

func (s *otpService) VerifyOTP(ctx context.Context, phone, code string) (bool, error) {
	otp, err := s.repo.GetLatestByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, repository.ErrOTPNotFound) {
			return false, ErrOTPInvalid
		}
		return false, fmt.Errorf("verify otp: %w", err)
	}
	if otp.VerifiedAt != nil {
		return false, ErrOTPInvalid
	}
	if time.Now().UTC().After(otp.ExpiresAt) {
		return false, ErrOTPExpired
	}
	if otp.Attempts >= otp.MaxAttempts {
		return false, ErrOTPTooManyAttempts
	}

	match, err := argon2id.ComparePasswordAndHash(code, otp.CodeHash)
	if err != nil {
		return false, fmt.Errorf("verify otp: compare code: %w", err)
	}
	if !match {
		if err := s.repo.IncrementAttempts(ctx, otp.ID); err != nil {
			return false, fmt.Errorf("verify otp: %w", err)
		}
		if otp.Attempts+1 >= otp.MaxAttempts {
			return false, ErrOTPTooManyAttempts
		}
		return false, ErrOTPInvalid
	}

	if err := s.repo.MarkVerified(ctx, otp.ID); err != nil {
		return false, fmt.Errorf("verify otp: %w", err)
	}
	s.log.Info("otp verified", zap.String("phone", phone))
	return true, nil
}

func newOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
