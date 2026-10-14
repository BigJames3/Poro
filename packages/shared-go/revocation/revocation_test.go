package revocation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/revocation"
)

func TestRevokeAndCheck(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	store, err := revocation.Open("redis://" + mr.Addr() + "/15")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.Ping(ctx))

	a, b, other := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, store.Revoke(ctx, 16*time.Minute, a, b))
	require.NoError(t, store.Revoke(ctx, time.Minute), "nothing to revoke is not an error")
	for _, id := range []uuid.UUID{a, b} {
		revoked, err := store.IsRevoked(ctx, id)
		require.NoError(t, err)
		require.True(t, revoked)
	}
	revoked, err := store.IsRevoked(ctx, other)
	require.NoError(t, err)
	require.False(t, revoked)

	mr.Select(15)
	require.True(t, mr.Exists("poro:revoked:sid:"+a.String()), "the key layout is shared with the Node and Python services")
	require.Equal(t, 16*time.Minute, mr.TTL(revocation.Key(a)))

	mr.FastForward(16 * time.Minute)
	revoked, err = store.IsRevoked(ctx, a)
	require.NoError(t, err)
	require.False(t, revoked, "entries expire with the access tokens")

	require.Error(t, store.Revoke(ctx, 0, a))
}

func TestUnavailableRegistry(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	store, err := revocation.Open("redis://" + mr.Addr())
	require.NoError(t, err)
	mr.Close()

	_, err = store.IsRevoked(ctx, uuid.New())
	require.True(t, errors.Is(err, revocation.ErrUnavailable))
	require.ErrorIs(t, store.Revoke(ctx, time.Minute, uuid.New()), revocation.ErrUnavailable)
	require.ErrorIs(t, store.Ping(ctx), revocation.ErrUnavailable)

	_, err = revocation.Open("http://not-redis")
	require.Error(t, err)
}

func TestCheckURL(t *testing.T) {
	require.NoError(t, revocation.CheckURL("", true), "optional for a service started alone in dev")
	require.ErrorContains(t, revocation.CheckURL("", false), "required outside dev")
	require.NoError(t, revocation.CheckURL("redis://redis:6379/15", false))
	require.NoError(t, revocation.CheckURL("rediss://user:secret@revocations:6380/0", false))
	require.ErrorContains(t, revocation.CheckURL("http://redis:6379", true), "redis://")
}
