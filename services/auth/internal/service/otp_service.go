package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

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
	// ErrOTPThrottled is returned when a phone asks for codes too often.
	ErrOTPThrottled = errors.New("otp requests throttled")
	// ErrPhoneCountryNotSupported is returned for numbers outside the allowed calling codes.
	ErrPhoneCountryNotSupported = errors.New("phone country not supported")
	// ErrPhoneUnreachable is returned when the SMS provider permanently refuses the number.
	ErrPhoneUnreachable = errors.New("phone number cannot receive sms")
	// ErrSMSUnavailable is returned when the SMS provider fails or cannot be reached.
	ErrSMSUnavailable = errors.New("sms provider unavailable")
)

// ThrottledError carries how long the client must wait. It matches ErrOTPThrottled.
type ThrottledError struct {
	RetryAfter time.Duration
}

func (e *ThrottledError) Error() string {
	return fmt.Sprintf("%s: retry after %s", ErrOTPThrottled, e.RetryAfter)
}

// Is makes errors.Is(err, ErrOTPThrottled) true.
func (e *ThrottledError) Is(target error) bool { return target == ErrOTPThrottled }

// OTPThrottle limits how often one recipient (a phone number, or an email
// address) can receive a code. Allow returns a *ThrottledError when the
// recipient must wait. Release lifts the cooldown after a failed delivery; the
// hourly cap still counts the attempt.
type OTPThrottle interface {
	Allow(ctx context.Context, recipient string) error
	Release(ctx context.Context, recipient string) error
}

// SMSSender delivers a code to a phone number.
type SMSSender interface {
	SendOTP(ctx context.Context, phone, code string, ttl time.Duration) error
}

// OTPService issues and checks SMS one-time codes.
type OTPService interface {
	RequestOTP(ctx context.Context, phone string) (expiresIn int, err error)
	VerifyOTP(ctx context.Context, phone, code string) (bool, error)
}

type otpService struct {
	repo         repository.OTPRepository
	throttle     OTPThrottle
	budget       SMSBudget
	sender       SMSSender
	log          *zap.Logger
	secret       []byte
	ttl          time.Duration
	maxAttempts  int
	callingCodes []string
}

// NewOTPService builds an OTP service. In dev an empty OTP secret is replaced
// by a random one, so codes do not survive a restart.
func NewOTPService(cfg *config.Config, repo repository.OTPRepository, throttle OTPThrottle, budget SMSBudget, sender SMSSender, log *zap.Logger) (OTPService, error) {
	ttl, err := time.ParseDuration(cfg.OtpTTL)
	if err != nil {
		return nil, fmt.Errorf("parse otp ttl: %w", err)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("parse otp ttl: must be positive")
	}
	if throttle == nil || budget == nil || sender == nil {
		return nil, fmt.Errorf("otp service: throttle, sms budget and sender are required")
	}
	secret := []byte(cfg.OtpSecret)
	if len(secret) == 0 {
		if !cfg.IsDev() {
			return nil, fmt.Errorf("otp service: OTP_HMAC_SECRET is required outside dev")
		}
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("otp service: generate dev secret: %w", err)
		}
	}
	maxAttempts := cfg.OtpMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	return &otpService{
		repo:         repo,
		throttle:     throttle,
		budget:       budget,
		sender:       sender,
		log:          log,
		secret:       secret,
		ttl:          ttl,
		maxAttempts:  maxAttempts,
		callingCodes: cfg.OtpAllowedCallingCodes,
	}, nil
}

func (s *otpService) RequestOTP(ctx context.Context, phone string) (int, error) {
	if !s.countryAllowed(phone) {
		return 0, ErrPhoneCountryNotSupported
	}
	if err := s.throttle.Allow(ctx, phone); err != nil {
		return 0, fmt.Errorf("request otp: %w", err)
	}
	country := s.callingCode(phone)
	if err := s.budget.Spend(ctx, phone, country); err != nil {
		// Refused by the budget: no SMS goes out, so the phone may ask again
		// once the budget allows it.
		s.releaseCooldown(ctx, phone)
		if errors.Is(err, ErrSMSBudgetExhausted) {
			s.log.Error("daily sms budget exhausted: no otp is sent until tomorrow (UTC)")
		} else {
			s.log.Warn("otp refused by the sms budget", zap.String("phone", MaskPhone(phone)), zap.Error(err))
		}
		return 0, fmt.Errorf("request otp: %w", err)
	}
	code, err := newOTPCode()
	if err != nil {
		s.refund(ctx, phone, country)
		return 0, fmt.Errorf("request otp: %w", err)
	}

	now := time.Now().UTC()
	otp := &model.OTPCode{
		Phone:       phone,
		CodeHash:    s.hash(phone, code),
		MaxAttempts: s.maxAttempts,
		ExpiresAt:   now.Add(s.ttl),
		CreatedAt:   now,
	}
	if err := s.repo.Create(ctx, otp); err != nil {
		s.refund(ctx, phone, country)
		return 0, fmt.Errorf("request otp: %w", err)
	}
	if err := s.sender.SendOTP(ctx, phone, code, s.ttl); err != nil {
		s.releaseCooldown(ctx, phone)
		s.refund(ctx, phone, country)
		return 0, fmt.Errorf("request otp: send sms: %w", err)
	}
	s.budget.Delivered(country)

	expiresIn := int(s.ttl.Seconds())
	s.log.Info("otp issued", zap.String("phone", MaskPhone(phone)), zap.Int("expires_in", expiresIn))
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
	now := time.Now().UTC()
	if otp.VerifiedAt != nil {
		return false, ErrOTPInvalid
	}
	if now.After(otp.ExpiresAt) {
		return false, ErrOTPExpired
	}
	if otp.Attempts >= otp.MaxAttempts {
		return false, ErrOTPTooManyAttempts
	}

	attempts, err := s.repo.ConsumeAttempt(ctx, otp.ID, now)
	if err != nil {
		if errors.Is(err, repository.ErrOTPNotFound) {
			return false, ErrOTPTooManyAttempts
		}
		return false, fmt.Errorf("verify otp: %w", err)
	}

	if !hmac.Equal([]byte(s.hash(phone, code)), []byte(otp.CodeHash)) {
		s.log.Info("otp mismatch", zap.String("phone", MaskPhone(phone)), zap.Int("attempts", attempts))
		if attempts >= otp.MaxAttempts {
			return false, ErrOTPTooManyAttempts
		}
		return false, ErrOTPInvalid
	}

	if err := s.repo.MarkVerified(ctx, otp.ID, now); err != nil {
		if errors.Is(err, repository.ErrOTPNotFound) {
			return false, ErrOTPInvalid
		}
		return false, fmt.Errorf("verify otp: %w", err)
	}
	s.log.Info("otp verified", zap.String("phone", MaskPhone(phone)))
	return true, nil
}

// countryAllowed matches E.164 calling codes by prefix, which is unambiguous
// because calling codes are prefix-free. No configured code allows every country.
func (s *otpService) releaseCooldown(ctx context.Context, phone string) {
	if err := s.throttle.Release(ctx, phone); err != nil {
		s.log.Warn("otp cooldown release failed", zap.String("phone", MaskPhone(phone)), zap.Error(err))
	}
}

// refund gives the SMS back to the budget when it was not sent. A failure
// only makes the budget stricter, so it is logged.
func (s *otpService) refund(ctx context.Context, phone, country string) {
	if err := s.budget.Refund(ctx, phone, country); err != nil {
		s.log.Warn("sms budget refund failed", zap.String("phone", MaskPhone(phone)), zap.Error(err))
	}
}

// callingCode is the allowed calling code the phone starts with, the country
// of the budget. Without an allow list every number shares one bucket.
func (s *otpService) callingCode(phone string) string {
	for _, code := range s.callingCodes {
		if strings.HasPrefix(phone, code) {
			return code
		}
	}
	return "other"
}

func (s *otpService) countryAllowed(phone string) bool {
	if len(s.callingCodes) == 0 {
		return true
	}
	for _, code := range s.callingCodes {
		if strings.HasPrefix(phone, code) {
			return true
		}
	}
	return false
}

// hash binds the code to the phone, so a stored hash cannot be replayed for another number.
func (s *otpService) hash(phone, code string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(phone))
	mac.Write([]byte{0})
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func newOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// MaskPhone keeps the country prefix and the last two digits, for logs.
func MaskPhone(phone string) string {
	if len(phone) <= 6 {
		return "***"
	}
	return phone[:4] + "****" + phone[len(phone)-2:]
}
