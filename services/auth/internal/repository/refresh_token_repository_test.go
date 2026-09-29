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

func TestRefreshTokenRepository(t *testing.T) {
	ctx := context.Background()
	users := repository.NewUserRepository(testPool)
	repo := repository.NewRefreshTokenRepository(testPool)

	user := &model.User{Phone: ptr(uniquePhone())}
	require.NoError(t, users.Create(ctx, user))

	agent := "Poro/1.0"
	ip := "203.0.113.10"
	token := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: "hash-" + uuid.Must(uuid.NewV7()).String(),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
		UserAgent: &agent,
		IPAddress: &ip,
	}
	require.NoError(t, repo.Create(ctx, token))
	require.NotEqual(t, uuid.Nil, token.ID)

	stored, err := repo.GetByHash(ctx, token.TokenHash)
	require.NoError(t, err)
	require.Equal(t, token.ID, stored.ID)
	require.Equal(t, user.ID, stored.UserID)
	require.Nil(t, stored.RevokedAt)
	require.Equal(t, agent, *stored.UserAgent)

	require.NoError(t, repo.Revoke(ctx, token.ID))
	stored, err = repo.GetByHash(ctx, token.TokenHash)
	require.NoError(t, err)
	require.NotNil(t, stored.RevokedAt)
	require.NoError(t, repo.Revoke(ctx, token.ID))

	require.ErrorIs(t, repo.Revoke(ctx, uuid.Must(uuid.NewV7())), repository.ErrRefreshTokenNotFound)
	_, err = repo.GetByHash(ctx, "missing-hash")
	require.ErrorIs(t, err, repository.ErrRefreshTokenNotFound)
}

func TestRefreshTokenRepositoryRevokeAllAndDeleteExpired(t *testing.T) {
	ctx := context.Background()
	users := repository.NewUserRepository(testPool)
	repo := repository.NewRefreshTokenRepository(testPool)

	user := &model.User{Phone: ptr(uniquePhone())}
	require.NoError(t, users.Create(ctx, user))

	active := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: "active-" + uuid.Must(uuid.NewV7()).String(),
		ExpiresAt: time.Now().Add(24 * time.Hour).UTC(),
	}
	expired := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: "expired-" + uuid.Must(uuid.NewV7()).String(),
		ExpiresAt: time.Now().Add(-time.Hour).UTC(),
	}
	require.NoError(t, repo.Create(ctx, active))
	require.NoError(t, repo.Create(ctx, expired))

	require.NoError(t, repo.RevokeAllForUser(ctx, user.ID))
	stored, err := repo.GetByHash(ctx, active.TokenHash)
	require.NoError(t, err)
	require.NotNil(t, stored.RevokedAt)

	deleted, err := repo.DeleteExpired(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))
	_, err = repo.GetByHash(ctx, expired.TokenHash)
	require.ErrorIs(t, err, repository.ErrRefreshTokenNotFound)
	_, err = repo.GetByHash(ctx, active.TokenHash)
	require.NoError(t, err)
}
