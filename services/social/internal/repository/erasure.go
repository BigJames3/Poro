package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UserLike is a like of an account with the owner of the liked video, nil
// when the video was never projected.
type UserLike struct {
	VideoID uuid.UUID
	OwnerID *uuid.UUID
}

// VideoShares is how many shares of a video were removed.
type VideoShares struct {
	VideoID uuid.UUID
	Count   int64
}

// TombstoneUser records an erased account, so EnsureUser never brings it back.
func TombstoneUser(ctx context.Context, db DBTX, userID uuid.UUID, at time.Time) error {
	if _, err := db.Exec(ctx, `INSERT INTO erased_users (user_id, erased_at) VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING`, userID, at); err != nil {
		return fmt.Errorf("tombstone user: %w", err)
	}
	return nil
}

// DeleteUser removes the projection and the counters of an account.
func DeleteUser(ctx context.Context, db DBTX, userID uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM user_counters WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete user counters: %w", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users_projection WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// ListAllUserLikes returns every like of an account whatever the status of
// the video, in video id order.
func ListAllUserLikes(ctx context.Context, db DBTX, userID uuid.UUID) ([]UserLike, error) {
	rows, err := db.Query(ctx, `SELECT l.video_id, v.owner_id FROM likes l
		LEFT JOIN videos_projection v ON v.video_id = l.video_id
		WHERE l.user_id = $1 ORDER BY l.video_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user likes: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (UserLike, error) {
		var l UserLike
		err := r.Scan(&l.VideoID, &l.OwnerID)
		return l, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan user likes: %w", err)
	}
	return out, nil
}

// ListUserCommentLikes returns the comments an account likes, in id order.
func ListUserCommentLikes(ctx context.Context, db DBTX, userID uuid.UUID) ([]uuid.UUID, error) {
	return collectIDs(db.Query(ctx, `SELECT comment_id FROM comment_likes WHERE user_id = $1 ORDER BY comment_id`, userID))
}

// ListUserCommentIDs returns the comments of an account not deleted yet, in id order.
func ListUserCommentIDs(ctx context.Context, db DBTX, userID uuid.UUID) ([]uuid.UUID, error) {
	return collectIDs(db.Query(ctx, `SELECT id FROM comments WHERE user_id = $1 AND deleted_at IS NULL ORDER BY id`, userID))
}

// ClearUserComments empties the text of every deleted comment of an account.
func ClearUserComments(ctx context.Context, db DBTX, userID uuid.UUID) error {
	if _, err := db.Exec(ctx, `UPDATE comments SET content = '' WHERE user_id = $1 AND deleted_at IS NOT NULL AND content <> ''`,
		userID); err != nil {
		return fmt.Errorf("clear user comments: %w", err)
	}
	return nil
}

// ListAllUserFollows returns the follows of an account in both directions,
// in order of the other account's id.
func ListAllUserFollows(ctx context.Context, db DBTX, userID uuid.UUID) ([]Follow, error) {
	rows, err := db.Query(ctx, `SELECT id, follower_id, following_id, created_at FROM follows
		WHERE follower_id = $1 OR following_id = $1
		ORDER BY CASE WHEN follower_id = $1 THEN following_id ELSE follower_id END`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user follows: %w", err)
	}
	return collectFollows(rows)
}

// DeleteUserShares removes every share of an account and returns how many
// were removed per video, in video id order.
func DeleteUserShares(ctx context.Context, db DBTX, userID uuid.UUID) ([]VideoShares, error) {
	rows, err := db.Query(ctx, `WITH removed AS (DELETE FROM shares WHERE user_id = $1 RETURNING video_id)
		SELECT video_id, count(*) FROM removed GROUP BY video_id ORDER BY video_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("delete user shares: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (VideoShares, error) {
		var s VideoShares
		err := r.Scan(&s.VideoID, &s.Count)
		return s, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan user shares: %w", err)
	}
	return out, nil
}

func collectIDs(rows pgx.Rows, err error) ([]uuid.UUID, error) {
	if err != nil {
		return nil, fmt.Errorf("list ids: %w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("scan ids: %w", err)
	}
	return out, nil
}
