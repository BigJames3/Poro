package outbox_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/internal/testinfra"
	"github.com/poro/shared-go/outbox"
)

// selectivePublisher refuses some subjects for good and fails others once.
type selectivePublisher struct {
	mu        sync.Mutex
	rejected  map[string]bool
	transient map[string]int
	subjects  map[uuid.UUID]string
	sent      []string
}

func (p *selectivePublisher) Publish(_ context.Context, msgs []outbox.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	batch := &outbox.BatchError{Failed: map[uuid.UUID]error{}, Rejected: map[uuid.UUID]bool{}}
	for _, m := range msgs {
		subject := string(m.Key)
		switch {
		case p.rejected[subject]:
			batch.Failed[m.ID] = errors.New("MESSAGE_TOO_LARGE")
			batch.Rejected[m.ID] = true
		case p.transient[subject] > 0:
			p.transient[subject]--
			batch.Failed[m.ID] = errors.New("NOT_LEADER_FOR_PARTITION")
		default:
			p.sent = append(p.sent, p.subjects[m.ID])
		}
	}
	if len(batch.Failed) == 0 {
		return nil
	}
	return batch
}

func TestRelayParksRejectedMessages(t *testing.T) {
	pool := testinfra.Postgres(t)
	ctx := context.Background()
	reset(t, pool)

	pub := &selectivePublisher{rejected: map[string]bool{"a": true}, transient: map[string]int{}, subjects: map[uuid.UUID]string{}}
	enqueue := func(key, name string) {
		env := newEvent(t, key)
		pub.subjects[env.ID] = name
		require.NoError(t, outbox.Enqueue(ctx, pool, env))
	}
	reg := prometheus.NewRegistry()
	relay, err := outbox.NewRelay(pool, pub, zap.NewNop(), outbox.RelayConfig{}, reg)
	require.NoError(t, err)

	enqueue("a", "a1")
	enqueue("b", "b1")
	n, err := relay.PublishBatch(ctx)
	require.NoError(t, err, "a rejection alone does not make the relay back off")
	require.Equal(t, 1, n)
	require.Equal(t, []string{"b1"}, pub.sent)
	var lastError string
	require.NoError(t, pool.QueryRow(ctx, `SELECT last_error FROM outbox_events WHERE published_at IS NULL`).Scan(&lastError))
	require.Contains(t, lastError, outbox.RejectedPrefix+"MESSAGE_TOO_LARGE")

	enqueue("a", "a2")
	enqueue("c", "c1")
	n, err = relay.PublishBatch(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, []string{"b1", "c1"}, pub.sent, "other keys flow; a2 waits behind the parked a1")
	require.Equal(t, 1.0, metric(t, reg, "outbox_parked"))
	require.Equal(t, 1.0, metric(t, reg, "outbox_rejected_total"))

	// An operator fixes the cause and requeues: order is kept.
	pub.rejected = map[string]bool{}
	_, err = pool.Exec(ctx, `UPDATE outbox_events SET last_error = NULL WHERE starts_with(last_error, $1)`, outbox.RejectedPrefix)
	require.NoError(t, err)
	n, err = relay.PublishBatch(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.Equal(t, []string{"b1", "c1", "a1", "a2"}, pub.sent)
	require.Equal(t, 0, pending(t, pool))
}

func TestRelayRetriesTransientFailuresWithoutParking(t *testing.T) {
	pool := testinfra.Postgres(t)
	ctx := context.Background()
	reset(t, pool)

	pub := &selectivePublisher{rejected: map[string]bool{}, transient: map[string]int{"b": 1}, subjects: map[uuid.UUID]string{}}
	for _, k := range []string{"a", "b"} {
		env := newEvent(t, k)
		pub.subjects[env.ID] = k
		require.NoError(t, outbox.Enqueue(ctx, pool, env))
	}
	relay, err := outbox.NewRelay(pool, pub, zap.NewNop(), outbox.RelayConfig{}, nil)
	require.NoError(t, err)

	n, err := relay.PublishBatch(ctx)
	require.Error(t, err, "a transient failure makes the relay back off")
	require.Equal(t, 1, n, "the message that went through is marked published")
	var attempts int
	var lastError string
	require.NoError(t, pool.QueryRow(ctx, `SELECT attempts, last_error FROM outbox_events WHERE published_at IS NULL`).Scan(&attempts, &lastError))
	require.Equal(t, 1, attempts)
	require.NotContains(t, lastError, outbox.RejectedPrefix, "never parked for a transient error")

	n, err = relay.PublishBatch(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, []string{"a", "b"}, pub.sent)
}

// metric reads the value of a gauge or counter without labels.
func metric(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		m := f.GetMetric()[0]
		if m.GetGauge() != nil {
			return m.GetGauge().GetValue()
		}
		return m.GetCounter().GetValue()
	}
	t.Fatalf("metric %s not registered", name)
	return 0
}
