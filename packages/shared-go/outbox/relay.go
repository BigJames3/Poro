package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// relayLockID is the advisory lock that elects one active relay per database.
// A single publisher keeps the per-key order that consumers rely on.
const relayLockID int64 = 0x706f726f6f7574 // "poroout"

const maxErrorLen = 1000

// RejectedPrefix marks, in last_error, a message the broker refused for good
// (too large, invalid). Such a message is parked: the relay skips it, and the
// later messages of the same topic and key, so that the per-key order is kept
// while every other key keeps flowing. Clearing last_error requeues them.
const RejectedPrefix = "rejected: "

// BatchError is returned by a Publisher that knows which messages of a batch
// failed. Messages absent from Failed were published. Any other error fails
// the whole batch as a transient error.
type BatchError struct {
	Failed map[uuid.UUID]error
	// Rejected marks the failed messages the broker refused for good:
	// retrying them cannot succeed.
	Rejected map[uuid.UUID]bool
}

func (e *BatchError) Error() string {
	return fmt.Sprintf("%d of the batch not published (%d rejected)", len(e.Failed), len(e.Rejected))
}

// Message is one record to publish.
type Message struct {
	ID    uuid.UUID
	Topic string
	Key   []byte
	Value []byte
}

// Publisher sends a batch and returns only once every message is acknowledged.
type Publisher interface {
	Publish(ctx context.Context, msgs []Message) error
}

// RelayConfig tunes the relay. Zero values select the defaults.
type RelayConfig struct {
	BatchSize       int           // 100
	PollInterval    time.Duration // 500ms, when the outbox is empty
	ErrorBackoff    time.Duration // 2s, after a failed batch
	Retention       time.Duration // 72h, published rows kept for investigation
	CleanupInterval time.Duration // 10m
}

// Relay moves outbox rows to Kafka.
type Relay struct {
	pool      *pgxpool.Pool
	publisher Publisher
	log       *zap.Logger
	cfg       RelayConfig

	published prometheus.Counter
	failures  prometheus.Counter
	pending   prometheus.Gauge
	parked    prometheus.Gauge
	rejected  prometheus.Counter
}

// NewRelay builds a relay. reg may be nil.
func NewRelay(pool *pgxpool.Pool, publisher Publisher, log *zap.Logger, cfg RelayConfig, reg prometheus.Registerer) (*Relay, error) {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.ErrorBackoff <= 0 {
		cfg.ErrorBackoff = 2 * time.Second
	}
	if cfg.Retention <= 0 {
		cfg.Retention = 72 * time.Hour
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 10 * time.Minute
	}
	r := &Relay{
		pool:      pool,
		publisher: publisher,
		log:       log,
		cfg:       cfg,
		published: prometheus.NewCounter(prometheus.CounterOpts{Name: "outbox_published_total", Help: "Outbox events published."}),
		failures:  prometheus.NewCounter(prometheus.CounterOpts{Name: "outbox_publish_failures_total", Help: "Failed outbox batches."}),
		pending:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "outbox_pending", Help: "Unpublished outbox events at the last poll."}),
		parked:    prometheus.NewGauge(prometheus.GaugeOpts{Name: "outbox_parked", Help: "Outbox events refused by the broker and set aside."}),
		rejected:  prometheus.NewCounter(prometheus.CounterOpts{Name: "outbox_rejected_total", Help: "Outbox events the broker refused for good."}),
	}
	if reg != nil {
		for _, c := range []prometheus.Collector{r.published, r.failures, r.pending, r.parked, r.rejected} {
			if err := reg.Register(c); err != nil {
				return nil, fmt.Errorf("register relay metrics: %w", err)
			}
		}
	}
	return r, nil
}

// Run publishes until ctx is cancelled. It never returns on transient errors.
func (r *Relay) Run(ctx context.Context) error {
	nextCleanup := time.Now().Add(r.cfg.CleanupInterval)
	for {
		n, err := r.PublishBatch(ctx)
		wait := time.Duration(0)
		switch {
		case ctx.Err() != nil:
			return nil
		case err != nil:
			r.failures.Inc()
			r.log.Error("outbox batch failed", zap.Error(err))
			wait = r.cfg.ErrorBackoff
		case n < r.cfg.BatchSize:
			wait = r.cfg.PollInterval
		}
		if time.Now().After(nextCleanup) {
			if deleted, err := r.Cleanup(ctx); err != nil {
				r.log.Warn("outbox cleanup failed", zap.Error(err))
			} else if deleted > 0 {
				r.log.Info("outbox cleanup", zap.Int64("deleted", deleted))
			}
			nextCleanup = time.Now().Add(r.cfg.CleanupInterval)
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(wait):
			}
		}
	}
}

// PublishBatch publishes the oldest pending rows in order. It returns 0 without
// error when another relay holds the lock.
func (r *Relay) PublishBatch(ctx context.Context) (int, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, relayLockID).Scan(&locked); err != nil {
		return 0, fmt.Errorf("advisory lock: %w", err)
	}
	if !locked {
		return 0, nil
	}

	var parked int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM outbox_events
		WHERE published_at IS NULL AND starts_with(last_error, $1)`, RejectedPrefix).Scan(&parked); err != nil {
		return 0, fmt.Errorf("count parked: %w", err)
	}
	r.parked.Set(float64(parked))

	// Parked messages are skipped, and so are the later messages of their
	// topic and key: Kafka orders by key, and consumers rely on that order.
	rows, err := tx.Query(ctx,
		`SELECT o.id, o.topic, o.event_key, o.payload::text FROM outbox_events o
		 WHERE o.published_at IS NULL
		   AND NOT EXISTS (SELECT 1 FROM outbox_events p
		     WHERE p.published_at IS NULL AND starts_with(p.last_error, $2)
		       AND p.topic = o.topic AND p.event_key = o.event_key)
		 ORDER BY o.created_at, o.id LIMIT $1`, r.cfg.BatchSize, RejectedPrefix)
	if err != nil {
		return 0, fmt.Errorf("select pending: %w", err)
	}
	var msgs []Message
	var ids []uuid.UUID
	for rows.Next() {
		var m Message
		var key, payload string
		if err := rows.Scan(&m.ID, &m.Topic, &key, &payload); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan pending: %w", err)
		}
		m.Key, m.Value = []byte(key), []byte(payload)
		msgs = append(msgs, m)
		ids = append(ids, m.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read pending: %w", err)
	}
	r.pending.Set(float64(len(msgs)))
	if len(msgs) == 0 {
		return 0, nil
	}

	if pubErr := r.publisher.Publish(ctx, msgs); pubErr != nil {
		var batch *BatchError
		if errors.As(pubErr, &batch) {
			return r.recordPartial(ctx, tx, msgs, batch)
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox_events SET attempts = attempts + 1, last_error = $2 WHERE id = ANY($1)`, ids, truncate(pubErr.Error())); err != nil {
			return 0, errors.Join(fmt.Errorf("publish: %w", pubErr), fmt.Errorf("record failure: %w", err))
		}
		if err := tx.Commit(ctx); err != nil {
			return 0, errors.Join(fmt.Errorf("publish: %w", pubErr), fmt.Errorf("commit failure: %w", err))
		}
		return 0, fmt.Errorf("publish: %w", pubErr)
	}

	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET published_at = now(), last_error = NULL WHERE id = ANY($1)`, ids); err != nil {
		return 0, fmt.Errorf("mark published: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	r.published.Add(float64(len(msgs)))
	return len(msgs), nil
}

// recordPartial marks the published messages of a batch, parks the rejected
// ones and counts an attempt on the others. It returns an error, for the
// relay to back off, only when a message failed for a transient reason.
func (r *Relay) recordPartial(ctx context.Context, tx pgx.Tx, msgs []Message, batch *BatchError) (int, error) {
	var published []uuid.UUID
	transient := 0
	for _, m := range msgs {
		cause, failed := batch.Failed[m.ID]
		switch {
		case !failed:
			published = append(published, m.ID)
		case batch.Rejected[m.ID]:
			if _, err := tx.Exec(ctx, `UPDATE outbox_events SET attempts = attempts + 1, last_error = $2 WHERE id = $1`,
				m.ID, truncate(RejectedPrefix+cause.Error())); err != nil {
				return 0, fmt.Errorf("park rejected message: %w", err)
			}
			r.rejected.Inc()
			r.log.Error("outbox message rejected by the broker, parked",
				zap.String("id", m.ID.String()), zap.String("topic", m.Topic), zap.Error(cause))
		default:
			transient++
			if _, err := tx.Exec(ctx, `UPDATE outbox_events SET attempts = attempts + 1, last_error = $2 WHERE id = $1`,
				m.ID, truncate(cause.Error())); err != nil {
				return 0, fmt.Errorf("record failure: %w", err)
			}
		}
	}
	if len(published) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE outbox_events SET published_at = now(), last_error = NULL WHERE id = ANY($1)`, published); err != nil {
			return 0, fmt.Errorf("mark published: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	r.published.Add(float64(len(published)))
	if transient > 0 {
		return len(published), fmt.Errorf("publish: %w", batch)
	}
	return len(published), nil
}

func truncate(msg string) string {
	if len(msg) > maxErrorLen {
		return msg[:maxErrorLen]
	}
	return msg
}

// Cleanup deletes published rows older than the retention.
func (r *Relay) Cleanup(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM outbox_events WHERE published_at IS NOT NULL AND published_at < now() - make_interval(secs => $1)`,
		r.cfg.Retention.Seconds())
	if err != nil {
		return 0, fmt.Errorf("cleanup outbox: %w", err)
	}
	return tag.RowsAffected(), nil
}
