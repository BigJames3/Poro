package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
)

// janitorBatch bounds the rows one pass locks at a time.
const janitorBatch = 100

// UploadJanitor closes the uploads that were started and never completed
// (PORO-008): their multipart upload is aborted so the storage they hold is
// freed, and the row becomes abandoned. Every replica may run it: rows are
// claimed with SKIP LOCKED.
type UploadJanitor struct {
	repo     *repository.Videos
	store    storage.Storage
	log      *zap.Logger
	after    time.Duration
	interval time.Duration
	now      func() time.Time
}

// NewUploadJanitor returns a janitor that abandons uploads older than after,
// every interval.
func NewUploadJanitor(repo *repository.Videos, store storage.Storage, after, interval time.Duration, log *zap.Logger) *UploadJanitor {
	return &UploadJanitor{repo: repo, store: store, log: log, after: after, interval: interval, now: time.Now}
}

// Run passes once at start, then every interval, until ctx is cancelled.
func (j *UploadJanitor) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		if _, _, err := j.RunOnce(ctx); err != nil && ctx.Err() == nil {
			j.log.Error("upload janitor pass failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunOnce abandons every stale upload known to the database, then aborts the
// stale multipart uploads no row knows about. It returns both counts.
func (j *UploadJanitor) RunOnce(ctx context.Context) (abandoned, orphans int, err error) {
	before := j.now().Add(-j.after)
	for {
		rows, err := j.repo.AbandonStaleUploads(ctx, before, janitorBatch, func(v model.Video) error {
			if v.S3UploadID == nil {
				return nil
			}
			if err := j.store.AbortMultipart(ctx, v.SourceKey, *v.S3UploadID); err != nil {
				j.log.Warn("abort of a stale upload failed, kept for the next pass",
					zap.String("video_id", v.ID), zap.Error(err))
				return err
			}
			return nil
		})
		if err != nil {
			return abandoned, orphans, fmt.Errorf("abandon stale uploads: %w", err)
		}
		for _, v := range rows {
			j.log.Info("upload abandoned", zap.String("video_id", v.ID), zap.String("user_id", v.UserID),
				zap.Time("started_at", v.CreatedAt))
		}
		abandoned += len(rows)
		if len(rows) < janitorBatch {
			break
		}
	}
	// A crash between creating the multipart upload and saving its row leaves
	// an upload no row points to; the multipart sweep frees it too.
	orphans, err = j.store.AbortStaleMultiparts(ctx, before)
	if err != nil {
		return abandoned, orphans, fmt.Errorf("abort stale multipart uploads: %w", err)
	}
	if abandoned > 0 || orphans > 0 {
		j.log.Info("upload janitor pass", zap.Int("abandoned", abandoned), zap.Int("orphan_multiparts", orphans))
	}
	return abandoned, orphans, nil
}
