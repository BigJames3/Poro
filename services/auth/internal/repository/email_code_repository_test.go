package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

func newEmailAccount(t *testing.T) *model.User {
	t.Helper()
	hash := "old-hash"
	user := &model.User{Email: ptr(uniqueEmail()), PasswordHash: &hash, Status: model.StatusActive}
	require.NoError(t, repository.NewUserRepository(testPool).Create(context.Background(), user))
	return user
}

func newEmailCode(t *testing.T, user *model.User, purpose model.EmailCodePurpose, createdAt time.Time) *model.EmailCode {
	t.Helper()
	code := &model.EmailCode{
		UserID: user.ID, Purpose: purpose, Email: *user.Email, CodeHash: "hash-" + uuid.Must(uuid.NewV7()).String(),
		MaxAttempts: 2, ExpiresAt: createdAt.Add(15 * time.Minute), CreatedAt: createdAt,
	}
	require.NoError(t, repository.NewEmailCodeRepository(testPool).Create(context.Background(), code))
	return code
}

func TestEmailCodesLatestAndAttempts(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewEmailCodeRepository(testPool)
	user := newEmailAccount(t)
	now := time.Now().UTC()

	_, err := repo.Latest(ctx, user.ID, model.EmailCodeVerify)
	require.ErrorIs(t, err, repository.ErrEmailCodeNotFound)

	newEmailCode(t, user, model.EmailCodeVerify, now.Add(-time.Minute))
	latest := newEmailCode(t, user, model.EmailCodeVerify, now)
	reset := newEmailCode(t, user, model.EmailCodeReset, now.Add(time.Second))

	got, err := repo.Latest(ctx, user.ID, model.EmailCodeVerify)
	require.NoError(t, err)
	require.Equal(t, latest.ID, got.ID, "the newest code of the purpose")
	require.Equal(t, *user.Email, got.Email)
	require.Equal(t, 2, got.MaxAttempts)
	got, err = repo.Latest(ctx, user.ID, model.EmailCodeReset)
	require.NoError(t, err)
	require.Equal(t, reset.ID, got.ID)

	attempts, err := repo.ConsumeAttempt(ctx, latest.ID, now)
	require.NoError(t, err)
	require.Equal(t, 1, attempts)
	attempts, err = repo.ConsumeAttempt(ctx, latest.ID, now)
	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	_, err = repo.ConsumeAttempt(ctx, latest.ID, now)
	require.ErrorIs(t, err, repository.ErrEmailCodeNotFound, "the budget is spent")
	_, err = repo.ConsumeAttempt(ctx, reset.ID, now.Add(time.Hour))
	require.ErrorIs(t, err, repository.ErrEmailCodeNotFound, "expired")
}

func TestVerifyEmailMarksTheAddressOnce(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewEmailCodeRepository(testPool)
	users := repository.NewUserRepository(testPool)
	user := newEmailAccount(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	code := newEmailCode(t, user, model.EmailCodeVerify, now)

	require.NoError(t, repo.VerifyEmail(ctx, code, now))
	stored, err := users.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, now.Equal(*stored.EmailVerifiedAt))

	require.ErrorIs(t, repo.VerifyEmail(ctx, code, now.Add(time.Minute)), repository.ErrEmailCodeNotFound, "a code works once")

	stale := newEmailCode(t, user, model.EmailCodeVerify, now)
	stale.Email = "old-" + *user.Email
	require.ErrorIs(t, repo.VerifyEmail(ctx, stale, now), repository.ErrUserNotFound, "only the account's current address")
	again, err := repo.Latest(ctx, user.ID, model.EmailCodeVerify)
	require.NoError(t, err)
	require.Nil(t, again.ConsumedAt, "the failed transaction left the code unused")
}

func TestResetPasswordRevokesEverySession(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewEmailCodeRepository(testPool)
	users := repository.NewUserRepository(testPool)
	refresh := repository.NewRefreshTokenRepository(testPool)
	user := newEmailAccount(t)
	other := newEmailAccount(t)
	now := time.Now().UTC()
	for _, owner := range []*model.User{user, user, other} {
		require.NoError(t, refresh.Create(ctx, &model.RefreshToken{
			UserID: owner.ID, TokenHash: "hash-" + uuid.Must(uuid.NewV7()).String(), ExpiresAt: now.Add(time.Hour),
		}))
	}
	code := newEmailCode(t, user, model.EmailCodeReset, now)

	require.NoError(t, repo.ResetPassword(ctx, code, "new-hash", now))
	stored, err := users.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "new-hash", *stored.PasswordHash)
	require.NotNil(t, stored.EmailVerifiedAt)
	sessions, err := refresh.ActiveSessions(ctx, user.ID)
	require.NoError(t, err)
	require.Empty(t, sessions, "every device signs in again")
	sessions, err = refresh.ActiveSessions(ctx, other.ID)
	require.NoError(t, err)
	require.Len(t, sessions, 1, "other accounts keep theirs")

	require.ErrorIs(t, repo.ResetPassword(ctx, code, "another-hash", now), repository.ErrEmailCodeNotFound)
	stored, err = users.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "new-hash", *stored.PasswordHash)
}

func TestExpiredEmailCodesAndErasedAccounts(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewEmailCodeRepository(testPool)
	user := newEmailAccount(t)
	now := time.Now().UTC()
	old := newEmailCode(t, user, model.EmailCodeVerify, now.Add(-48*time.Hour))
	fresh := newEmailCode(t, user, model.EmailCodeReset, now)

	deleted, err := repo.DeleteExpired(ctx, now.Add(-24*time.Hour))
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))
	var n int
	require.NoError(t, testPool.QueryRow(ctx, `SELECT count(*) FROM email_codes WHERE id = $1`, old.ID).Scan(&n))
	require.Zero(t, n)
	require.NoError(t, testPool.QueryRow(ctx, `SELECT count(*) FROM email_codes WHERE id = $1`, fresh.ID).Scan(&n))
	require.Equal(t, 1, n)

	_, err = repository.NewUserRepository(testPool).ScheduleDeletion(ctx, user.ID, now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = repository.NewAccountPurger(testPool).PurgeDue(ctx, now, 100)
	require.NoError(t, err)
	require.NoError(t, testPool.QueryRow(ctx, `SELECT count(*) FROM email_codes WHERE user_id = $1`, user.ID).Scan(&n))
	require.Zero(t, n, "an erased account keeps no address in its codes")
	var verified *time.Time
	require.NoError(t, testPool.QueryRow(ctx, `SELECT email_verified_at FROM users WHERE id = $1`, user.ID).Scan(&verified))
	require.Nil(t, verified)
}
