package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Video is the projection of a ready video.
type Video struct {
	VideoID      uuid.UUID
	AuthorID     uuid.UUID
	Title        string
	Hashtags     []string
	ThumbnailKey string
	HLSKey       string
	DurationMs   int
	PublishedAt  time.Time
}

// UpsertReadyVideo records a ready video. A deleted video (or its tombstone)
// stays deleted.
func UpsertReadyVideo(ctx context.Context, db DBTX, v Video) error {
	hashtags := v.Hashtags
	if hashtags == nil {
		hashtags = []string{}
	}
	_, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id, title, hashtags, thumbnail_key, hls_key, duration_ms, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (video_id) DO UPDATE SET
			author_id = EXCLUDED.author_id, title = EXCLUDED.title, hashtags = EXCLUDED.hashtags,
			thumbnail_key = EXCLUDED.thumbnail_key, hls_key = EXCLUDED.hls_key,
			duration_ms = EXCLUDED.duration_ms, published_at = EXCLUDED.published_at
		WHERE videos.deleted_at IS NULL`,
		v.VideoID, v.AuthorID, v.Title, hashtags, v.ThumbnailKey, v.HLSKey, v.DurationMs, v.PublishedAt)
	if err != nil {
		return fmt.Errorf("upsert video: %w", err)
	}
	return nil
}

// DeleteVideo marks a video deleted, or stores a tombstone for an unknown one.
func DeleteVideo(ctx context.Context, db DBTX, videoID, authorID uuid.UUID, at time.Time) error {
	_, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id, published_at, deleted_at) VALUES ($1, $2, $3, $3)
		ON CONFLICT (video_id) DO UPDATE SET deleted_at = COALESCE(videos.deleted_at, EXCLUDED.deleted_at)`,
		videoID, authorID, at)
	if err != nil {
		return fmt.Errorf("delete video: %w", err)
	}
	return nil
}

// AddFollow records a follow and bumps the author's follower count once.
func AddFollow(ctx context.Context, db DBTX, followerID, followingID uuid.UUID, at time.Time) error {
	tag, err := db.Exec(ctx, `INSERT INTO follows (follower_id, following_id, created_at) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, followerID, followingID, at)
	if err != nil {
		return fmt.Errorf("insert follow: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	_, err = db.Exec(ctx, `INSERT INTO author_followers (author_id, followers_count) VALUES ($1, 1)
		ON CONFLICT (author_id) DO UPDATE SET followers_count = author_followers.followers_count + 1`, followingID)
	if err != nil {
		return fmt.Errorf("count follower: %w", err)
	}
	return nil
}

// RemoveFollow deletes a follow and lowers the author's follower count once.
func RemoveFollow(ctx context.Context, db DBTX, followerID, followingID uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM follows WHERE follower_id = $1 AND following_id = $2`, followerID, followingID)
	if err != nil {
		return fmt.Errorf("delete follow: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	_, err = db.Exec(ctx, `UPDATE author_followers SET followers_count = GREATEST(followers_count - 1, 0) WHERE author_id = $1`, followingID)
	if err != nil {
		return fmt.Errorf("uncount follower: %w", err)
	}
	return nil
}

// AddStats applies counter deltas, clamped at zero, then rescores the video.
func AddStats(ctx context.Context, db DBTX, videoID uuid.UUID, likes, comments, shares int64) error {
	_, err := db.Exec(ctx, `INSERT INTO video_stats (video_id, likes_count, comments_count, shares_count)
		VALUES ($1, GREATEST($2::bigint, 0), GREATEST($3::bigint, 0), GREATEST($4::bigint, 0))
		ON CONFLICT (video_id) DO UPDATE SET
			likes_count = GREATEST(video_stats.likes_count + $2, 0),
			comments_count = GREATEST(video_stats.comments_count + $3, 0),
			shares_count = GREATEST(video_stats.shares_count + $4, 0),
			updated_at = now()`,
		videoID, likes, comments, shares)
	if err != nil {
		return fmt.Errorf("update stats: %w", err)
	}
	return Rescore(ctx, db, videoID)
}

// UpsertAuthor keeps the most recent profile snapshot of an author.
func UpsertAuthor(ctx context.Context, db DBTX, authorID uuid.UUID, username, displayName, avatarURL *string, at time.Time) error {
	_, err := db.Exec(ctx, `INSERT INTO authors (author_id, username, display_name, avatar_url, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (author_id) DO UPDATE SET
			username = EXCLUDED.username, display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url, updated_at = EXCLUDED.updated_at
		WHERE authors.updated_at < EXCLUDED.updated_at`,
		authorID, username, displayName, avatarURL, at)
	if err != nil {
		return fmt.Errorf("upsert author: %w", err)
	}
	return nil
}
