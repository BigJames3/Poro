// Package erasure forgets the videos of an erased account (ADR-0018): every
// video is soft-deleted with its text emptied, poro.video.deleted is published
// once per video so the moderation consumer quarantines its media, open
// uploads are aborted and source objects are deleted.
package erasure

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/service"
	"github.com/poro/video/internal/storage"
)

// Group is the Kafka consumer group that erases accounts.
const Group = "poro-video-account-erasure"

// Topics are the events the group consumes.
var Topics = []string{events.TypeAuthUserDeleted}

// Handler erases one account. As in the moderation consumer, the database is
// committed first, then the objects are deleted, and the inbox is claimed
// last: a storage failure fails the message and its redelivery redoes the
// deletions. The database step is idempotent on its own (only videos still
// live are deleted, so poro.video.deleted is never published twice) and the
// storage step is derived from the rows, so a replay or a second event for
// the same account changes nothing more.
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
	if env.Type != events.TypeAuthUserDeleted || env.Version != 1 {
		return kafka.Permanent(fmt.Errorf("unsupported event %s v%d", env.Type, env.Version))
	}
	var d events.AuthUserDeletedV1
	if err := env.DecodeData(&d); err != nil {
		return kafka.Permanent(err)
	}
	userID, err := uuid.Parse(d.UserID)
	if err != nil || userID == uuid.Nil {
		return kafka.Permanent(fmt.Errorf("invalid user id %q", d.UserID))
	}
	if err := h.erase(ctx, userID); err != nil {
		return err
	}
	if err := h.purge(ctx, userID); err != nil {
		return err
	}
	return h.claim(ctx, env.ID)
}

// erase soft-deletes the account's videos and publishes their deletion.
func (h *Handler) erase(ctx context.Context, userID uuid.UUID) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	deleted, err := h.repo.EraseUser(ctx, tx, userID)
	if err != nil {
		return err
	}
	for _, row := range deleted {
		env, err := service.DeletedEvent(row.ID, row.UserID, row.DeletedAt.UTC())
		if err != nil {
			return err
		}
		if err := outbox.Enqueue(ctx, tx, env); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	h.log.Info("account videos erased", zap.String("user_id", userID.String()), zap.Int("deleted", len(deleted)))
	return nil
}

// purge aborts the open uploads and deletes the source objects of every
// video of the account. Media are quarantined by the moderation consumer on
// poro.video.deleted.
func (h *Handler) purge(ctx context.Context, userID uuid.UUID) error {
	rows, err := h.repo.ListAllByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == model.StatusUploading && row.S3UploadID != nil {
			if err := h.store.AbortMultipart(ctx, row.SourceKey, *row.S3UploadID); err != nil {
				return err
			}
		}
		if err := h.store.DeleteSource(ctx, row.SourceKey); err != nil {
			return err
		}
	}
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
