// Package moderation applies moderation decisions and deletions to the media
// of a video: a removed or deleted video moves to the private quarantine
// bucket, a restored one comes back.
package moderation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
)

// Group is the Kafka consumer group of the video API.
const Group = "poro-video-moderation"

// Topics are the events the group consumes.
var Topics = []string{
	events.TypeModerationContentRemoved, events.TypeModerationContentRestored, events.TypeVideoDeleted,
}

// Handler applies one event. The status is committed before objects move and
// the inbox is claimed last, so a redelivery redoes an interrupted move and
// the media-worker sees the status before it publishes media (see
// worker.Handler.HideIfWithdrawn).
type Handler struct {
	pool  *pgxpool.Pool
	repo  *repository.Videos
	store storage.Storage
	log   *zap.Logger
}

// New returns a handler.
func New(pool *pgxpool.Pool, repo *repository.Videos, store storage.Storage, log *zap.Logger) *Handler {
	return &Handler{pool: pool, repo: repo, store: store, log: log}
}

// Handle is a kafka.Handler.
func (h *Handler) Handle(ctx context.Context, env events.Envelope) error {
	if env.Version != 1 {
		return kafka.Permanent(fmt.Errorf("unsupported event %s v%d", env.Type, env.Version))
	}
	if err := h.apply(ctx, env); err != nil {
		return err
	}
	return h.claim(ctx, env.ID)
}

func (h *Handler) apply(ctx context.Context, env events.Envelope) error {
	switch env.Type {
	case events.TypeModerationContentRemoved:
		var d events.ModerationContentRemovedV1
		if err := env.DecodeData(&d); err != nil {
			return kafka.Permanent(err)
		}
		if d.TargetType != events.ModerationTargetVideo {
			return nil
		}
		id, err := parseID(d.TargetID)
		if err != nil {
			return err
		}
		row, err := h.repo.SetModeration(ctx, id, model.ModerationRemoved, at(d.RemovedAt))
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return h.hide(ctx, row, "removed")
	case events.TypeModerationContentRestored:
		var d events.ModerationContentRestoredV1
		if err := env.DecodeData(&d); err != nil {
			return kafka.Permanent(err)
		}
		if d.TargetType != events.ModerationTargetVideo {
			return kafka.Permanent(fmt.Errorf("unsupported target_type %q", d.TargetType))
		}
		id, err := parseID(d.TargetID)
		if err != nil {
			return err
		}
		row, err := h.repo.SetModeration(ctx, id, model.ModerationApproved, at(d.RestoredAt))
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if row.DeletedAt != nil {
			return nil // deleted by its owner meanwhile: its media stay hidden
		}
		n, err := h.store.Reveal(ctx, model.MediaPrefix(row.UserID, row.ID))
		if err != nil {
			return fmt.Errorf("reveal media: %w", err)
		}
		h.log.Info("video media restored", zap.String("video_id", row.ID), zap.Int("objects", n))
		return nil
	case events.TypeVideoDeleted:
		var d events.VideoDeletedV1
		if err := env.DecodeData(&d); err != nil {
			return kafka.Permanent(err)
		}
		id, err := parseID(d.VideoID)
		if err != nil {
			return err
		}
		row, err := h.repo.GetAny(ctx, id)
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return h.hide(ctx, row, "deleted")
	default:
		return kafka.Permanent(fmt.Errorf("unsupported event %s", env.Type))
	}
}

func (h *Handler) hide(ctx context.Context, row *model.Video, why string) error {
	n, err := h.store.Hide(ctx, model.MediaPrefix(row.UserID, row.ID))
	if err != nil {
		return fmt.Errorf("hide media: %w", err)
	}
	h.log.Info("video media quarantined", zap.String("video_id", row.ID), zap.String("why", why), zap.Int("objects", n))
	return nil
}

func (h *Handler) claim(ctx context.Context, eventID uuid.UUID) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := inbox.Claim(ctx, tx, Group, eventID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, kafka.Permanent(fmt.Errorf("invalid video id %q", raw))
	}
	return id, nil
}

// at falls back to now for a zero decision time.
func at(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}
