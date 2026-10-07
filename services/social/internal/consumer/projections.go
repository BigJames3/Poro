// Package consumer keeps the social projections of other services up to date.
package consumer

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/social/internal/repository"
)

// Group is the Kafka consumer group of the social projections.
const Group = "poro-social-projections"

// Topics are the events the projections consume.
var Topics = []string{events.TypeVideoReady, events.TypeVideoDeleted, events.TypeAuthUserCreated}

// NewProjections applies each event once, in the transaction of its inbox claim.
func NewProjections(pool *pgxpool.Pool, log *zap.Logger) kafka.Handler {
	return func(ctx context.Context, env events.Envelope) error {
		if env.Version != 1 {
			return kafka.Permanent(fmt.Errorf("unsupported event %s v%d", env.Type, env.Version))
		}
		apply, err := decode(env)
		if err != nil {
			return kafka.Permanent(err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		first, err := inbox.Claim(ctx, tx, Group, env.ID)
		if err != nil {
			return err
		}
		if first {
			if err := apply(ctx, tx); err != nil {
				return err
			}
			log.Debug("projection applied", zap.String("type", env.Type), zap.String("subject", env.Subject))
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		return nil
	}
}

type applyFunc func(context.Context, pgx.Tx) error

// decode validates the payload before any database work, so bad data goes
// straight to the DLQ.
func decode(env events.Envelope) (applyFunc, error) {
	switch env.Type {
	case events.TypeVideoReady:
		var d events.VideoReadyV1
		if err := env.DecodeData(&d); err != nil {
			return nil, err
		}
		video, owner, err := ids(d.VideoID, d.UserID)
		if err != nil {
			return nil, err
		}
		at := d.PublishedAt
		if at.IsZero() {
			at = d.ReadyAt
		}
		return func(ctx context.Context, tx pgx.Tx) error {
			if err := repository.EnsureUser(ctx, tx, owner); err != nil {
				return err
			}
			return repository.MarkVideoReady(ctx, tx, video, owner, at)
		}, nil
	case events.TypeVideoDeleted:
		var d events.VideoDeletedV1
		if err := env.DecodeData(&d); err != nil {
			return nil, err
		}
		video, owner, err := ids(d.VideoID, d.UserID)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, tx pgx.Tx) error {
			return repository.MarkVideoDeleted(ctx, tx, video, owner, d.DeletedAt)
		}, nil
	case events.TypeAuthUserCreated:
		var d events.AuthUserCreatedV1
		if err := env.DecodeData(&d); err != nil {
			return nil, err
		}
		user, err := uuid.Parse(d.UserID)
		if err != nil {
			return nil, fmt.Errorf("user_id: %w", err)
		}
		return func(ctx context.Context, tx pgx.Tx) error {
			return repository.EnsureUser(ctx, tx, user)
		}, nil
	default:
		return nil, fmt.Errorf("unsupported event %s", env.Type)
	}
}

func ids(videoID, userID string) (uuid.UUID, uuid.UUID, error) {
	video, err := uuid.Parse(videoID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("video_id: %w", err)
	}
	user, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("user_id: %w", err)
	}
	if video == uuid.Nil || user == uuid.Nil {
		return uuid.Nil, uuid.Nil, errors.New("ids must not be nil")
	}
	return video, user, nil
}
