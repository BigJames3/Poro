package inbox

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Retention is how long a processed event is remembered. Kafka keeps events
// 7 days and dead-lettered events are by definition not claimed: an older
// row can no longer stop a duplicate (PORO-019).
const Retention = 30 * 24 * time.Hour

// purgeBatch bounds one delete statement.
const purgeBatch = 5000

// Purge deletes the rows processed before `before`, in batches, and returns
// how many it deleted.
func Purge(ctx context.Context, db Execer, before time.Time) (int64, error) {
	var total int64
	for {
		tag, err := db.Exec(ctx, `DELETE FROM processed_events WHERE ctid IN (
			SELECT ctid FROM processed_events WHERE processed_at < $1 LIMIT $2)`, before, purgeBatch)
		if err != nil {
			return total, fmt.Errorf("purge processed events: %w", err)
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < purgeBatch {
			return total, nil
		}
	}
}

// RunPurge purges rows older than Retention every interval, starting one
// interval after the call, until ctx ends. Every replica may run it.
func RunPurge(ctx context.Context, db Execer, interval time.Duration, log *zap.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		deleted, err := Purge(ctx, db, time.Now().Add(-Retention))
		switch {
		case err != nil && ctx.Err() == nil:
			log.Error("processed events purge failed", zap.Error(err))
		case deleted > 0:
			log.Info("processed events purged", zap.Int64("count", deleted))
		}
	}
}
