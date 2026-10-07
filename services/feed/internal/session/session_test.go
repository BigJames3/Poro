package session

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newRedis(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	mr, rdb := newRedis(t)
	store := NewStore(rdb, 30*time.Minute)
	viewer, other := uuid.New(), uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	id, err := store.Create(ctx, viewer, ids)
	require.NoError(t, err)
	got, ok, err := store.Load(ctx, viewer, id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, ids, got)

	_, ok, err = store.Load(ctx, other, id)
	require.NoError(t, err)
	require.False(t, ok, "a session is private to its viewer")

	mr.FastForward(20 * time.Minute)
	_, ok, err = store.Load(ctx, viewer, id)
	require.NoError(t, err)
	require.True(t, ok, "reading extends the session")
	mr.FastForward(20 * time.Minute)
	_, ok, err = store.Load(ctx, viewer, id)
	require.NoError(t, err)
	require.True(t, ok)

	mr.FastForward(31 * time.Minute)
	_, ok, err = store.Load(ctx, viewer, id)
	require.NoError(t, err)
	require.False(t, ok, "expired after 30 minutes without a read")
}

func TestEmptyAndCorruptSessions(t *testing.T) {
	ctx := context.Background()
	mr, rdb := newRedis(t)
	store := NewStore(rdb, time.Minute)
	viewer := uuid.New()

	id, err := store.Create(ctx, viewer, nil)
	require.NoError(t, err)
	got, ok, err := store.Load(ctx, viewer, id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, got)

	require.NoError(t, mr.Set(sessionKey(viewer, "bad"), "not-a-uuid"))
	_, ok, err = store.Load(ctx, viewer, "bad")
	require.NoError(t, err)
	require.False(t, ok, "a corrupt session is treated as expired")
}

func TestRedisDownIsAnError(t *testing.T) {
	ctx := context.Background()
	mr, rdb := newRedis(t)
	mr.Close()
	store := NewStore(rdb, time.Minute)
	_, err := store.Create(ctx, uuid.New(), nil)
	require.Error(t, err)
	_, _, err = store.Load(ctx, uuid.New(), "x")
	require.Error(t, err)
	cache := NewCache(rdb, time.Minute)
	_, _, err = cache.Get(ctx, "k")
	require.Error(t, err)
	require.Error(t, cache.Set(ctx, "k", []byte("v")))
}

func TestCache(t *testing.T) {
	ctx := context.Background()
	mr, rdb := newRedis(t)
	cache := NewCache(rdb, 60*time.Second)
	_, ok, err := cache.Get(ctx, "trending")
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, cache.Set(ctx, "trending", []byte(`{"items":[]}`)))
	got, ok, err := cache.Get(ctx, "trending")
	require.NoError(t, err)
	require.True(t, ok)
	require.JSONEq(t, `{"items":[]}`, string(got))
	mr.FastForward(61 * time.Second)
	_, ok, err = cache.Get(ctx, "trending")
	require.NoError(t, err)
	require.False(t, ok)
}
