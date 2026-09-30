package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

func TestRefreshTokenRepositoryCreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewRefreshTokenRepository(testPool)
	user := newTestUser(t)

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
	require.Equal(t, token.ID, token.FamilyID, "a token without family starts its own session")

	stored, err := repo.GetByHash(ctx, token.TokenHash)
	require.NoError(t, err)
	require.Equal(t, token.ID, stored.ID)
	require.Equal(t, user.ID, stored.UserID)
	require.Equal(t, token.FamilyID, stored.FamilyID)
	require.Nil(t, stored.RevokedAt)
	require.Nil(t, stored.ReplacedBy)
	require.Equal(t, agent, *stored.UserAgent)

	_, err = repo.GetByHash(ctx, "missing-hash")
	require.ErrorIs(t, err, repository.ErrRefreshTokenNotFound)
}

func TestRefreshTokenRepositoryRotationInTx(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewRefreshTokenRepository(testPool)
	user := newTestUser(t)
	first := newTestToken(t, repo, user.ID, uuid.Nil)

	var next *model.RefreshToken
	err := repo.InTx(ctx, func(tx repository.RefreshTokenTx) error {
		locked, err := tx.GetByHashForUpdate(ctx, first.TokenHash)
		if err != nil {
			return err
		}
		next = &model.RefreshToken{UserID: user.ID, FamilyID: locked.FamilyID, TokenHash: "next-" + uuid.Must(uuid.NewV7()).String(), ExpiresAt: time.Now().Add(time.Hour).UTC()}
		if err := tx.Create(ctx, next); err != nil {
			return err
		}
		return tx.MarkReplaced(ctx, locked.ID, next.ID, time.Now().UTC())
	})
	require.NoError(t, err)

	stored, err := repo.GetByHash(ctx, first.TokenHash)
	require.NoError(t, err)
	require.NotNil(t, stored.RevokedAt)
	require.Equal(t, next.ID, *stored.ReplacedBy)
	require.Equal(t, first.FamilyID, next.FamilyID)

	rollback := errors.New("abort")
	err = repo.InTx(ctx, func(tx repository.RefreshTokenTx) error {
		if _, err := tx.RevokeFamily(ctx, first.FamilyID, time.Now().UTC()); err != nil {
			return err
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	live, err := repo.GetByHash(ctx, next.TokenHash)
	require.NoError(t, err)
	require.Nil(t, live.RevokedAt, "a failed transaction leaves no revocation behind")

	err = repo.InTx(ctx, func(tx repository.RefreshTokenTx) error {
		locked, err := tx.GetByIDForUpdate(ctx, next.ID)
		if err != nil {
			return err
		}
		return tx.Revoke(ctx, locked.ID, time.Now().UTC())
	})
	require.NoError(t, err)
	live, err = repo.GetByHash(ctx, next.TokenHash)
	require.NoError(t, err)
	require.NotNil(t, live.RevokedAt)

	err = repo.InTx(ctx, func(tx repository.RefreshTokenTx) error {
		_, err := tx.GetByIDForUpdate(ctx, uuid.Must(uuid.NewV7()))
		return err
	})
	require.ErrorIs(t, err, repository.ErrRefreshTokenNotFound)
}

func TestRefreshTokenRepositoryLockSerializesRotations(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewRefreshTokenRepository(testPool)
	user := newTestUser(t)
	token := newTestToken(t, repo, user.ID, uuid.Nil)

	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := repo.InTx(ctx, func(tx repository.RefreshTokenTx) error {
				locked, err := tx.GetByHashForUpdate(ctx, token.TokenHash)
				if err != nil {
					return err
				}
				if locked.RevokedAt != nil {
					return nil
				}
				time.Sleep(10 * time.Millisecond)
				if err := tx.Revoke(ctx, locked.ID, time.Now().UTC()); err != nil {
					return err
				}
				mu.Lock()
				winners++
				mu.Unlock()
				return nil
			})
			require.NoError(t, err)
		}()
	}
	wg.Wait()
	require.Equal(t, 1, winners, "FOR UPDATE lets exactly one transaction rotate a token")
}

func TestRefreshTokenRepositoryRevokeFamilyIsScopedToUser(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewRefreshTokenRepository(testPool)
	owner := newTestUser(t)
	stranger := newTestUser(t)

	session := newTestToken(t, repo, owner.ID, uuid.Nil)
	sibling := newTestToken(t, repo, owner.ID, session.FamilyID)
	otherDevice := newTestToken(t, repo, owner.ID, uuid.Nil)

	n, err := repo.RevokeFamily(ctx, stranger.ID, session.FamilyID)
	require.NoError(t, err)
	require.Zero(t, n, "a user cannot revoke another user's session")

	n, err = repo.RevokeFamily(ctx, owner.ID, session.FamilyID)
	require.NoError(t, err)
	require.Equal(t, int64(2), n)

	for _, tok := range []*model.RefreshToken{session, sibling} {
		stored, err := repo.GetByHash(ctx, tok.TokenHash)
		require.NoError(t, err)
		require.NotNil(t, stored.RevokedAt)
	}
	stored, err := repo.GetByHash(ctx, otherDevice.TokenHash)
	require.NoError(t, err)
	require.Nil(t, stored.RevokedAt)
}

func TestRefreshTokenRepositoryRevokeAllAndDeleteExpired(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewRefreshTokenRepository(testPool)
	user := newTestUser(t)

	active := newTestToken(t, repo, user.ID, uuid.Nil)
	expired := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: "expired-" + uuid.Must(uuid.NewV7()).String(),
		ExpiresAt: time.Now().Add(-time.Hour).UTC(),
	}
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

func newTestUser(t *testing.T) *model.User {
	t.Helper()
	user := &model.User{Phone: ptr(uniquePhone())}
	require.NoError(t, repository.NewUserRepository(testPool).Create(context.Background(), user))
	return user
}

func newTestToken(t *testing.T, repo repository.RefreshTokenRepository, userID, familyID uuid.UUID) *model.RefreshToken {
	t.Helper()
	token := &model.RefreshToken{
		UserID:    userID,
		FamilyID:  familyID,
		TokenHash: "tok-" + uuid.Must(uuid.NewV7()).String(),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
	require.NoError(t, repo.Create(context.Background(), token))
	return token
}
