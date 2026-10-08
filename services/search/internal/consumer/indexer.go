// Package consumer keeps the search read model up to date from events.
package consumer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/search/internal/model"
	"github.com/poro/search/internal/repository"
)

// Group is the Kafka consumer group of the search indexer.
const Group = "poro-search-indexer"

// Topics are the events the indexer consumes.
var Topics = []string{
	events.TypeVideoReady, events.TypeVideoDeleted,
	events.TypeModerationContentRemoved, events.TypeModerationContentRestored,
	events.TypeAuthUserCreated, events.TypeUserProfileUpdated,
	events.TypeSocialLikeCreated, events.TypeSocialLikeDeleted,
	events.TypeSocialCommentCreated, events.TypeSocialCommentDeleted,
	events.TypeSocialShareCreated,
	events.TypeSocialFollowCreated, events.TypeSocialFollowDeleted,
}

type applyFunc func(context.Context, pgx.Tx) error

// NewIndexer applies each event once, in the transaction of its inbox claim.
func NewIndexer(pool *pgxpool.Pool, log *zap.Logger) kafka.Handler {
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
			log.Debug("indexed", zap.String("type", env.Type), zap.String("subject", env.Subject))
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		return nil
	}
}

// decode validates the payload before any database work, so bad data goes
// straight to the DLQ.
func decode(env events.Envelope) (applyFunc, error) {
	switch env.Type {
	case events.TypeVideoReady:
		return decodeReady(env)
	case events.TypeVideoDeleted:
		var d events.VideoDeletedV1
		if err := env.DecodeData(&d); err != nil {
			return nil, err
		}
		ids, err := parseIDs(d.VideoID, d.UserID)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, tx pgx.Tx) error {
			return repository.DeleteVideo(ctx, tx, ids[0], ids[1], orNow(d.DeletedAt))
		}, nil
	case events.TypeModerationContentRemoved, events.TypeModerationContentRestored:
		return decodeModeration(env)
	case events.TypeAuthUserCreated:
		var d events.AuthUserCreatedV1
		if err := env.DecodeData(&d); err != nil {
			return nil, err
		}
		ids, err := parseIDs(d.UserID)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, tx pgx.Tx) error { return repository.EnsureUser(ctx, tx, ids[0]) }, nil
	case events.TypeUserProfileUpdated:
		var d events.UserProfileUpdatedV1
		if err := env.DecodeData(&d); err != nil {
			return nil, err
		}
		ids, err := parseIDs(d.UserID)
		if err != nil {
			return nil, err
		}
		if d.UpdatedAt.IsZero() {
			return nil, errors.New("updated_at is required")
		}
		p := repository.Profile{UserID: ids[0], Username: d.Username, DisplayName: d.DisplayName,
			AvatarURL: d.AvatarURL, IsCreator: d.IsCreator, UpdatedAt: d.UpdatedAt}
		return func(ctx context.Context, tx pgx.Tx) error { return repository.UpsertProfile(ctx, tx, p) }, nil
	case events.TypeSocialFollowCreated, events.TypeSocialFollowDeleted:
		var d struct {
			FollowingID string `json:"following_id"`
		}
		if err := env.DecodeData(&d); err != nil {
			return nil, err
		}
		ids, err := parseIDs(d.FollowingID)
		if err != nil {
			return nil, err
		}
		delta := int64(1)
		if env.Type == events.TypeSocialFollowDeleted {
			delta = -1
		}
		return func(ctx context.Context, tx pgx.Tx) error { return repository.AddFollowers(ctx, tx, ids[0], delta) }, nil
	case events.TypeSocialLikeCreated, events.TypeSocialLikeDeleted, events.TypeSocialCommentCreated,
		events.TypeSocialCommentDeleted, events.TypeSocialShareCreated:
		return decodeStat(env)
	default:
		return nil, fmt.Errorf("unsupported event %s", env.Type)
	}
}

func decodeReady(env events.Envelope) (applyFunc, error) {
	var d events.VideoReadyV1
	if err := env.DecodeData(&d); err != nil {
		return nil, err
	}
	ids, err := parseIDs(d.VideoID, d.UserID)
	if err != nil {
		return nil, err
	}
	published := d.PublishedAt
	if published.IsZero() {
		published = d.ReadyAt
	}
	if published.IsZero() {
		return nil, errors.New("published_at or ready_at is required")
	}
	v := repository.Video{
		VideoID: ids[0], AuthorID: ids[1], Title: d.Title, Description: d.Description, Hashtags: d.Hashtags,
		ThumbnailKey: d.ThumbnailKey, DurationMs: d.DurationMs, PublishedAt: published,
	}
	return func(ctx context.Context, tx pgx.Tx) error { return repository.UpsertReadyVideo(ctx, tx, v) }, nil
}

// decodeModeration hides removed videos and shows restored ones. Comments are
// not searchable, so their removals are skipped.
func decodeModeration(env events.Envelope) (applyFunc, error) {
	var d struct {
		TargetType string    `json:"target_type"`
		TargetID   string    `json:"target_id"`
		OwnerID    string    `json:"owner_id"`
		RemovedAt  time.Time `json:"removed_at"`
		RestoredAt time.Time `json:"restored_at"`
	}
	if err := env.DecodeData(&d); err != nil {
		return nil, err
	}
	if d.TargetType == events.ModerationTargetComment && env.Type == events.TypeModerationContentRemoved {
		return func(context.Context, pgx.Tx) error { return nil }, nil
	}
	if d.TargetType != events.ModerationTargetVideo {
		return nil, fmt.Errorf("unsupported target_type %q", d.TargetType)
	}
	ids, err := parseIDs(d.TargetID, d.OwnerID)
	if err != nil {
		return nil, err
	}
	status, at := model.ModerationRejected, d.RemovedAt
	if env.Type == events.TypeModerationContentRestored {
		status, at = model.ModerationApproved, d.RestoredAt
	}
	return func(ctx context.Context, tx pgx.Tx) error {
		return repository.SetModeration(ctx, tx, ids[0], ids[1], status, orNow(at))
	}, nil
}

// decodeStat turns a social event into counter deltas on its video.
func decodeStat(env events.Envelope) (applyFunc, error) {
	var d struct {
		VideoID      string `json:"video_id"`
		VideoOwnerID string `json:"video_owner_id"`
	}
	if err := env.DecodeData(&d); err != nil {
		return nil, err
	}
	ids, err := parseIDs(d.VideoID)
	if err != nil {
		return nil, err
	}
	var owner *uuid.UUID
	if d.VideoOwnerID != "" {
		parsed, err := parseIDs(d.VideoOwnerID)
		if err != nil {
			return nil, err
		}
		owner = &parsed[0]
	}
	var likes, comments, shares int64
	switch env.Type {
	case events.TypeSocialLikeCreated:
		likes = 1
	case events.TypeSocialLikeDeleted:
		likes = -1
	case events.TypeSocialCommentCreated:
		comments = 1
	case events.TypeSocialCommentDeleted:
		comments = -1
	case events.TypeSocialShareCreated:
		shares = 1
	}
	return func(ctx context.Context, tx pgx.Tx) error {
		return repository.AddVideoStats(ctx, tx, ids[0], owner, likes, comments, shares)
	}, nil
}

func parseIDs(raw ...string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, len(raw))
	for i, r := range raw {
		id, err := uuid.Parse(r)
		if err != nil {
			return nil, fmt.Errorf("id %q: %w", r, err)
		}
		if id == uuid.Nil {
			return nil, errors.New("ids must not be nil")
		}
		out[i] = id
	}
	return out, nil
}

func orNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}
