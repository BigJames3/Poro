package repository_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

func TestOTPRepository(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewOTPRepository(testPool)
	phone := uniquePhone()
	now := time.Now().UTC()

	older := &model.OTPCode{Phone: phone, CodeHash: "older-hash", ExpiresAt: now.Add(5 * time.Minute), CreatedAt: now.Add(-time.Minute)}
	newer := &model.OTPCode{Phone: phone, CodeHash: "newer-hash", ExpiresAt: now.Add(5 * time.Minute), CreatedAt: now}
	require.NoError(t, repo.Create(ctx, older))
	require.NoError(t, repo.Create(ctx, newer))
	require.Equal(t, 3, newer.MaxAttempts)

	latest, err := repo.GetLatestByPhone(ctx, phone)
	require.NoError(t, err)
	require.Equal(t, newer.ID, latest.ID)
	require.Equal(t, 0, latest.Attempts)

	attempts, err := repo.ConsumeAttempt(ctx, newer.ID, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, 1, attempts)

	require.NoError(t, repo.MarkVerified(ctx, newer.ID, time.Now().UTC()))
	latest, err = repo.GetLatestByPhone(ctx, phone)
	require.NoError(t, err)
	require.NotNil(t, latest.VerifiedAt)
	require.ErrorIs(t, repo.MarkVerified(ctx, newer.ID, time.Now().UTC()), repository.ErrOTPNotFound, "a code is verified once")

	_, err = repo.ConsumeAttempt(ctx, newer.ID, time.Now().UTC())
	require.ErrorIs(t, err, repository.ErrOTPNotFound, "a verified code accepts no attempt")
	_, err = repo.ConsumeAttempt(ctx, uuid.Must(uuid.NewV7()), time.Now().UTC())
	require.ErrorIs(t, err, repository.ErrOTPNotFound)
	_, err = repo.GetLatestByPhone(ctx, uniquePhone())
	require.ErrorIs(t, err, repository.ErrOTPNotFound)
}

func TestOTPRepositoryConsumeAttemptRejectsExpired(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewOTPRepository(testPool)
	otp := &model.OTPCode{Phone: uniquePhone(), CodeHash: "h", ExpiresAt: time.Now().Add(time.Minute).UTC()}
	require.NoError(t, repo.Create(ctx, otp))

	_, err := repo.ConsumeAttempt(ctx, otp.ID, time.Now().Add(2*time.Minute).UTC())
	require.ErrorIs(t, err, repository.ErrOTPNotFound)
}

func TestOTPRepositoryConcurrentAttemptsRespectBudget(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewOTPRepository(testPool)
	otp := &model.OTPCode{Phone: uniquePhone(), CodeHash: "h", MaxAttempts: 3, ExpiresAt: time.Now().Add(time.Minute).UTC()}
	require.NoError(t, repo.Create(ctx, otp))

	var granted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.ConsumeAttempt(ctx, otp.ID, time.Now().UTC()); err == nil {
				granted.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(3), granted.Load())

	latest, err := repo.GetLatestByPhone(ctx, otp.Phone)
	require.NoError(t, err)
	require.Equal(t, 3, latest.Attempts)
}

func TestOTPRepositoryDeleteExpired(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewOTPRepository(testPool)
	phone := uniquePhone()

	expired := &model.OTPCode{Phone: phone, CodeHash: "expired-hash", ExpiresAt: time.Now().Add(-time.Minute).UTC()}
	fresh := &model.OTPCode{Phone: phone, CodeHash: "fresh-hash", ExpiresAt: time.Now().Add(5 * time.Minute).UTC(), CreatedAt: time.Now().Add(time.Second).UTC()}
	require.NoError(t, repo.Create(ctx, expired))
	require.NoError(t, repo.Create(ctx, fresh))

	deleted, err := repo.DeleteExpired(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))

	latest, err := repo.GetLatestByPhone(ctx, phone)
	require.NoError(t, err)
	require.Equal(t, fresh.ID, latest.ID)
}
