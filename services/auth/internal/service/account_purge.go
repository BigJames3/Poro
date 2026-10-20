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

// expiredCodeRetention keeps expired email codes one day for investigation:
// they hold addresses, so they do not stay longer.
const expiredCodeRetention = 24 * time.Hour

// ExpiredCodes deletes codes that expired before a date.
type ExpiredCodes interface {
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

// ExpiredSweep deletes rows that expired and no longer serve any purpose,
// such as OTP codes or refresh tokens (PORO-019).
type ExpiredSweep struct {
	Name   string
	Delete func(ctx context.Context) (int64, error)
}

// AccountPurgeJob erases the accounts whose deletion date has passed, on
// every replica: SKIP LOCKED keeps them from erasing the same account.
type AccountPurgeJob struct {
	purger   repository.AccountPurger
	codes    ExpiredCodes
	sweeps   []ExpiredSweep
	interval time.Duration
	log      *zap.Logger
	now      func() time.Time
}

// NewAccountPurgeJob returns a job that runs every interval. It also deletes
// the email codes expired for more than a day, and runs every sweep.
func NewAccountPurgeJob(purger repository.AccountPurger, codes ExpiredCodes, interval time.Duration, log *zap.Logger, sweeps ...ExpiredSweep) (*AccountPurgeJob, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("account purge interval must be positive, got %s", interval)
	}
	return &AccountPurgeJob{
		purger:   purger,
		codes:    codes,
		sweeps:   sweeps,
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
		if deleted, err := j.codes.DeleteExpired(ctx, j.now().Add(-expiredCodeRetention)); err != nil {
			j.log.Error("expired email code cleanup failed", zap.Error(err))
		} else if deleted > 0 {
			j.log.Info("expired email codes deleted", zap.Int64("count", deleted))
		}
		j.sweep(ctx)
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

// sweep runs every expired-row cleanup; one failing does not stop the others.
func (j *AccountPurgeJob) sweep(ctx context.Context) {
	for _, s := range j.sweeps {
		deleted, err := s.Delete(ctx)
		switch {
		case err != nil:
			j.log.Error("expired rows cleanup failed", zap.String("table", s.Name), zap.Error(err))
		case deleted > 0:
			j.log.Info("expired rows deleted", zap.String("table", s.Name), zap.Int64("count", deleted))
		}
	}
}
