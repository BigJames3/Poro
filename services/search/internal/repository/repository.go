// Package repository writes the search read model from events. Every function
// runs in the caller's transaction, next to the inbox claim.
package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is satisfied by pgx.Tx and *pgxpool.Pool.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Video is what poro.video.ready says about a video.
type Video struct {
	VideoID      uuid.UUID
	AuthorID     uuid.UUID
	Title        string
	Description  string
	Hashtags     []string
	ThumbnailKey string
	DurationMs   int
	PublishedAt  time.Time
}

const visible = `published_at IS NOT NULL AND deleted_at IS NULL AND moderation_status = 'approved'`

// visibleTags locks the video row and returns its hashtags when it is
// searchable, nil otherwise.
func visibleTags(ctx context.Context, db DBTX, videoID uuid.UUID) ([]string, error) {
	var tags []string
	var shown bool
	err := db.QueryRow(ctx, `SELECT hashtags, `+visible+` FROM videos WHERE video_id = $1 FOR UPDATE`, videoID).
		Scan(&tags, &shown)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !shown) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("visible tags: %w", err)
	}
	return tags, nil
}

// changeVideo applies change and keeps hashtags.videos_count equal to the
// number of searchable videos carrying each tag.
func changeVideo(ctx context.Context, db DBTX, videoID uuid.UUID, at time.Time, change func() error) error {
	before, err := visibleTags(ctx, db, videoID)
	if err != nil {
		return err
	}
	if err := change(); err != nil {
		return err
	}
	after, err := visibleTags(ctx, db, videoID)
	if err != nil {
		return err
	}
	for _, tag := range distinct(after) {
		if slices.Contains(before, tag) {
			continue
		}
		if _, err := db.Exec(ctx, `INSERT INTO hashtags (tag, videos_count, last_used_at) VALUES ($1, 1, $2)
			ON CONFLICT (tag) DO UPDATE SET videos_count = hashtags.videos_count + 1,
				last_used_at = GREATEST(hashtags.last_used_at, EXCLUDED.last_used_at)`, tag, at); err != nil {
			return fmt.Errorf("count hashtag: %w", err)
		}
	}
	for _, tag := range distinct(before) {
		if slices.Contains(after, tag) {
			continue
		}
		if _, err := db.Exec(ctx, `UPDATE hashtags SET videos_count = GREATEST(videos_count - 1, 0) WHERE tag = $1`, tag); err != nil {
			return fmt.Errorf("uncount hashtag: %w", err)
		}
	}
	return nil
}

func distinct(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if !slices.Contains(out, tag) {
			out = append(out, tag)
		}
	}
	return out
}

// UpsertReadyVideo records a ready video. A deleted or rejected video keeps
// that state whatever order the events arrive in.
func UpsertReadyVideo(ctx context.Context, db DBTX, v Video) error {
	tags := v.Hashtags
	if tags == nil {
		tags = []string{}
	}
	return changeVideo(ctx, db, v.VideoID, v.PublishedAt, func() error {
		_, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id, title, description, hashtags, thumbnail_key, duration_ms, published_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (video_id) DO UPDATE SET author_id = EXCLUDED.author_id, title = EXCLUDED.title,
				description = EXCLUDED.description, hashtags = EXCLUDED.hashtags, thumbnail_key = EXCLUDED.thumbnail_key,
				duration_ms = EXCLUDED.duration_ms, published_at = EXCLUDED.published_at`,
			v.VideoID, v.AuthorID, v.Title, v.Description, tags, v.ThumbnailKey, v.DurationMs, v.PublishedAt)
		if err != nil {
			return fmt.Errorf("upsert video: %w", err)
		}
		return nil
	})
}

// DeleteVideo hides a video for good, or leaves a tombstone for an unknown one.
func DeleteVideo(ctx context.Context, db DBTX, videoID, authorID uuid.UUID, at time.Time) error {
	return changeVideo(ctx, db, videoID, at, func() error {
		_, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id, deleted_at) VALUES ($1, $2, $3)
			ON CONFLICT (video_id) DO UPDATE SET deleted_at = COALESCE(videos.deleted_at, EXCLUDED.deleted_at)`,
			videoID, authorID, at)
		if err != nil {
			return fmt.Errorf("delete video: %w", err)
		}
		return nil
	})
}

// SetModeration hides (rejected) or shows again (approved) a video. An
// unknown video gets a stub, so a late poro.video.ready keeps it hidden.
func SetModeration(ctx context.Context, db DBTX, videoID, authorID uuid.UUID, status string, at time.Time) error {
	return changeVideo(ctx, db, videoID, at, func() error {
		_, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id, moderation_status) VALUES ($1, $2, $3)
			ON CONFLICT (video_id) DO UPDATE SET moderation_status = EXCLUDED.moderation_status`,
			videoID, authorID, status)
		if err != nil {
			return fmt.Errorf("set moderation: %w", err)
		}
		return nil
	})
}

// AddVideoStats moves the counters used for ranking, never below zero. A
// video not projected yet gets a stub when its owner is known.
func AddVideoStats(ctx context.Context, db DBTX, videoID uuid.UUID, ownerID *uuid.UUID, likes, comments, shares int64) error {
	if ownerID != nil {
		if _, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id) VALUES ($1, $2) ON CONFLICT (video_id) DO NOTHING`,
			videoID, *ownerID); err != nil {
			return fmt.Errorf("stub video: %w", err)
		}
	}
	_, err := db.Exec(ctx, `UPDATE videos SET likes_count = GREATEST(likes_count + $2, 0),
		comments_count = GREATEST(comments_count + $3, 0), shares_count = GREATEST(shares_count + $4, 0)
		WHERE video_id = $1`, videoID, likes, comments, shares)
	if err != nil {
		return fmt.Errorf("video stats: %w", err)
	}
	return nil
}

// EnsureUser records an account; it becomes searchable once it has a username.
func EnsureUser(ctx context.Context, db DBTX, userID uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO users (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	return nil
}

// Profile is a poro.user.profile.updated snapshot.
type Profile struct {
	UserID      uuid.UUID
	Username    *string
	DisplayName *string
	AvatarURL   *string
	IsCreator   bool
	UpdatedAt   time.Time
}

// UpsertProfile stores the snapshot unless a newer one is already stored.
func UpsertProfile(ctx context.Context, db DBTX, p Profile) error {
	_, err := db.Exec(ctx, `INSERT INTO users (user_id, username, display_name, avatar_url, is_creator, profile_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET username = EXCLUDED.username, display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url, is_creator = EXCLUDED.is_creator, profile_at = EXCLUDED.profile_at
		WHERE users.profile_at IS NULL OR users.profile_at < EXCLUDED.profile_at`,
		p.UserID, p.Username, p.DisplayName, p.AvatarURL, p.IsCreator, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert profile: %w", err)
	}
	return nil
}

// AddFollowers moves the follower count used to rank accounts, never below zero.
func AddFollowers(ctx context.Context, db DBTX, userID uuid.UUID, delta int64) error {
	_, err := db.Exec(ctx, `INSERT INTO users (user_id, followers_count) VALUES ($1, GREATEST($2::bigint, 0))
		ON CONFLICT (user_id) DO UPDATE SET followers_count = GREATEST(users.followers_count + $2, 0)`, userID, delta)
	if err != nil {
		return fmt.Errorf("followers: %w", err)
	}
	return nil
}
