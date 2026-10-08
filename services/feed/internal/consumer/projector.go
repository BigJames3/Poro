// Package consumer keeps the feed projections up to date from video, social
// and user events.
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

	"github.com/poro/feed/internal/repository"
)

// Group is the Kafka consumer group of the feed projections.
const Group = "poro-feed-projector"

// Topics are the events the feed consumes.
var Topics = []string{
	events.TypeVideoReady, events.TypeVideoDeleted,
	events.TypeSocialFollowCreated, events.TypeSocialFollowDeleted,
	events.TypeSocialLikeCreated, events.TypeSocialLikeDeleted,
	events.TypeSocialCommentCreated, events.TypeSocialCommentDeleted,
	events.TypeSocialShareCreated,
	events.TypeUserProfileUpdated,
	events.TypeModerationContentRemoved, events.TypeModerationContentRestored,
}

// NewProjector applies each event once, in the transaction of its inbox claim.
func NewProjector(pool *pgxpool.Pool, log *zap.Logger) kafka.Handler {
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
		return decodeVideoReady(env)
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
			return repository.DeleteVideo(ctx, tx, ids[0], ids[1], d.DeletedAt)
		}, nil
	case events.TypeSocialFollowCreated, events.TypeSocialFollowDeleted:
		return decodeFollow(env)
	case events.TypeSocialLikeCreated, events.TypeSocialLikeDeleted,
		events.TypeSocialCommentCreated, events.TypeSocialCommentDeleted, events.TypeSocialShareCreated:
		return decodeStat(env)
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
		return func(ctx context.Context, tx pgx.Tx) error {
			return repository.UpsertAuthor(ctx, tx, ids[0], d.Username, d.DisplayName, d.AvatarURL, d.UpdatedAt)
		}, nil
	case events.TypeModerationContentRemoved, events.TypeModerationContentRestored:
		return decodeModeration(env)
	default:
		return nil, fmt.Errorf("unsupported event %s", env.Type)
	}
}

// decodeModeration hides removed videos and shows restored ones. Removed
// comments reach the feed as social.comment.deleted, so they are skipped here.
func decodeModeration(env events.Envelope) (applyFunc, error) {
	skip := func(context.Context, pgx.Tx) error { return nil }
	if env.Type == events.TypeModerationContentRemoved {
		var d events.ModerationContentRemovedV1
		if err := env.DecodeData(&d); err != nil {
			return nil, err
		}
		ids, err := parseIDs(d.TargetID, d.OwnerID)
		if err != nil {
			return nil, err
		}
		switch d.TargetType {
		case events.ModerationTargetVideo:
			return func(ctx context.Context, tx pgx.Tx) error {
				return repository.RejectVideo(ctx, tx, ids[0], ids[1], d.RemovedAt)
			}, nil
		case events.ModerationTargetComment:
			return skip, nil
		default:
			return nil, fmt.Errorf("unsupported target_type %q", d.TargetType)
		}
	}
	var d events.ModerationContentRestoredV1
	if err := env.DecodeData(&d); err != nil {
		return nil, err
	}
	ids, err := parseIDs(d.TargetID, d.OwnerID)
	if err != nil {
		return nil, err
	}
	if d.TargetType != events.ModerationTargetVideo {
		return nil, fmt.Errorf("unsupported target_type %q", d.TargetType)
	}
	return func(ctx context.Context, tx pgx.Tx) error {
		return repository.ApproveVideo(ctx, tx, ids[0])
	}, nil
}

func decodeVideoReady(env events.Envelope) (applyFunc, error) {
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
		VideoID: ids[0], AuthorID: ids[1], Title: d.Title, Hashtags: d.Hashtags,
		ThumbnailKey: d.ThumbnailKey, HLSKey: d.HLSKey, DurationMs: d.DurationMs, PublishedAt: published,
	}
	return func(ctx context.Context, tx pgx.Tx) error {
		if err := repository.UpsertReadyVideo(ctx, tx, v); err != nil {
			return err
		}
		// Likes may have arrived before the video: score them now.
		return repository.Rescore(ctx, tx, v.VideoID)
	}, nil
}

func decodeFollow(env events.Envelope) (applyFunc, error) {
	if env.Type == events.TypeSocialFollowCreated {
		var d events.SocialFollowCreatedV1
		if err := env.DecodeData(&d); err != nil {
			return nil, err
		}
		ids, err := parseIDs(d.FollowerID, d.FollowingID)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, tx pgx.Tx) error {
			return repository.AddFollow(ctx, tx, ids[0], ids[1], d.CreatedAt)
		}, nil
	}
	var d events.SocialFollowDeletedV1
	if err := env.DecodeData(&d); err != nil {
		return nil, err
	}
	ids, err := parseIDs(d.FollowerID, d.FollowingID)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, tx pgx.Tx) error {
		return repository.RemoveFollow(ctx, tx, ids[0], ids[1])
	}, nil
}

// decodeStat turns a social event into counter deltas on its video.
func decodeStat(env events.Envelope) (applyFunc, error) {
	var payload struct {
		VideoID string `json:"video_id"`
	}
	if err := env.DecodeData(&payload); err != nil {
		return nil, err
	}
	ids, err := parseIDs(payload.VideoID)
	if err != nil {
		return nil, err
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
		return repository.AddStats(ctx, tx, ids[0], likes, comments, shares)
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
