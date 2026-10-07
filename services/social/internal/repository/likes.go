package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/poro/social/internal/cursor"
	"github.com/poro/social/internal/model"
)

// Like is one row of likes.
type Like struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	VideoID   uuid.UUID
	CreatedAt time.Time
}

// InsertLike adds the like and reports whether it is new.
func InsertLike(ctx context.Context, db DBTX, l Like) (bool, error) {
	tag, err := db.Exec(ctx, `INSERT INTO likes (id, user_id, video_id, created_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, video_id) DO NOTHING`, l.ID, l.UserID, l.VideoID, l.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("insert like: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// DeleteLike removes the like and reports whether one existed.
func DeleteLike(ctx context.Context, db DBTX, userID, videoID uuid.UUID) (bool, error) {
	tag, err := db.Exec(ctx, `DELETE FROM likes WHERE user_id = $1 AND video_id = $2`, userID, videoID)
	if err != nil {
		return false, fmt.Errorf("delete like: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// HasLiked reports whether the user likes the video.
func HasLiked(ctx context.Context, db DBTX, userID, videoID uuid.UUID) (bool, error) {
	var ok bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM likes WHERE user_id = $1 AND video_id = $2)`,
		userID, videoID).Scan(&ok); err != nil {
		return false, fmt.Errorf("has liked: %w", err)
	}
	return ok, nil
}

// ListVideoLikes returns the likes of a video, newest first.
func ListVideoLikes(ctx context.Context, db DBTX, videoID uuid.UUID, after *cursor.Position, limit int) ([]Like, error) {
	ts, id := keyset(after)
	rows, err := db.Query(ctx, `SELECT id, user_id, video_id, created_at FROM likes
		WHERE video_id = $1 AND ($2::timestamptz IS NULL OR (created_at, id) < ($2::timestamptz, $3::uuid))
		ORDER BY created_at DESC, id DESC LIMIT $4`, videoID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("list video likes: %w", err)
	}
	return collectLikes(rows)
}

// ListUserLikes returns the ready videos a user likes, newest like first.
func ListUserLikes(ctx context.Context, db DBTX, userID uuid.UUID, after *cursor.Position, limit int) ([]Like, error) {
	ts, id := keyset(after)
	rows, err := db.Query(ctx, `SELECT l.id, l.user_id, l.video_id, l.created_at FROM likes l
		JOIN videos_projection v ON v.video_id = l.video_id AND v.status = $5
		WHERE l.user_id = $1 AND ($2::timestamptz IS NULL OR (l.created_at, l.id) < ($2::timestamptz, $3::uuid))
		ORDER BY l.created_at DESC, l.id DESC LIMIT $4`, userID, ts, id, limit, model.VideoReady)
	if err != nil {
		return nil, fmt.Errorf("list user likes: %w", err)
	}
	return collectLikes(rows)
}

func collectLikes(rows pgx.Rows) ([]Like, error) {
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Like, error) {
		var l Like
		err := r.Scan(&l.ID, &l.UserID, &l.VideoID, &l.CreatedAt)
		return l, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan likes: %w", err)
	}
	return out, nil
}
