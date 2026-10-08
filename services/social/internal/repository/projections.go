package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/poro/social/internal/model"
)

// ReadyVideoOwner returns the owner of a ready video, or ErrNotFound.
func ReadyVideoOwner(ctx context.Context, db DBTX, videoID uuid.UUID) (uuid.UUID, error) {
	var owner uuid.UUID
	err := db.QueryRow(ctx, `SELECT owner_id FROM videos_projection WHERE video_id = $1 AND status = $2`,
		videoID, model.VideoReady).Scan(&owner)
	if err != nil {
		return uuid.Nil, notFound(err)
	}
	return owner, nil
}

// MarkVideoReady records a ready video. A video already deleted or removed by
// moderation keeps that status, whatever order the events arrive in.
func MarkVideoReady(ctx context.Context, db DBTX, videoID, ownerID uuid.UUID, at time.Time) error {
	_, err := db.Exec(ctx, `INSERT INTO videos_projection (video_id, owner_id, status, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (video_id) DO UPDATE SET owner_id = EXCLUDED.owner_id, status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
		WHERE videos_projection.status NOT IN ($5, $6)`,
		videoID, ownerID, model.VideoReady, at, model.VideoDeleted, model.VideoRemoved)
	if err != nil {
		return fmt.Errorf("mark video ready: %w", err)
	}
	return nil
}

// MarkVideoDeleted records a deleted video. Likes, comments and counters are
// kept; the video simply stops accepting actions and reads.
func MarkVideoDeleted(ctx context.Context, db DBTX, videoID, ownerID uuid.UUID, at time.Time) error {
	_, err := db.Exec(ctx, `INSERT INTO videos_projection (video_id, owner_id, status, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (video_id) DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at`,
		videoID, ownerID, model.VideoDeleted, at)
	if err != nil {
		return fmt.Errorf("mark video deleted: %w", err)
	}
	return nil
}

// MarkVideoRemoved hides a video removed by moderation. A deleted video stays
// deleted; an unknown one is recorded removed, so a late video.ready cannot
// publish it.
func MarkVideoRemoved(ctx context.Context, db DBTX, videoID, ownerID uuid.UUID, at time.Time) error {
	_, err := db.Exec(ctx, `INSERT INTO videos_projection (video_id, owner_id, status, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (video_id) DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
		WHERE videos_projection.status <> $5`,
		videoID, ownerID, model.VideoRemoved, at, model.VideoDeleted)
	if err != nil {
		return fmt.Errorf("mark video removed: %w", err)
	}
	return nil
}

// MarkVideoRestored makes a removed video ready again. Other statuses are left
// as they are.
func MarkVideoRestored(ctx context.Context, db DBTX, videoID uuid.UUID, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE videos_projection SET status = $2, updated_at = $3
		WHERE video_id = $1 AND status = $4`,
		videoID, model.VideoReady, at, model.VideoRemoved)
	if err != nil {
		return fmt.Errorf("mark video restored: %w", err)
	}
	return nil
}

// EnsureUser records that an account exists.
func EnsureUser(ctx context.Context, db DBTX, userID uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO users_projection (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	return nil
}

// UserExists reports whether the account is known.
func UserExists(ctx context.Context, db DBTX, userID uuid.UUID) (bool, error) {
	var ok bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users_projection WHERE user_id = $1)`, userID).Scan(&ok); err != nil {
		return false, fmt.Errorf("user exists: %w", err)
	}
	return ok, nil
}
