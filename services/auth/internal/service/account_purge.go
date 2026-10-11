package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/poro/auth/internal/repository"
)

// purgeBatchSize bounds one purge transaction.
const purgeBatchSize = 100

// AccountPurgeJob erases the accounts whose deletion date has passed, on
// every replica: SKIP LOCKED keeps them from erasing the same account.
type AccountPurgeJob struct {
	purger   repository.AccountPurger
	interval time.Duration
	log      *zap.Logger
	now      func() time.Time
}

// NewAccountPurgeJob returns a job that runs every interval.
func NewAccountPurgeJob(purger repository.AccountPurger, interval time.Duration, log *zap.Logger) (*AccountPurgeJob, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("account purge interval must be positive, got %s", interval)
	}
	return &AccountPurgeJob{
		purger:   purger,
		interval: interval,
		log:      log,
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

// Run purges once at start, then every interval, until ctx ends. A failed
// pass is logged and retried at the next tick.
func (j *AccountPurgeJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		if erased, err := j.RunOnce(ctx); err != nil {
			j.log.Error("account purge failed", zap.Error(err))
		} else if erased > 0 {
			j.log.Info("accounts erased", zap.Int("count", erased))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunOnce erases every account due now, batch after batch.
func (j *AccountPurgeJob) RunOnce(ctx context.Context) (int, error) {
	total := 0
	for {
		ids, err := j.purger.PurgeDue(ctx, j.now(), purgeBatchSize)
		if err != nil {
			return total, err
		}
		for _, id := range ids {
			j.log.Info("account erased", zap.String("user_id", id.String()))
		}
		total += len(ids)
		if len(ids) < purgeBatchSize {
			return total, nil
		}
	}
}
