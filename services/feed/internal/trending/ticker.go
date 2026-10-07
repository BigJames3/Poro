package trending

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Recompute refreshes every score that ages and returns how many changed.
type Recompute func(ctx context.Context) (int64, error)

// Run recomputes scores once immediately, then every interval, until ctx is
// cancelled. Each event already rescores its own video; the ticker only
// applies the decay that time alone causes. A failed run is logged and
// retried at the next tick.
func Run(ctx context.Context, interval time.Duration, recompute Recompute, log *zap.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if n, err := recompute(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Warn("trending recompute failed", zap.Error(err))
		} else {
			log.Debug("trending recomputed", zap.Int64("rows", n))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
