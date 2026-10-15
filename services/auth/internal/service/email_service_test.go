package service

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

var codePattern = regexp.MustCompile(`\b[0-9]{6}\b`)

type emailFixture struct {
	svc      *emailService
	users    *fakeUsers
	codes    *fakeEmailCodes
	throttle *fakeThrottle
	sender   *fakeEmailSender
	sessions *fakeSessionRevoker
	now      time.Time
}

// fakeSessionRevoker records who was signed out everywhere, and whether the
// password had already changed at that moment.
type fakeSessionRevoker struct {
	users        []uuid.UUID
	afterChange  []bool
	passwordHash func(uuid.UUID) string
	before       map[uuid.UUID]string
	err          error
}

func (f *fakeSessionRevoker) RevokeAllSessions(_ context.Context, userID uuid.UUID, _ ...uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.users = append(f.users, userID)
	f.afterChange = append(f.afterChange, f.passwordHash(userID) != f.before[userID])
	return nil
}

func emailConfig() *config.Config {
	return &config.Config{AppEnv: config.EnvDev, EmailCodeTTL: "15m", EmailCodeMaxAttempts: 3}
}

func newEmailFixture(t *testing.T) *emailFixture {
	t.Helper()
	f := &emailFixture{
		users:    newFakeUsers(),
		throttle: &fakeThrottle{},
		sender:   &fakeEmailSender{},
		now:      time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
	}
	f.sessions = &fakeSessionRevoker{before: map[uuid.UUID]string{}, passwordHash: func(id uuid.UUID) string {
		if user := f.users.byID[id]; user != nil && user.PasswordHash != nil {
			return *user.PasswordHash
		}
		return ""
	}}
	f.codes = &fakeEmailCodes{users: f.users}
	svc, err := NewEmailService(emailConfig(), f.users, f.codes, f.throttle, f.sender, f.sessions, zap.NewNop())
	require.NoError(t, err)
	f.svc = svc.(*emailService)
	f.svc.now = func() time.Time { return f.now }
	return f
}

func (f *emailFixture) emailUser(t *testing.T, email string) *model.User {
	t.Helper()
	hash := hashPassword(t)
	user := &model.User{Email: &email, PasswordHash: &hash, Status: model.StatusActive}
	require.NoError(t, f.users.Create(context.Background(), user))
	return user
}

// lastCode reads the code from the last email sent.
func (f *emailFixture) lastCode(t *testing.T) string {
	t.Helper()
	require.NotEmpty(t, f.sender.sent)
	code := codePattern.FindString(f.sender.sent[len(f.sender.sent)-1].Body)
	require.NotEmpty(t, code)
	return code
}

func wrongCode(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

func TestNewEmailServiceRejectsBadSettings(t *testing.T) {
	cases := map[string]func(*config.Config){
		"ttl":      func(c *config.Config) { c.EmailCodeTTL = "soon" },
		"zero ttl": func(c *config.Config) { c.EmailCodeTTL = "0s" },
		"attempts": func(c *config.Config) { c.EmailCodeMaxAttempts = 0 },
		"secret":   func(c *config.Config) { c.AppEnv = config.EnvProd },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := emailConfig()
			mutate(cfg)
			_, err := NewEmailService(cfg, newFakeUsers(), &fakeEmailCodes{}, &fakeThrottle{}, &fakeEmailSender{}, &fakeSessionRevoker{}, zap.NewNop())
			require.Error(t, err)
		})
	}
	_, err := NewEmailService(emailConfig(), newFakeUsers(), &fakeEmailCodes{}, nil, &fakeEmailSender{}, &fakeSessionRevoker{}, zap.NewNop())
	require.Error(t, err)
	_, err = NewEmailService(emailConfig(), newFakeUsers(), &fakeEmailCodes{}, &fakeThrottle{}, &fakeEmailSender{}, nil, zap.NewNop())
	require.Error(t, err, "a reset must be able to sign every device out")
	cfg := emailConfig()
	cfg.AppEnv = config.EnvProd
	cfg.OtpSecret = "0123456789abcdef0123456789abcdef"
	_, err = NewEmailService(cfg, newFakeUsers(), &fakeEmailCodes{}, &fakeThrottle{}, &fakeEmailSender{}, &fakeSessionRevoker{}, zap.NewNop())
	require.NoError(t, err)
}

func TestVerifyEmail(t *testing.T) {
	f := newEmailFixture(t)
	user := f.emailUser(t, "awa@poro.test")
	ctx := context.Background()

	expiresIn, err := f.svc.RequestVerification(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 900, expiresIn)
	require.Equal(t, []string{"awa@poro.test"}, f.throttle.allowed)
	msg := f.sender.sent[0]
	require.Equal(t, "awa@poro.test", msg.To)
	require.Equal(t, "Ton code de vérification PORO", msg.Subject)
	require.Contains(t, msg.Body, "Il expire dans 15 minutes.")
	code := f.lastCode(t)
	stored := f.codes.items[0]
	require.Equal(t, model.EmailCodeVerify, stored.Purpose)
	require.NotContains(t, stored.CodeHash, code, "only a hash is stored")
	require.Equal(t, f.now.Add(15*time.Minute), stored.ExpiresAt)

	_, err = f.svc.ConfirmVerification(ctx, user.ID, wrongCode(code))
	require.ErrorIs(t, err, ErrEmailCodeInvalid)
	require.Equal(t, 1, stored.Attempts)

	res, err := f.svc.ConfirmVerification(ctx, user.ID, code)
	require.NoError(t, err)
	require.True(t, res.EmailVerified)
	require.NotNil(t, user.EmailVerifiedAt)

	_, err = f.svc.RequestVerification(ctx, user.ID)
	require.ErrorIs(t, err, ErrEmailAlreadyVerified)
	res, err = f.svc.ConfirmVerification(ctx, user.ID, code)
	require.NoError(t, err, "confirming a verified address again is harmless")
	require.True(t, res.EmailVerified)
}

func TestVerificationCodeLimits(t *testing.T) {
	f := newEmailFixture(t)
	user := f.emailUser(t, "awa@poro.test")
	ctx := context.Background()

	_, err := f.svc.ConfirmVerification(ctx, user.ID, "123456")
	require.ErrorIs(t, err, ErrEmailCodeInvalid, "no code was sent")

	_, err = f.svc.RequestVerification(ctx, user.ID)
	require.NoError(t, err)
	code := f.lastCode(t)
	for i := 0; i < 2; i++ {
		_, err = f.svc.ConfirmVerification(ctx, user.ID, wrongCode(code))
		require.ErrorIs(t, err, ErrEmailCodeInvalid)
	}
	_, err = f.svc.ConfirmVerification(ctx, user.ID, wrongCode(code))
	require.ErrorIs(t, err, ErrEmailCodeTooManyAttempts, "the last attempt is spent")
	_, err = f.svc.ConfirmVerification(ctx, user.ID, code)
	require.ErrorIs(t, err, ErrEmailCodeTooManyAttempts, "even the right code is refused now")

	_, err = f.svc.RequestVerification(ctx, user.ID)
	require.NoError(t, err)
	code = f.lastCode(t)
	f.now = f.now.Add(16 * time.Minute)
	_, err = f.svc.ConfirmVerification(ctx, user.ID, code)
	require.ErrorIs(t, err, ErrEmailCodeExpired)

	_, err = f.svc.RequestVerification(ctx, user.ID)
	require.NoError(t, err)
	code = f.lastCode(t)
	f.codes.consumeErr = repository.ErrEmailCodeNotFound
	_, err = f.svc.ConfirmVerification(ctx, user.ID, code)
	require.ErrorIs(t, err, ErrEmailCodeTooManyAttempts, "a parallel guess spent the budget")
	f.codes.consumeErr = nil

	other := "new@poro.test"
	user.Email = &other
	_, err = f.svc.ConfirmVerification(ctx, user.ID, code)
	require.ErrorIs(t, err, ErrEmailCodeInvalid, "a code sent to an older address is void")
}

func TestRequestVerificationRefusals(t *testing.T) {
	f := newEmailFixture(t)
	ctx := context.Background()

	_, err := f.svc.RequestVerification(ctx, uuid.Must(uuid.NewV7()))
	require.ErrorIs(t, err, ErrAccountNotFound)

	phone := "+2250707070707"
	phoneUser := &model.User{Phone: &phone, Status: model.StatusActive}
	require.NoError(t, f.users.Create(ctx, phoneUser))
	_, err = f.svc.RequestVerification(ctx, phoneUser.ID)
	require.ErrorIs(t, err, ErrEmailMissing)
	_, err = f.svc.ConfirmVerification(ctx, phoneUser.ID, "123456")
	require.ErrorIs(t, err, ErrEmailMissing)

	suspended := f.emailUser(t, "suspended@poro.test")
	suspended.Status = model.StatusSuspended
	_, err = f.svc.RequestVerification(ctx, suspended.ID)
	require.ErrorIs(t, err, ErrAccountBlocked)
	_, err = f.svc.ConfirmVerification(ctx, suspended.ID, "123456")
	require.ErrorIs(t, err, ErrAccountBlocked)

	user := f.emailUser(t, "awa@poro.test")
	f.throttle.err = &ThrottledError{RetryAfter: time.Minute}
	_, err = f.svc.RequestVerification(ctx, user.ID)
	require.ErrorIs(t, err, ErrOTPThrottled)
	require.Empty(t, f.sender.sent)
	f.throttle.err = nil

	f.sender.err = errors.New("relay down")
	_, err = f.svc.RequestVerification(ctx, user.ID)
	require.ErrorIs(t, err, ErrEmailUnavailable)
	require.Equal(t, []string{"awa@poro.test"}, f.throttle.released, "a failed delivery lifts the cooldown")
	f.sender.err = nil

	f.codes.createErr = errors.New("db down")
	_, err = f.svc.RequestVerification(ctx, user.ID)
	require.Error(t, err)
	f.codes.createErr = nil

	f.users.getByIDErr = errors.New("db down")
	_, err = f.svc.RequestVerification(ctx, user.ID)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrAccountNotFound)
	f.users.getByIDErr = nil

	_, err = f.svc.RequestVerification(ctx, user.ID)
	require.NoError(t, err)
	code := f.lastCode(t)
	f.codes.latestErr = errors.New("db down")
	_, err = f.svc.ConfirmVerification(ctx, user.ID, code)
	require.Error(t, err)
	f.codes.latestErr = nil
	f.codes.applyErr = repository.ErrEmailCodeNotFound
	_, err = f.svc.ConfirmVerification(ctx, user.ID, code)
	require.ErrorIs(t, err, ErrEmailCodeInvalid, "used by a parallel request")
}

func TestForgotAndResetPassword(t *testing.T) {
	f := newEmailFixture(t)
	user := f.emailUser(t, "awa@poro.test")
	ctx := context.Background()

	expiresIn, err := f.svc.ForgotPassword(ctx, "  AWA@poro.test ")
	require.NoError(t, err)
	require.Equal(t, 900, expiresIn)
	require.Equal(t, []string{"awa@poro.test"}, f.throttle.allowed, "throttled on the normalized address")
	msg := f.sender.sent[0]
	require.Equal(t, "Réinitialise ton mot de passe PORO", msg.Subject)
	require.Contains(t, msg.Body, "tous tes appareils seront déconnectés")
	code := f.lastCode(t)

	err = f.svc.ResetPassword(ctx, dto.ResetPasswordRequest{Email: "awa@poro.test", Code: wrongCode(code), NewPassword: "nouveau-mot-de-passe"})
	require.ErrorIs(t, err, ErrEmailCodeInvalid)
	require.Empty(t, f.sessions.users, "a wrong code signs nobody out")

	f.sessions.err = errors.New("registry down")
	err = f.svc.ResetPassword(ctx, dto.ResetPasswordRequest{Email: "awa@poro.test", Code: code, NewPassword: "nouveau-mot-de-passe"})
	require.Error(t, err)
	match, err := argon2id.ComparePasswordAndHash(testPassword, *user.PasswordHash)
	require.NoError(t, err)
	require.True(t, match, "the password does not change while devices stay signed in")
	f.sessions.err = nil

	f.sessions.before[user.ID] = *user.PasswordHash
	err = f.svc.ResetPassword(ctx, dto.ResetPasswordRequest{Email: "Awa@Poro.test", Code: code, NewPassword: "nouveau-mot-de-passe"})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{user.ID}, f.sessions.users, "every device is signed out, in every service")
	require.Equal(t, []bool{false}, f.sessions.afterChange, "before the password changes")
	match, err = argon2id.ComparePasswordAndHash("nouveau-mot-de-passe", *user.PasswordHash)
	require.NoError(t, err)
	require.True(t, match)
	require.NotNil(t, user.EmailVerifiedAt, "receiving the code proves the address")
	require.True(t, f.codes.sessionsRevoked)

	err = f.svc.ResetPassword(ctx, dto.ResetPasswordRequest{Email: "awa@poro.test", Code: code, NewPassword: "encore-un-autre"})
	require.ErrorIs(t, err, ErrEmailCodeInvalid, "a code works once")
}

func TestVerificationCodeCannotResetAPassword(t *testing.T) {
	f := newEmailFixture(t)
	user := f.emailUser(t, "awa@poro.test")
	ctx := context.Background()
	_, err := f.svc.RequestVerification(ctx, user.ID)
	require.NoError(t, err)
	code := f.lastCode(t)

	err = f.svc.ResetPassword(ctx, dto.ResetPasswordRequest{Email: "awa@poro.test", Code: code, NewPassword: "nouveau-mot-de-passe"})
	require.ErrorIs(t, err, ErrEmailCodeInvalid)

	// Even stored under the reset purpose, its hash is bound to verification.
	f.codes.items[0].Purpose = model.EmailCodeReset
	err = f.svc.ResetPassword(ctx, dto.ResetPasswordRequest{Email: "awa@poro.test", Code: code, NewPassword: "nouveau-mot-de-passe"})
	require.ErrorIs(t, err, ErrEmailCodeInvalid)
}

func TestForgotPasswordRevealsNoAccount(t *testing.T) {
	f := newEmailFixture(t)
	ctx := context.Background()
	phone := "+2250707070707"
	require.NoError(t, f.users.Create(ctx, &model.User{Phone: &phone, Status: model.StatusActive}))
	noPassword := "otp-only@poro.test"
	require.NoError(t, f.users.Create(ctx, &model.User{Email: &noPassword, Status: model.StatusActive}))
	suspended := f.emailUser(t, "suspended@poro.test")
	suspended.Status = model.StatusSuspended
	working := f.emailUser(t, "awa@poro.test")

	for _, email := range []string{"nobody@poro.test", noPassword, "suspended@poro.test"} {
		expiresIn, err := f.svc.ForgotPassword(ctx, email)
		require.NoError(t, err, email)
		require.Equal(t, 900, expiresIn, email)
	}
	require.Empty(t, f.sender.sent, "nothing is sent, the answer is the same")

	f.sender.err = errors.New("relay down")
	_, err := f.svc.ForgotPassword(ctx, *working.Email)
	require.NoError(t, err, "a failed delivery is not reported either")
	f.sender.err = nil

	f.throttle.err = &ThrottledError{RetryAfter: time.Minute}
	_, err = f.svc.ForgotPassword(ctx, "nobody@poro.test")
	require.ErrorIs(t, err, ErrOTPThrottled, "throttled before the lookup, whatever the address")
	f.throttle.err = nil

	f.users.getByEmailErr = errors.New("db down")
	_, err = f.svc.ForgotPassword(ctx, *working.Email)
	require.Error(t, err)
}

func TestResetPasswordRefusals(t *testing.T) {
	f := newEmailFixture(t)
	ctx := context.Background()
	req := dto.ResetPasswordRequest{Email: "nobody@poro.test", Code: "123456", NewPassword: "nouveau-mot-de-passe"}
	require.ErrorIs(t, f.svc.ResetPassword(ctx, req), ErrEmailCodeInvalid, "unknown address fails like a wrong code")

	suspended := f.emailUser(t, "suspended@poro.test")
	suspended.Status = model.StatusSuspended
	req.Email = *suspended.Email
	require.ErrorIs(t, f.svc.ResetPassword(ctx, req), ErrAccountBlocked)

	user := f.emailUser(t, "awa@poro.test")
	_, err := f.svc.ForgotPassword(ctx, *user.Email)
	require.NoError(t, err)
	req = dto.ResetPasswordRequest{Email: *user.Email, Code: f.lastCode(t), NewPassword: "nouveau-mot-de-passe"}
	f.codes.applyErr = errors.New("db down")
	require.Error(t, f.svc.ResetPassword(ctx, req))
	f.codes.applyErr = repository.ErrUserNotFound
	require.ErrorIs(t, f.svc.ResetPassword(ctx, req), ErrEmailCodeInvalid)
	f.codes.applyErr = nil

	f.users.getByEmailErr = errors.New("db down")
	require.Error(t, f.svc.ResetPassword(ctx, req))
}

func TestSendVerificationAfterSignupOnlyLogs(t *testing.T) {
	f := newEmailFixture(t)
	user := f.emailUser(t, "awa@poro.test")
	SendVerificationAfterSignup(context.Background(), f.svc, user.ID, zap.NewNop())
	require.Len(t, f.sender.sent, 1)

	f.sender.err = errors.New("relay down")
	SendVerificationAfterSignup(context.Background(), f.svc, user.ID, zap.NewNop())
}

func TestEmailForCodeTexts(t *testing.T) {
	verify := emailForCode(model.EmailCodeVerify, "awa@poro.test", "123456", 15*time.Minute)
	require.Contains(t, verify.Body, "Voici ton code pour vérifier ton adresse email sur PORO : 123456")
	reset := emailForCode(model.EmailCodeReset, "awa@poro.test", "654321", 10*time.Minute)
	require.Contains(t, reset.Body, "654321")
	require.Contains(t, reset.Body, "Il expire dans 10 minutes.")
}

type fakeEmailCodes struct {
	items           []*model.EmailCode
	createErr       error
	latestErr       error
	consumeErr      error
	applyErr        error
	sessionsRevoked bool
	users           *fakeUsers
}

func (f *fakeEmailCodes) Create(_ context.Context, code *model.EmailCode) error {
	if f.createErr != nil {
		return f.createErr
	}
	code.ID = uuid.Must(uuid.NewV7())
	f.items = append(f.items, code)
	return nil
}

func (f *fakeEmailCodes) Latest(_ context.Context, userID uuid.UUID, purpose model.EmailCodePurpose) (*model.EmailCode, error) {
	if f.latestErr != nil {
		return nil, f.latestErr
	}
	for i := len(f.items) - 1; i >= 0; i-- {
		if f.items[i].UserID == userID && f.items[i].Purpose == purpose {
			return f.items[i], nil
		}
	}
	return nil, repository.ErrEmailCodeNotFound
}

func (f *fakeEmailCodes) ConsumeAttempt(_ context.Context, id uuid.UUID, _ time.Time) (int, error) {
	if f.consumeErr != nil {
		return 0, f.consumeErr
	}
	for _, code := range f.items {
		if code.ID == id {
			code.Attempts++
			return code.Attempts, nil
		}
	}
	return 0, repository.ErrEmailCodeNotFound
}

// use mirrors the repository: consume the code, verify the address it was sent to.
func (f *fakeEmailCodes) use(code *model.EmailCode, now time.Time) (*model.User, error) {
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	if code.ConsumedAt != nil {
		return nil, repository.ErrEmailCodeNotFound
	}
	code.ConsumedAt = &now
	user := f.users.byID[code.UserID]
	if user.EmailVerifiedAt == nil {
		user.EmailVerifiedAt = &now
	}
	return user, nil
}

func (f *fakeEmailCodes) VerifyEmail(_ context.Context, code *model.EmailCode, now time.Time) error {
	_, err := f.use(code, now)
	return err
}

func (f *fakeEmailCodes) ResetPassword(_ context.Context, code *model.EmailCode, hash string, now time.Time) error {
	user, err := f.use(code, now)
	if err != nil {
		return err
	}
	user.PasswordHash = &hash
	f.sessionsRevoked = true
	return nil
}

func (f *fakeEmailCodes) DeleteExpired(context.Context, time.Time) (int64, error) { return 0, nil }

type fakeThrottle struct {
	allowed  []string
	released []string
	err      error
}

func (f *fakeThrottle) Allow(_ context.Context, recipient string) error {
	if f.err != nil {
		return f.err
	}
	f.allowed = append(f.allowed, recipient)
	return nil
}

func (f *fakeThrottle) Release(_ context.Context, recipient string) error {
	f.released = append(f.released, recipient)
	return nil
}

type fakeEmailSender struct {
	sent []EmailMessage
	err  error
}

func (f *fakeEmailSender) Send(_ context.Context, msg EmailMessage) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, msg)
	return nil
}
