package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

func TestOTPServiceIssuesAndVerifies(t *testing.T) {
	repo := &memOTP{}
	core, logs := observer.New(zap.InfoLevel)
	svc := newOTP(t, &config.Config{OtpTTL: "5m", OtpMaxAttempts: 3, AppEnv: "dev"}, repo, zap.New(core))

	expires, err := svc.RequestOTP(context.Background(), "+2250707070707")
	require.NoError(t, err)
	require.Equal(t, 300, expires)
	require.Equal(t, 3, repo.latest.MaxAttempts)
	code, ok := logs.All()[0].ContextMap()["code"].(string)
	require.True(t, ok)
	require.Len(t, code, 6)

	verified, err := svc.VerifyOTP(context.Background(), "+2250707070707", code)
	require.NoError(t, err)
	require.True(t, verified)
	require.NotNil(t, repo.latest.VerifiedAt)
}

func TestOTPServiceHidesCodeOutsideDev(t *testing.T) {
	repo := &memOTP{}
	core, logs := observer.New(zap.InfoLevel)
	svc := newOTP(t, &config.Config{OtpTTL: "5m", AppEnv: "prod"}, repo, zap.New(core))
	_, err := svc.RequestOTP(context.Background(), "+2250707070707")
	require.NoError(t, err)
	_, ok := logs.All()[0].ContextMap()["code"]
	require.False(t, ok)
	require.Equal(t, 3, repo.latest.MaxAttempts)
}

func TestOTPServiceVerifyFailures(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		svc := newOTP(t, &config.Config{OtpTTL: "5m"}, &memOTP{}, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), "+2250707070707", "123456")
		require.ErrorIs(t, err, ErrOTPInvalid)
	})

	t.Run("store error", func(t *testing.T) {
		repo := &memOTP{getErr: errors.New("db")}
		svc := newOTP(t, &config.Config{OtpTTL: "5m"}, repo, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), "+2250707070707", "123456")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrOTPInvalid)
	})

	t.Run("already used", func(t *testing.T) {
		repo := issuedOTP(t)
		now := time.Now().UTC()
		repo.latest.VerifiedAt = &now
		svc := newOTP(t, &config.Config{OtpTTL: "5m"}, repo, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), repo.latest.Phone, "000000")
		require.ErrorIs(t, err, ErrOTPInvalid)
	})

	t.Run("expired", func(t *testing.T) {
		repo := issuedOTP(t)
		repo.latest.ExpiresAt = time.Now().UTC().Add(-time.Minute)
		svc := newOTP(t, &config.Config{OtpTTL: "5m"}, repo, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), repo.latest.Phone, "000000")
		require.ErrorIs(t, err, ErrOTPExpired)
	})

	t.Run("attempt budget spent", func(t *testing.T) {
		repo := issuedOTP(t)
		repo.latest.Attempts = repo.latest.MaxAttempts
		svc := newOTP(t, &config.Config{OtpTTL: "5m"}, repo, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), repo.latest.Phone, "000000")
		require.ErrorIs(t, err, ErrOTPTooManyAttempts)
	})

	t.Run("wrong code", func(t *testing.T) {
		repo := issuedOTP(t)
		svc := newOTP(t, &config.Config{OtpTTL: "5m"}, repo, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), repo.latest.Phone, otherCode(t))
		require.ErrorIs(t, err, ErrOTPInvalid)
		require.Equal(t, 1, repo.increments)
	})

	t.Run("wrong code exhausts budget", func(t *testing.T) {
		repo := issuedOTP(t)
		repo.latest.MaxAttempts = 1
		svc := newOTP(t, &config.Config{OtpTTL: "5m"}, repo, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), repo.latest.Phone, otherCode(t))
		require.ErrorIs(t, err, ErrOTPTooManyAttempts)
	})

	t.Run("bad hash", func(t *testing.T) {
		repo := issuedOTP(t)
		repo.latest.CodeHash = "not-a-hash"
		svc := newOTP(t, &config.Config{OtpTTL: "5m"}, repo, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), repo.latest.Phone, "123456")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrOTPInvalid)
	})

	t.Run("increment failed", func(t *testing.T) {
		repo := issuedOTP(t)
		repo.incErr = errors.New("db")
		svc := newOTP(t, &config.Config{OtpTTL: "5m"}, repo, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), repo.latest.Phone, otherCode(t))
		require.Error(t, err)
	})

	t.Run("mark failed", func(t *testing.T) {
		repo := &memOTP{}
		core, logs := observer.New(zap.InfoLevel)
		svc := newOTP(t, &config.Config{OtpTTL: "5m", AppEnv: "dev"}, repo, zap.New(core))
		_, err := svc.RequestOTP(context.Background(), "+2250707070707")
		require.NoError(t, err)
		repo.markErr = errors.New("db")
		code := logs.All()[0].ContextMap()["code"].(string)
		_, err = svc.VerifyOTP(context.Background(), "+2250707070707", code)
		require.Error(t, err)
	})
}

func TestOTPServiceRequestFailure(t *testing.T) {
	_, err := NewOTPService(&config.Config{OtpTTL: "nope"}, &memOTP{}, zap.NewNop())
	require.Error(t, err)
	_, err = NewOTPService(&config.Config{OtpTTL: "0s"}, &memOTP{}, zap.NewNop())
	require.Error(t, err)

	repo := &memOTP{createErr: errors.New("db")}
	svc := newOTP(t, &config.Config{OtpTTL: "5m"}, repo, zap.NewNop())
	_, err = svc.RequestOTP(context.Background(), "+2250707070707")
	require.Error(t, err)
}

func newOTP(t *testing.T, cfg *config.Config, repo *memOTP, log *zap.Logger) OTPService {
	t.Helper()
	svc, err := NewOTPService(cfg, repo, log)
	require.NoError(t, err)
	return svc
}

func issuedOTP(t *testing.T) *memOTP {
	t.Helper()
	repo := &memOTP{}
	svc := newOTP(t, &config.Config{OtpTTL: "5m", AppEnv: "prod"}, repo, zap.NewNop())
	_, err := svc.RequestOTP(context.Background(), "+2250707070707")
	require.NoError(t, err)
	return repo
}

func otherCode(t *testing.T) string {
	t.Helper()
	return "000000"
}

type memOTP struct {
	latest     *model.OTPCode
	getErr     error
	createErr  error
	incErr     error
	markErr    error
	increments int
}

func (m *memOTP) Create(_ context.Context, otp *model.OTPCode) error {
	if m.createErr != nil {
		return m.createErr
	}
	if otp.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		otp.ID = id
	}
	cp := *otp
	m.latest = &cp
	return nil
}

func (m *memOTP) GetLatestByPhone(_ context.Context, phone string) (*model.OTPCode, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.latest == nil || m.latest.Phone != phone {
		return nil, repository.ErrOTPNotFound
	}
	cp := *m.latest
	return &cp, nil
}

func (m *memOTP) IncrementAttempts(_ context.Context, id uuid.UUID) error {
	if m.incErr != nil {
		return m.incErr
	}
	if m.latest == nil || m.latest.ID != id {
		return repository.ErrOTPNotFound
	}
	m.latest.Attempts++
	m.increments++
	return nil
}

func (m *memOTP) MarkVerified(_ context.Context, id uuid.UUID) error {
	if m.markErr != nil {
		return m.markErr
	}
	if m.latest == nil || m.latest.ID != id {
		return repository.ErrOTPNotFound
	}
	now := time.Now().UTC()
	m.latest.VerifiedAt = &now
	return nil
}

func (m *memOTP) DeleteExpired(context.Context) (int64, error) { return 0, nil }
