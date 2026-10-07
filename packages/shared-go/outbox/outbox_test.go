package outbox_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/internal/testinfra"
	"github.com/poro/shared-go/outbox"
)

type fakePublisher struct {
	mu   sync.Mutex
	sent []outbox.Message
	err  error
}

func (f *fakePublisher) Publish(_ context.Context, msgs []outbox.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, msgs...)
	return nil
}

func (f *fakePublisher) messages() []outbox.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]outbox.Message(nil), f.sent...)
}

func newEvent(t *testing.T, subject string) events.Envelope {
	t.Helper()
	env, err := events.New(events.TypeAuthUserCreated, 1, "auth", subject, map[string]string{"user_id": subject}, time.Now())
	require.NoError(t, err)
	return env
}

func TestOutbox(t *testing.T) {
	pool := testinfra.Postgres(t)
	ctx := context.Background()

	t.Run("enqueue follows the transaction", func(t *testing.T) {
		reset(t, pool)
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		require.NoError(t, outbox.Enqueue(ctx, tx, newEvent(t, "rolled-back")))
		require.NoError(t, tx.Rollback(ctx))
		require.Equal(t, 0, pending(t, pool))

		tx, err = pool.Begin(ctx)
		require.NoError(t, err)
		require.NoError(t, outbox.Enqueue(ctx, tx, newEvent(t, "kept")))
		require.NoError(t, tx.Commit(ctx))
		require.Equal(t, 1, pending(t, pool))

		require.ErrorIs(t, outbox.Enqueue(ctx, pool, events.Envelope{}), events.ErrInvalidEnvelope)
	})

	t.Run("relay publishes in order once", func(t *testing.T) {
		reset(t, pool)
		var want []uuid.UUID
		for i := 0; i < 5; i++ {
			env := newEvent(t, "user-1")
			want = append(want, env.ID)
			require.NoError(t, outbox.Enqueue(ctx, pool, env))
		}
		pub := &fakePublisher{}
		reg := prometheus.NewRegistry()
		relay, err := outbox.NewRelay(pool, pub, zap.NewNop(), outbox.RelayConfig{BatchSize: 3}, reg)
		require.NoError(t, err)

		n, err := relay.PublishBatch(ctx)
		require.NoError(t, err)
		require.Equal(t, 3, n)
		n, err = relay.PublishBatch(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, n)
		n, err = relay.PublishBatch(ctx)
		require.NoError(t, err)
		require.Zero(t, n)

		sent := pub.messages()
		require.Len(t, sent, 5)
		for i, m := range sent {
			require.Equal(t, want[i], m.ID, "creation order is kept")
			require.Equal(t, events.TypeAuthUserCreated, m.Topic)
			require.Equal(t, "user-1", string(m.Key))
			env, err := events.Decode(m.Value)
			require.NoError(t, err, "the value is the full envelope")
			require.Equal(t, want[i], env.ID)
		}
		require.Zero(t, pending(t, pool))

		_, err = outbox.NewRelay(pool, pub, zap.NewNop(), outbox.RelayConfig{}, reg)
		require.Error(t, err, "metrics cannot be registered twice")
	})

	t.Run("failed publish is recorded and retried", func(t *testing.T) {
		reset(t, pool)
		require.NoError(t, outbox.Enqueue(ctx, pool, newEvent(t, "user-2")))
		pub := &fakePublisher{err: errors.New("broker down " + strings.Repeat("x", 2000))}
		relay, err := outbox.NewRelay(pool, pub, zap.NewNop(), outbox.RelayConfig{}, nil)
		require.NoError(t, err)

		_, err = relay.PublishBatch(ctx)
		require.ErrorContains(t, err, "broker down")
		var attempts int
		var lastErr string
		require.NoError(t, pool.QueryRow(ctx, `SELECT attempts, last_error FROM outbox_events`).Scan(&attempts, &lastErr))
		require.Equal(t, 1, attempts)
		require.Len(t, lastErr, 1000, "stored errors are truncated")

		pub.err = nil
		n, err := relay.PublishBatch(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, n)
	})

	t.Run("only one relay publishes at a time", func(t *testing.T) {
		reset(t, pool)
		require.NoError(t, outbox.Enqueue(ctx, pool, newEvent(t, "user-3")))
		holder, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = holder.Rollback(ctx) }()
		var locked bool
		require.NoError(t, holder.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, int64(0x706f726f6f7574)).Scan(&locked))
		require.True(t, locked)

		pub := &fakePublisher{}
		relay, err := outbox.NewRelay(pool, pub, zap.NewNop(), outbox.RelayConfig{}, nil)
		require.NoError(t, err)
		n, err := relay.PublishBatch(ctx)
		require.NoError(t, err)
		require.Zero(t, n)
		require.Empty(t, pub.messages())

		require.NoError(t, holder.Rollback(ctx))
		n, err = relay.PublishBatch(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, n)
	})

	t.Run("cleanup keeps recent rows", func(t *testing.T) {
		reset(t, pool)
		for _, age := range []string{"100 hours", "1 hour"} {
			env := newEvent(t, "user-4")
			require.NoError(t, outbox.Enqueue(ctx, pool, env))
			_, err := pool.Exec(ctx, `UPDATE outbox_events SET published_at = now() - $2::interval WHERE id = $1`, env.ID, age)
			require.NoError(t, err)
		}
		require.NoError(t, outbox.Enqueue(ctx, pool, newEvent(t, "user-4")))
		relay, err := outbox.NewRelay(pool, &fakePublisher{}, zap.NewNop(), outbox.RelayConfig{Retention: 72 * time.Hour}, nil)
		require.NoError(t, err)
		deleted, err := relay.Cleanup(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(1), deleted)
		require.Equal(t, 1, pending(t, pool), "unpublished rows are never deleted")
	})

	t.Run("run loop drains and stops", func(t *testing.T) {
		reset(t, pool)
		for i := 0; i < 7; i++ {
			require.NoError(t, outbox.Enqueue(ctx, pool, newEvent(t, "user-5")))
		}
		pub := &fakePublisher{}
		relay, err := outbox.NewRelay(pool, pub, zap.NewNop(), outbox.RelayConfig{BatchSize: 2, PollInterval: 10 * time.Millisecond, CleanupInterval: time.Nanosecond}, nil)
		require.NoError(t, err)
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- relay.Run(runCtx) }()
		require.Eventually(t, func() bool { return len(pub.messages()) == 7 }, 10*time.Second, 20*time.Millisecond)
		cancel()
		require.NoError(t, <-done)
	})

	t.Run("run loop survives failures", func(t *testing.T) {
		reset(t, pool)
		require.NoError(t, outbox.Enqueue(ctx, pool, newEvent(t, "user-6")))
		pub := &fakePublisher{err: errors.New("down")}
		relay, err := outbox.NewRelay(pool, pub, zap.NewNop(), outbox.RelayConfig{ErrorBackoff: 10 * time.Millisecond}, nil)
		require.NoError(t, err)
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- relay.Run(runCtx) }()
		require.Eventually(t, func() bool {
			var attempts int
			_ = pool.QueryRow(ctx, `SELECT attempts FROM outbox_events`).Scan(&attempts)
			return attempts >= 2
		}, 10*time.Second, 20*time.Millisecond)
		cancel()
		require.NoError(t, <-done)
	})
}

func TestInboxClaim(t *testing.T) {
	pool := testinfra.Postgres(t)
	ctx := context.Background()
	id := uuid.Must(uuid.NewV7())

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	first, err := inbox.Claim(ctx, tx, "poro-user-profiles", id)
	require.NoError(t, err)
	require.True(t, first)
	require.NoError(t, tx.Rollback(ctx))

	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	first, err = inbox.Claim(ctx, tx, "poro-user-profiles", id)
	require.NoError(t, err)
	require.True(t, first, "a rolled-back claim does not count")
	again, err := inbox.Claim(ctx, tx, "poro-user-profiles", id)
	require.NoError(t, err)
	require.False(t, again)
	require.NoError(t, tx.Commit(ctx))

	other, err := inbox.Claim(ctx, pool, "poro-auth-roles", id)
	require.NoError(t, err)
	require.True(t, other, "each consumer group tracks its own events")

	_, err = inbox.Claim(ctx, pool, strings.Repeat("x", 200), id)
	require.Error(t, err)
}

func reset(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `TRUNCATE outbox_events`)
	require.NoError(t, err)
}

func pending(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`).Scan(&n))
	return n
}
