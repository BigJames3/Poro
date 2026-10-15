package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

var (
	// ErrEmailMissing is returned when the account has no email address.
	ErrEmailMissing = errors.New("account has no email")
	// ErrEmailAlreadyVerified is returned when the address is already verified.
	ErrEmailAlreadyVerified = errors.New("email already verified")
	// ErrEmailCodeInvalid is returned when an email code is wrong, missing or already used.
	ErrEmailCodeInvalid = errors.New("invalid email code")
	// ErrEmailCodeExpired is returned when the latest email code is past its lifetime.
	ErrEmailCodeExpired = errors.New("email code expired")
	// ErrEmailCodeTooManyAttempts is returned when the attempt budget of the code is spent.
	ErrEmailCodeTooManyAttempts = errors.New("too many email code attempts")
)

// EmailService verifies email addresses and resets passwords with codes sent by email.
type EmailService interface {
	RequestVerification(ctx context.Context, userID uuid.UUID) (expiresIn int, err error)
	ConfirmVerification(ctx context.Context, userID uuid.UUID, code string) (*dto.UserResponse, error)
	ForgotPassword(ctx context.Context, email string) (expiresIn int, err error)
	ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) error
}

type emailService struct {
	users       repository.UserRepository
	codes       repository.EmailCodeRepository
	throttle    OTPThrottle
	sender      EmailSender
	logins      LoginGuard
	log         *zap.Logger
	secret      []byte
	ttl         time.Duration
	maxAttempts int
	now         func() time.Time
}

// NewEmailService wires email codes. It shares OTP_HMAC_SECRET with SMS codes:
// the hash binds each code to its purpose, so the two never collide.
func NewEmailService(cfg *config.Config, users repository.UserRepository, codes repository.EmailCodeRepository, throttle OTPThrottle, sender EmailSender, logins LoginGuard, log *zap.Logger) (EmailService, error) {
	ttl, err := time.ParseDuration(cfg.EmailCodeTTL)
	if err != nil || ttl <= 0 {
		return nil, fmt.Errorf("email service: EMAIL_CODE_TTL must be a positive duration, got %q", cfg.EmailCodeTTL)
	}
	if cfg.EmailCodeMaxAttempts <= 0 {
		return nil, errors.New("email service: EMAIL_CODE_MAX_ATTEMPTS must be positive")
	}
	if throttle == nil || sender == nil || logins == nil {
		return nil, errors.New("email service: throttle, sender and login guard are required")
	}
	secret := []byte(cfg.OtpSecret)
	if len(secret) == 0 {
		if !cfg.IsDev() {
			return nil, errors.New("email service: OTP_HMAC_SECRET is required outside dev")
		}
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("email service: generate dev secret: %w", err)
		}
	}
	return &emailService{
		users:       users,
		codes:       codes,
		throttle:    throttle,
		sender:      sender,
		logins:      logins,
		log:         log,
		secret:      secret,
		ttl:         ttl,
		maxAttempts: cfg.EmailCodeMaxAttempts,
		now:         func() time.Time { return time.Now().UTC() },
	}, nil
}

// RequestVerification sends a verification code to the caller's address.
func (s *emailService) RequestVerification(ctx context.Context, userID uuid.UUID) (int, error) {
	user, err := s.activeUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("request email verification: %w", err)
	}
	if user.Email == nil {
		return 0, ErrEmailMissing
	}
	if user.EmailVerifiedAt != nil {
		return 0, ErrEmailAlreadyVerified
	}
	if err := s.issue(ctx, user, model.EmailCodeVerify); err != nil {
		return 0, fmt.Errorf("request email verification: %w", err)
	}
	return int(s.ttl.Seconds()), nil
}

func (s *emailService) ConfirmVerification(ctx context.Context, userID uuid.UUID, code string) (*dto.UserResponse, error) {
	user, err := s.activeUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("confirm email verification: %w", err)
	}
	if user.Email == nil {
		return nil, ErrEmailMissing
	}
	if user.EmailVerifiedAt != nil {
		res := toUserResponse(user)
		return &res, nil
	}
	stored, err := s.check(ctx, user, model.EmailCodeVerify, code)
	if err != nil {
		return nil, fmt.Errorf("confirm email verification: %w", err)
	}
	now := s.now()
	if err := s.codes.VerifyEmail(ctx, stored, now); err != nil {
		return nil, fmt.Errorf("confirm email verification: %w", usedCode(err))
	}
	user.EmailVerifiedAt = &now
	s.log.Info("email verified", zap.String("user_id", user.ID.String()))
	res := toUserResponse(user)
	return &res, nil
}

// ForgotPassword sends a reset code when the address belongs to an active
// account with a password. It answers the same way otherwise, and does not
// report delivery failures either: the response reveals no account.
func (s *emailService) ForgotPassword(ctx context.Context, email string) (int, error) {
	expiresIn := int(s.ttl.Seconds())
	email = NormalizeEmail(email)
	if err := s.throttle.Allow(ctx, email); err != nil {
		return 0, fmt.Errorf("forgot password: %w", err)
	}
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return expiresIn, nil
		}
		return 0, fmt.Errorf("forgot password: %w", err)
	}
	if blocked(user.Status) || user.PasswordHash == nil {
		return expiresIn, nil
	}
	if err := s.send(ctx, user, model.EmailCodeReset); err != nil {
		s.log.Error("password reset email failed", zap.String("user_id", user.ID.String()), zap.Error(err))
	}
	return expiresIn, nil
}

// ResetPassword sets a new password with a reset code, verifies the address
// and signs every device out. Unknown addresses fail like a wrong code.
func (s *emailService) ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) error {
	user, err := s.users.GetByEmail(ctx, NormalizeEmail(req.Email))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrEmailCodeInvalid
		}
		return fmt.Errorf("reset password: %w", err)
	}
	if blocked(user.Status) {
		return ErrAccountBlocked
	}
	stored, err := s.check(ctx, user, model.EmailCodeReset, req.Code)
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	hash, err := argon2id.CreateHash(req.NewPassword, passwordParams)
	if err != nil {
		return fmt.Errorf("reset password: hash password: %w", err)
	}
	if err := s.codes.ResetPassword(ctx, stored, hash, s.now()); err != nil {
		return fmt.Errorf("reset password: %w", usedCode(err))
	}
	// The owner proved the address: a lock left by guessing attempts no longer
	// stands between them and the new password.
	if err := s.logins.Reset(ctx, *user.Email); err != nil {
		s.log.Warn("login lock reset failed after a password reset", zap.String("user_id", user.ID.String()), zap.Error(err))
	}
	s.log.Info("password reset, sessions revoked", zap.String("user_id", user.ID.String()))
	return nil
}

// SendVerificationAfterSignup is the best-effort email sent right after an
// email sign-up. Failures are logged: the user can ask again later.
func SendVerificationAfterSignup(ctx context.Context, emails EmailService, userID uuid.UUID, log *zap.Logger) {
	if _, err := emails.RequestVerification(ctx, userID); err != nil {
		log.Warn("verification email after sign-up failed", zap.String("user_id", userID.String()), zap.Error(err))
	}
}

func (s *emailService) activeUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	if blocked(user.Status) {
		return nil, ErrAccountBlocked
	}
	return user, nil
}

// issue throttles per address, then sends a new code. A failed delivery
// lifts the cooldown so the user may retry at once.
func (s *emailService) issue(ctx context.Context, user *model.User, purpose model.EmailCodePurpose) error {
	if err := s.throttle.Allow(ctx, *user.Email); err != nil {
		return err
	}
	if err := s.send(ctx, user, purpose); err != nil {
		if relErr := s.throttle.Release(ctx, *user.Email); relErr != nil {
			s.log.Warn("email cooldown release failed", zap.String("user_id", user.ID.String()), zap.Error(relErr))
		}
		return err
	}
	return nil
}

func (s *emailService) send(ctx context.Context, user *model.User, purpose model.EmailCodePurpose) error {
	code, err := newOTPCode()
	if err != nil {
		return err
	}
	now := s.now()
	stored := &model.EmailCode{
		UserID:      user.ID,
		Purpose:     purpose,
		Email:       *user.Email,
		CodeHash:    s.hash(user.ID, *user.Email, purpose, code),
		MaxAttempts: s.maxAttempts,
		ExpiresAt:   now.Add(s.ttl),
		CreatedAt:   now,
	}
	if err := s.codes.Create(ctx, stored); err != nil {
		return err
	}
	msg := emailForCode(purpose, *user.Email, code, s.ttl)
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("%w: %w", ErrEmailUnavailable, err)
	}
	s.log.Info("email code sent", zap.String("user_id", user.ID.String()), zap.String("purpose", string(purpose)))
	return nil
}

// check spends one attempt on the latest code of purpose and compares it.
func (s *emailService) check(ctx context.Context, user *model.User, purpose model.EmailCodePurpose, code string) (*model.EmailCode, error) {
	stored, err := s.codes.Latest(ctx, user.ID, purpose)
	if err != nil {
		if errors.Is(err, repository.ErrEmailCodeNotFound) {
			return nil, ErrEmailCodeInvalid
		}
		return nil, err
	}
	now := s.now()
	switch {
	case stored.ConsumedAt != nil:
		return nil, ErrEmailCodeInvalid
	case user.Email == nil || stored.Email != *user.Email:
		return nil, ErrEmailCodeInvalid
	case !now.Before(stored.ExpiresAt):
		return nil, ErrEmailCodeExpired
	case stored.Attempts >= stored.MaxAttempts:
		return nil, ErrEmailCodeTooManyAttempts
	}
	attempts, err := s.codes.ConsumeAttempt(ctx, stored.ID, now)
	if err != nil {
		if errors.Is(err, repository.ErrEmailCodeNotFound) {
			return nil, ErrEmailCodeTooManyAttempts
		}
		return nil, err
	}
	if !hmac.Equal([]byte(s.hash(user.ID, stored.Email, purpose, code)), []byte(stored.CodeHash)) {
		if attempts >= stored.MaxAttempts {
			return nil, ErrEmailCodeTooManyAttempts
		}
		return nil, ErrEmailCodeInvalid
	}
	return stored, nil
}

// hash binds the code to the account, the address and the purpose: a
// verification code cannot reset a password, and an old address's code is void.
func (s *emailService) hash(userID uuid.UUID, email string, purpose model.EmailCodePurpose, code string) string {
	mac := hmac.New(sha256.New, s.secret)
	for _, part := range []string{"email", string(purpose), userID.String(), email, code} {
		mac.Write([]byte(part))
		mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// usedCode turns a code consumed by a parallel request into a wrong code.
func usedCode(err error) error {
	if errors.Is(err, repository.ErrEmailCodeNotFound) || errors.Is(err, repository.ErrUserNotFound) {
		return ErrEmailCodeInvalid
	}
	return err
}
