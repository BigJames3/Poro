package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/model"
)

// scriptedBudget refuses with err and records refunds and deliveries.
type scriptedBudget struct {
	err       error
	refunds   int
	delivered []string
}

func (b *scriptedBudget) Spend(context.Context, string, string) error { return b.err }
func (b *scriptedBudget) Refund(context.Context, string, string) error {
	b.refunds++
	return nil
}
func (b *scriptedBudget) Delivered(country string) { b.delivered = append(b.delivered, country) }

func newBudgetedOTP(t *testing.T, cfg *config.Config, repo *memOTP, throttle OTPThrottle, budget SMSBudget, sender SMSSender) OTPService {
	t.Helper()
	svc, err := NewOTPService(cfg, repo, throttle, budget, sender, zap.NewNop())
	require.NoError(t, err)
	return svc
}

func TestRequestOTPHonoursTheSMSBudget(t *testing.T) {
	ctx := context.Background()
	cfg := devOTPConfig()
	cfg.OtpAllowedCallingCodes = []string{"+225", "+221"}

	t.Run("delivered", func(t *testing.T) {
		budget := &scriptedBudget{}
		svc := newBudgetedOTP(t, cfg, &memOTP{}, allowAll{}, budget, &captureSender{})
		_, err := svc.RequestOTP(ctx, testPhone)
		require.NoError(t, err)
		require.Equal(t, []string{"+225"}, budget.delivered, "counted under its calling code")
		require.Zero(t, budget.refunds)
	})

	t.Run("refused by a cap", func(t *testing.T) {
		throttle := &recordingThrottle{}
		repo, sender := &memOTP{}, &captureSender{}
		budget := &scriptedBudget{err: &ThrottledError{RetryAfter: time.Hour}}
		svc := newBudgetedOTP(t, cfg, repo, throttle, budget, sender)
		_, err := svc.RequestOTP(ctx, testPhone)
		require.ErrorIs(t, err, ErrOTPThrottled)
		require.Nil(t, repo.latest, "no code is stored")
		require.Empty(t, sender.code, "no SMS goes out")
		require.Equal(t, []string{testPhone}, throttle.released, "the cooldown does not punish the phone")
	})

	t.Run("daily budget exhausted", func(t *testing.T) {
		sender := &captureSender{}
		svc := newBudgetedOTP(t, cfg, &memOTP{}, allowAll{}, &scriptedBudget{err: ErrSMSBudgetExhausted}, sender)
		_, err := svc.RequestOTP(ctx, testPhone)
		require.ErrorIs(t, err, ErrSMSUnavailable)
		require.Empty(t, sender.code)
	})

	t.Run("provider failure refunds the budget", func(t *testing.T) {
		budget := &scriptedBudget{}
		svc := newBudgetedOTP(t, cfg, &memOTP{}, allowAll{}, budget, &captureSender{err: ErrSMSUnavailable})
		_, err := svc.RequestOTP(ctx, testPhone)
		require.ErrorIs(t, err, ErrSMSUnavailable)
		require.Equal(t, 1, budget.refunds)
		require.Empty(t, budget.delivered)
	})

	t.Run("store failure refunds the budget", func(t *testing.T) {
		budget := &scriptedBudget{}
		svc := newBudgetedOTP(t, cfg, &memOTP{createErr: errors.New("db")}, allowAll{}, budget, &captureSender{})
		_, err := svc.RequestOTP(ctx, testPhone)
		require.Error(t, err)
		require.Equal(t, 1, budget.refunds)
	})

	t.Run("without an allow list every number shares one bucket", func(t *testing.T) {
		budget := &scriptedBudget{}
		svc := newBudgetedOTP(t, devOTPConfig(), &memOTP{}, allowAll{}, budget, &captureSender{})
		_, err := svc.RequestOTP(ctx, testPhone)
		require.NoError(t, err)
		require.Equal(t, []string{"other"}, budget.delivered)
	})

	_, err := NewOTPService(cfg, &memOTP{}, allowAll{}, nil, &captureSender{}, zap.NewNop())
	require.Error(t, err, "the budget is required")
}

func TestLoginEmailLocksTheAccountWhateverTheIP(t *testing.T) {
	ctx := context.Background()
	users := newFakeUsers()
	hash := hashPassword(t)
	email := "awa@poro.test"
	require.NoError(t, users.Create(ctx, &model.User{Email: &email, PasswordHash: &hash, Status: model.StatusActive}))
	guard, mr := newTestLoginGuard(t)
	svc, err := NewAuthService(&config.Config{JWTAccessTTL: "15m", JWTRefreshTTL: "720h"},
		users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{}, guard, zap.NewNop())
	require.NoError(t, err)
	login := func(address, password, ip string) error {
		_, err := svc.LoginEmail(ctx, dto.LoginEmailRequest{Email: address, Password: password}, "test", ip)
		return err
	}

	for i := range 5 {
		require.ErrorIs(t, login(email, "wrong-password", "10.0.0."+string(rune('1'+i))), ErrInvalidCredentials)
	}
	err = login(email, testPassword, "10.0.0.9")
	var locked *LoginLockedError
	require.ErrorAs(t, err, &locked, "locked even with the right password, from a new IP")
	require.Equal(t, time.Minute, locked.RetryAfter)

	// An unknown address locks the same way: the answer reveals nothing.
	for range 5 {
		require.ErrorIs(t, login("nobody@poro.test", "guess", "10.0.1.1"), ErrInvalidCredentials)
	}
	require.ErrorIs(t, login("nobody@poro.test", "guess", "10.0.1.1"), ErrLoginLocked)

	mr.FastForward(time.Minute)
	require.NoError(t, login("AWA@poro.test", testPassword, "10.0.0.9"), "the lock lifts on time")
	for range 4 {
		require.ErrorIs(t, login(email, "wrong-password", "10.0.0.9"), ErrInvalidCredentials)
	}
	require.NoError(t, login(email, testPassword, "10.0.0.9"), "a success reset the count")

	_, err = NewAuthService(&config.Config{JWTAccessTTL: "15m", JWTRefreshTTL: "720h"},
		users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{}, nil, zap.NewNop())
	require.Error(t, err, "the login guard is required")

	mr.Close()
	require.Error(t, login(email, testPassword, "10.0.0.9"), "an unchecked lock fails closed")
}

func TestDeleteAccountPasswordShareTheSignInLock(t *testing.T) {
	ctx := context.Background()
	f := newAccountFixture(t)
	hash := hashPassword(t)
	user := f.addUser(t, &model.User{Email: ptrTo("awa@poro.test"), PasswordHash: &hash})
	current := f.refresh.seed(t, user.ID, "current")
	claims := claimsFor(user, current.FamilyID)

	for range 5 {
		_, err := f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{Password: "wrong-password"})
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
	_, err := f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{Password: testPassword})
	require.ErrorIs(t, err, ErrLoginLocked, "a stolen session cannot guess the password either")
	require.Nil(t, user.DeletionScheduledAt)
	require.ErrorIs(t, f.logins.Check(ctx, "awa@poro.test"), ErrLoginLocked, "sign-in is locked too")
}

func TestResetPasswordLiftsTheLock(t *testing.T) {
	ctx := context.Background()
	f := newEmailFixture(t)
	f.emailUser(t, "awa@poro.test")
	for range 5 {
		require.NoError(t, f.logins.Fail(ctx, "awa@poro.test"))
	}
	require.ErrorIs(t, f.logins.Check(ctx, "awa@poro.test"), ErrLoginLocked)

	_, err := f.svc.ForgotPassword(ctx, "awa@poro.test")
	require.NoError(t, err)
	require.NoError(t, f.svc.ResetPassword(ctx, dto.ResetPasswordRequest{Email: "awa@poro.test", Code: f.lastCode(t), NewPassword: "nouveau-mot-de-passe"}))
	require.NoError(t, f.logins.Check(ctx, "awa@poro.test"), "the owner proved the address")
}

func TestPasswordAccount(t *testing.T) {
	email := "awa@poro.test"
	user := &model.User{Email: &email}
	require.Equal(t, email, passwordAccount(user))
	user.Email = nil
	require.Equal(t, "user:"+user.ID.String(), passwordAccount(user))
}
