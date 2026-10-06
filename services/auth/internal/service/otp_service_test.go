package service

import (
	"context"
	"errors"
	"sync"
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

const testPhone = "+2250707070707"

func TestOTPServiceIssuesAndVerifies(t *testing.T) {
	repo := &memOTP{}
	sender := &captureSender{}
	core, logs := observer.New(zap.InfoLevel)
	svc := newOTP(t, devOTPConfig(), repo, allowAll{}, sender, zap.New(core))

	expires, err := svc.RequestOTP(context.Background(), testPhone)
	require.NoError(t, err)
	require.Equal(t, 300, expires)
	require.Equal(t, 3, repo.latest.MaxAttempts)
	require.Len(t, sender.code, 6)
	require.NotContains(t, repo.latest.CodeHash, sender.code)
	for _, entry := range logs.All() {
		_, hasCode := entry.ContextMap()["code"]
		require.False(t, hasCode, "the service never logs the code")
		require.NotEqual(t, testPhone, entry.ContextMap()["phone"], "phones are masked")
	}

	verified, err := svc.VerifyOTP(context.Background(), testPhone, sender.code)
	require.NoError(t, err)
	require.True(t, verified)
	require.NotNil(t, repo.latest.VerifiedAt)

	_, err = svc.VerifyOTP(context.Background(), testPhone, sender.code)
	require.ErrorIs(t, err, ErrOTPInvalid, "a code is single use")
}

func TestOTPServiceCodeIsBoundToPhone(t *testing.T) {
	repo := &memOTP{}
	sender := &captureSender{}
	svc := newOTP(t, devOTPConfig(), repo, allowAll{}, sender, zap.NewNop())
	_, err := svc.RequestOTP(context.Background(), testPhone)
	require.NoError(t, err)

	repo.latest.Phone = "+2250101010101"
	_, err = svc.VerifyOTP(context.Background(), "+2250101010101", sender.code)
	require.ErrorIs(t, err, ErrOTPInvalid)
}

func TestOTPServiceRequestFailures(t *testing.T) {
	t.Run("throttled", func(t *testing.T) {
		repo := &memOTP{}
		svc := newOTP(t, devOTPConfig(), repo, denyAll{}, &captureSender{}, zap.NewNop())
		_, err := svc.RequestOTP(context.Background(), testPhone)
		require.ErrorIs(t, err, ErrOTPThrottled)
		require.Nil(t, repo.latest, "a throttled request stores nothing")
	})

	t.Run("store error", func(t *testing.T) {
		svc := newOTP(t, devOTPConfig(), &memOTP{createErr: errors.New("db")}, allowAll{}, &captureSender{}, zap.NewNop())
		_, err := svc.RequestOTP(context.Background(), testPhone)
		require.Error(t, err)
	})

	t.Run("sms error releases the cooldown", func(t *testing.T) {
		throttle := &recordingThrottle{}
		svc := newOTP(t, devOTPConfig(), &memOTP{}, throttle, &captureSender{err: ErrSMSUnavailable}, zap.NewNop())
		_, err := svc.RequestOTP(context.Background(), testPhone)
		require.ErrorIs(t, err, ErrSMSUnavailable)
		require.Equal(t, []string{testPhone}, throttle.released)
	})

	t.Run("release failure keeps the send error", func(t *testing.T) {
		throttle := &recordingThrottle{releaseErr: errors.New("redis down")}
		svc := newOTP(t, devOTPConfig(), &memOTP{}, throttle, &captureSender{err: ErrPhoneUnreachable}, zap.NewNop())
		_, err := svc.RequestOTP(context.Background(), testPhone)
		require.ErrorIs(t, err, ErrPhoneUnreachable)
	})
}

func TestOTPServiceCountryAllowlist(t *testing.T) {
	cfg := devOTPConfig()
	cfg.OtpAllowedCallingCodes = []string{"+225", "+221", "+237", "+234"}
	repo := &memOTP{}
	sender := &captureSender{}
	svc := newOTP(t, cfg, repo, denyAll{}, sender, zap.NewNop())

	for _, phone := range []string{"+33612345678", "+2547000000", "+1234567890"} {
		_, err := svc.RequestOTP(context.Background(), phone)
		require.ErrorIs(t, err, ErrPhoneCountryNotSupported, phone)
	}
	require.Nil(t, repo.latest, "a refused country stores nothing")
	require.Empty(t, sender.code, "a refused country sends nothing")

	for _, phone := range []string{"+2250707070707", "+221771234567", "+237650000000", "+2348012345678"} {
		_, err := svc.RequestOTP(context.Background(), phone)
		require.ErrorIs(t, err, ErrOTPThrottled, "%s passes the allowlist and reaches the throttle", phone)
	}
}

func TestOTPServiceVerifyFailures(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		svc := newOTP(t, devOTPConfig(), &memOTP{}, allowAll{}, &captureSender{}, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), testPhone, "123456")
		require.ErrorIs(t, err, ErrOTPInvalid)
	})

	t.Run("store error", func(t *testing.T) {
		svc := newOTP(t, devOTPConfig(), &memOTP{getErr: errors.New("db")}, allowAll{}, &captureSender{}, zap.NewNop())
		_, err := svc.VerifyOTP(context.Background(), testPhone, "123456")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrOTPInvalid)
	})

	t.Run("expired", func(t *testing.T) {
		svc, repo, sender := issuedOTP(t)
		repo.latest.ExpiresAt = time.Now().UTC().Add(-time.Minute)
		_, err := svc.VerifyOTP(context.Background(), testPhone, sender.code)
		require.ErrorIs(t, err, ErrOTPExpired)
	})

	t.Run("attempt budget spent", func(t *testing.T) {
		svc, repo, sender := issuedOTP(t)
		repo.latest.Attempts = repo.latest.MaxAttempts
		_, err := svc.VerifyOTP(context.Background(), testPhone, sender.code)
		require.ErrorIs(t, err, ErrOTPTooManyAttempts)
	})

	t.Run("wrong code then exhausted", func(t *testing.T) {
		svc, repo, sender := issuedOTP(t)
		wrong := otherCode(sender.code)
		_, err := svc.VerifyOTP(context.Background(), testPhone, wrong)
		require.ErrorIs(t, err, ErrOTPInvalid)
		_, err = svc.VerifyOTP(context.Background(), testPhone, wrong)
		require.ErrorIs(t, err, ErrOTPInvalid)
		_, err = svc.VerifyOTP(context.Background(), testPhone, wrong)
		require.ErrorIs(t, err, ErrOTPTooManyAttempts)
		_, err = svc.VerifyOTP(context.Background(), testPhone, sender.code)
		require.ErrorIs(t, err, ErrOTPTooManyAttempts, "the right code is refused once the budget is spent")
		require.Equal(t, 3, repo.latest.Attempts)
	})

	t.Run("consume error", func(t *testing.T) {
		svc, repo, sender := issuedOTP(t)
		repo.consumeErr = errors.New("db")
		_, err := svc.VerifyOTP(context.Background(), testPhone, sender.code)
		require.Error(t, err)
	})

	t.Run("mark error", func(t *testing.T) {
		svc, repo, sender := issuedOTP(t)
		repo.markErr = errors.New("db")
		_, err := svc.VerifyOTP(context.Background(), testPhone, sender.code)
		require.Error(t, err)
	})
}

func TestOTPServiceParallelGuessesRespectBudget(t *testing.T) {
	svc, repo, sender := issuedOTP(t)
	wrong := otherCode(sender.code)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.VerifyOTP(context.Background(), testPhone, wrong)
		}()
	}
	wg.Wait()
	require.Equal(t, 3, repo.attempts(), "parallel guesses cannot exceed max attempts")
}

func TestNewOTPServiceValidation(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
	}{
		{name: "bad ttl", cfg: &config.Config{AppEnv: config.EnvDev, OtpTTL: "nope"}},
		{name: "zero ttl", cfg: &config.Config{AppEnv: config.EnvDev, OtpTTL: "0s"}},
		{name: "no secret in prod", cfg: &config.Config{AppEnv: config.EnvProd, OtpTTL: "5m"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewOTPService(tc.cfg, &memOTP{}, allowAll{}, &captureSender{}, zap.NewNop())
			require.Error(t, err)
		})
	}
	_, err := NewOTPService(devOTPConfig(), &memOTP{}, nil, &captureSender{}, zap.NewNop())
	require.Error(t, err)
}

func TestMaskPhone(t *testing.T) {
	require.Equal(t, "+225****07", MaskPhone("+2250707070707"))
	require.Equal(t, "***", MaskPhone("+225"))
}

func devOTPConfig() *config.Config {
	return &config.Config{AppEnv: config.EnvDev, OtpTTL: "5m", OtpMaxAttempts: 3}
}

func newOTP(t *testing.T, cfg *config.Config, repo *memOTP, throttle OTPThrottle, sender SMSSender, log *zap.Logger) OTPService {
	t.Helper()
	svc, err := NewOTPService(cfg, repo, throttle, sender, log)
	require.NoError(t, err)
	return svc
}

func issuedOTP(t *testing.T) (OTPService, *memOTP, *captureSender) {
	t.Helper()
	repo := &memOTP{}
	sender := &captureSender{}
	svc := newOTP(t, &config.Config{AppEnv: config.EnvProd, OtpTTL: "5m", OtpSecret: "0123456789abcdef0123456789abcdef"}, repo, allowAll{}, sender, zap.NewNop())
	_, err := svc.RequestOTP(context.Background(), testPhone)
	require.NoError(t, err)
	return svc, repo, sender
}

func otherCode(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

type allowAll struct{}

func (allowAll) Allow(context.Context, string) error   { return nil }
func (allowAll) Release(context.Context, string) error { return nil }

type denyAll struct{}

func (denyAll) Allow(context.Context, string) error   { return &ThrottledError{RetryAfter: time.Minute} }
func (denyAll) Release(context.Context, string) error { return nil }

type recordingThrottle struct {
	released   []string
	releaseErr error
}

func (*recordingThrottle) Allow(context.Context, string) error { return nil }
func (r *recordingThrottle) Release(_ context.Context, phone string) error {
	r.released = append(r.released, phone)
	return r.releaseErr
}

type captureSender struct {
	code string
	err  error
}

func (s *captureSender) SendOTP(_ context.Context, _, code string, _ time.Duration) error {
	s.code = code
	return s.err
}

// memOTP mirrors the SQL semantics of the Postgres repository, including the
// atomic attempt check.
type memOTP struct {
	mu         sync.Mutex
	latest     *model.OTPCode
	getErr     error
	createErr  error
	consumeErr error
	markErr    error
}

func (m *memOTP) attempts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.latest.Attempts
}

func (m *memOTP) Create(_ context.Context, otp *model.OTPCode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	if otp.ID == uuid.Nil {
		otp.ID = uuid.Must(uuid.NewV7())
	}
	cp := *otp
	m.latest = &cp
	return nil
}

func (m *memOTP) GetLatestByPhone(_ context.Context, phone string) (*model.OTPCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.latest == nil || m.latest.Phone != phone {
		return nil, repository.ErrOTPNotFound
	}
	cp := *m.latest
	return &cp, nil
}

func (m *memOTP) ConsumeAttempt(_ context.Context, id uuid.UUID, now time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.consumeErr != nil {
		return 0, m.consumeErr
	}
	otp := m.latest
	if otp == nil || otp.ID != id || otp.VerifiedAt != nil || otp.Attempts >= otp.MaxAttempts || !otp.ExpiresAt.After(now) {
		return 0, repository.ErrOTPNotFound
	}
	otp.Attempts++
	return otp.Attempts, nil
}

func (m *memOTP) MarkVerified(_ context.Context, id uuid.UUID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.markErr != nil {
		return m.markErr
	}
	if m.latest == nil || m.latest.ID != id || m.latest.VerifiedAt != nil {
		return repository.ErrOTPNotFound
	}
	m.latest.VerifiedAt = &now
	return nil
}

func (m *memOTP) DeleteExpired(context.Context) (int64, error) { return 0, nil }
