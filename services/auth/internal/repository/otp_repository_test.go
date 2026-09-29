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

func TestOTPRepository(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewOTPRepository(testPool)
	phone := uniquePhone()
	now := time.Now().UTC()

	older := &model.OTPCode{
		Phone:     phone,
		CodeHash:  "older-hash",
		ExpiresAt: now.Add(5 * time.Minute),
		CreatedAt: now.Add(-time.Minute),
	}
	newer := &model.OTPCode{
		Phone:     phone,
		CodeHash:  "newer-hash",
		ExpiresAt: now.Add(5 * time.Minute),
		CreatedAt: now,
	}
	require.NoError(t, repo.Create(ctx, older))
	require.NoError(t, repo.Create(ctx, newer))
	require.Equal(t, 3, newer.MaxAttempts)

	latest, err := repo.GetLatestByPhone(ctx, phone)
	require.NoError(t, err)
	require.Equal(t, newer.ID, latest.ID)
	require.Equal(t, 0, latest.Attempts)

	require.NoError(t, repo.IncrementAttempts(ctx, newer.ID))
	latest, err = repo.GetLatestByPhone(ctx, phone)
	require.NoError(t, err)
	require.Equal(t, 1, latest.Attempts)

	require.NoError(t, repo.MarkVerified(ctx, newer.ID))
	latest, err = repo.GetLatestByPhone(ctx, phone)
	require.NoError(t, err)
	require.NotNil(t, latest.VerifiedAt)
	require.NoError(t, repo.MarkVerified(ctx, newer.ID))

	require.ErrorIs(t, repo.IncrementAttempts(ctx, uuid.Must(uuid.NewV7())), repository.ErrOTPNotFound)
	_, err = repo.GetLatestByPhone(ctx, uniquePhone())
	require.ErrorIs(t, err, repository.ErrOTPNotFound)
}

func TestOTPRepositoryDeleteExpired(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewOTPRepository(testPool)
	phone := uniquePhone()

	expired := &model.OTPCode{
		Phone:     phone,
		CodeHash:  "expired-hash",
		ExpiresAt: time.Now().Add(-time.Minute).UTC(),
	}
	fresh := &model.OTPCode{
		Phone:     phone,
		CodeHash:  "fresh-hash",
		ExpiresAt: time.Now().Add(5 * time.Minute).UTC(),
		CreatedAt: time.Now().Add(time.Second).UTC(),
	}
	require.NoError(t, repo.Create(ctx, expired))
	require.NoError(t, repo.Create(ctx, fresh))

	deleted, err := repo.DeleteExpired(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))

	latest, err := repo.GetLatestByPhone(ctx, phone)
	require.NoError(t, err)
	require.Equal(t, fresh.ID, latest.ID)
}
