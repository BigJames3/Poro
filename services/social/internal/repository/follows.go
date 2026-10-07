package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/poro/social/internal/cursor"
)

// Follow is one row of follows.
type Follow struct {
	ID          uuid.UUID
	FollowerID  uuid.UUID
	FollowingID uuid.UUID
	CreatedAt   time.Time
}

// InsertFollow adds the follow and reports whether it is new.
func InsertFollow(ctx context.Context, db DBTX, f Follow) (bool, error) {
	tag, err := db.Exec(ctx, `INSERT INTO follows (id, follower_id, following_id, created_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (follower_id, following_id) DO NOTHING`, f.ID, f.FollowerID, f.FollowingID, f.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("insert follow: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// DeleteFollow removes the follow and reports whether one existed.
func DeleteFollow(ctx context.Context, db DBTX, followerID, followingID uuid.UUID) (bool, error) {
	tag, err := db.Exec(ctx, `DELETE FROM follows WHERE follower_id = $1 AND following_id = $2`, followerID, followingID)
	if err != nil {
		return false, fmt.Errorf("delete follow: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// IsFollowing reports whether follower follows following.
func IsFollowing(ctx context.Context, db DBTX, followerID, followingID uuid.UUID) (bool, error) {
	var ok bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM follows WHERE follower_id = $1 AND following_id = $2)`,
		followerID, followingID).Scan(&ok); err != nil {
		return false, fmt.Errorf("is following: %w", err)
	}
	return ok, nil
}

// ListFollowers returns who follows userID, newest first.
func ListFollowers(ctx context.Context, db DBTX, userID uuid.UUID, after *cursor.Position, limit int) ([]Follow, error) {
	ts, id := keyset(after)
	rows, err := db.Query(ctx, `SELECT id, follower_id, following_id, created_at FROM follows
		WHERE following_id = $1 AND ($2::timestamptz IS NULL OR (created_at, id) < ($2::timestamptz, $3::uuid))
		ORDER BY created_at DESC, id DESC LIMIT $4`, userID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("list followers: %w", err)
	}
	return collectFollows(rows)
}

// ListFollowing returns who userID follows, newest first.
func ListFollowing(ctx context.Context, db DBTX, userID uuid.UUID, after *cursor.Position, limit int) ([]Follow, error) {
	ts, id := keyset(after)
	rows, err := db.Query(ctx, `SELECT id, follower_id, following_id, created_at FROM follows
		WHERE follower_id = $1 AND ($2::timestamptz IS NULL OR (created_at, id) < ($2::timestamptz, $3::uuid))
		ORDER BY created_at DESC, id DESC LIMIT $4`, userID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("list following: %w", err)
	}
	return collectFollows(rows)
}

func collectFollows(rows pgx.Rows) ([]Follow, error) {
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Follow, error) {
		var f Follow
		err := r.Scan(&f.ID, &f.FollowerID, &f.FollowingID, &f.CreatedAt)
		return f, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan follows: %w", err)
	}
	return out, nil
}
