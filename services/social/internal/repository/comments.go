package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/poro/social/internal/cursor"
)

// Comment is one row of comments. LikedByMe is filled by the list queries.
type Comment struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	VideoID      uuid.UUID
	ParentID     *uuid.UUID
	Content      string
	LikesCount   int
	RepliesCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
	LikedByMe    bool
}

const commentColumns = `c.id, c.user_id, c.video_id, c.parent_id, c.content, c.likes_count, c.replies_count,
	c.created_at, c.updated_at, c.deleted_at`

func scanComment(row pgx.Row, extra ...any) (*Comment, error) {
	var c Comment
	dest := append([]any{&c.ID, &c.UserID, &c.VideoID, &c.ParentID, &c.Content, &c.LikesCount, &c.RepliesCount,
		&c.CreatedAt, &c.UpdatedAt, &c.DeletedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

// InsertComment stores a new comment or reply.
func InsertComment(ctx context.Context, db DBTX, c Comment) error {
	_, err := db.Exec(ctx, `INSERT INTO comments (id, user_id, video_id, parent_id, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)`, c.ID, c.UserID, c.VideoID, c.ParentID, c.Content, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert comment: %w", err)
	}
	return nil
}

// LockComment reads a comment and locks it until the transaction ends.
func LockComment(ctx context.Context, tx DBTX, id uuid.UUID) (*Comment, error) {
	return scanComment(tx.QueryRow(ctx, `SELECT `+commentColumns+` FROM comments c WHERE c.id = $1 FOR UPDATE`, id))
}

// GetComment reads a comment, with whether viewer likes it (uuid.Nil for anonymous).
func GetComment(ctx context.Context, db DBTX, id, viewer uuid.UUID) (*Comment, error) {
	var liked bool
	c, err := scanComment(db.QueryRow(ctx, `SELECT `+commentColumns+`,
		EXISTS (SELECT 1 FROM comment_likes cl WHERE cl.comment_id = c.id AND cl.user_id = $2)
		FROM comments c WHERE c.id = $1`, id, viewer), &liked)
	if err != nil {
		return nil, err
	}
	c.LikedByMe = liked
	return c, nil
}

// UpdateCommentContent replaces the text of a comment.
func UpdateCommentContent(ctx context.Context, tx DBTX, id uuid.UUID, content string, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE comments SET content = $2, updated_at = $3 WHERE id = $1`, id, content, at); err != nil {
		return fmt.Errorf("update comment: %w", err)
	}
	return nil
}

// SoftDeleteComment marks a comment deleted.
func SoftDeleteComment(ctx context.Context, tx DBTX, id uuid.UUID, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE comments SET deleted_at = $2, updated_at = $2 WHERE id = $1 AND deleted_at IS NULL`, id, at); err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}

// AddReplies changes the visible reply count of a top-level comment.
func AddReplies(ctx context.Context, tx DBTX, parentID uuid.UUID, delta int) error {
	if _, err := tx.Exec(ctx, `UPDATE comments SET replies_count = replies_count + $2 WHERE id = $1`, parentID, delta); err != nil {
		return fmt.Errorf("update replies count: %w", err)
	}
	return nil
}

// AddCommentLikes changes the like count of a comment and returns the new value.
func AddCommentLikes(ctx context.Context, tx DBTX, id uuid.UUID, delta int) (int, error) {
	var n int
	if err := tx.QueryRow(ctx, `UPDATE comments SET likes_count = likes_count + $2 WHERE id = $1 RETURNING likes_count`,
		id, delta).Scan(&n); err != nil {
		return 0, fmt.Errorf("update comment likes: %w", err)
	}
	return n, nil
}

// InsertCommentLike adds a comment like and reports whether it is new.
func InsertCommentLike(ctx context.Context, tx DBTX, id, userID, commentID uuid.UUID, at time.Time) (bool, error) {
	tag, err := tx.Exec(ctx, `INSERT INTO comment_likes (id, user_id, comment_id, created_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, comment_id) DO NOTHING`, id, userID, commentID, at)
	if err != nil {
		return false, fmt.Errorf("insert comment like: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// DeleteCommentLike removes a comment like and reports whether one existed.
func DeleteCommentLike(ctx context.Context, tx DBTX, userID, commentID uuid.UUID) (bool, error) {
	tag, err := tx.Exec(ctx, `DELETE FROM comment_likes WHERE user_id = $1 AND comment_id = $2`, userID, commentID)
	if err != nil {
		return false, fmt.Errorf("delete comment like: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ListTopComments returns the top-level comments of a video, newest first. A
// deleted comment is kept while it still has visible replies.
func ListTopComments(ctx context.Context, db DBTX, videoID, viewer uuid.UUID, after *cursor.Position, limit int) ([]Comment, error) {
	ts, id := keyset(after)
	rows, err := db.Query(ctx, `SELECT `+commentColumns+`,
		EXISTS (SELECT 1 FROM comment_likes cl WHERE cl.comment_id = c.id AND cl.user_id = $2)
		FROM comments c
		WHERE c.video_id = $1 AND c.parent_id IS NULL AND (c.deleted_at IS NULL OR c.replies_count > 0)
		  AND ($3::timestamptz IS NULL OR (c.created_at, c.id) < ($3::timestamptz, $4::uuid))
		ORDER BY c.created_at DESC, c.id DESC LIMIT $5`, videoID, viewer, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	return collectComments(rows)
}

// ListReplies returns the visible replies of a comment, oldest first.
func ListReplies(ctx context.Context, db DBTX, parentID, viewer uuid.UUID, after *cursor.Position, limit int) ([]Comment, error) {
	ts, id := keyset(after)
	rows, err := db.Query(ctx, `SELECT `+commentColumns+`,
		EXISTS (SELECT 1 FROM comment_likes cl WHERE cl.comment_id = c.id AND cl.user_id = $2)
		FROM comments c
		WHERE c.parent_id = $1 AND c.deleted_at IS NULL
		  AND ($3::timestamptz IS NULL OR (c.created_at, c.id) > ($3::timestamptz, $4::uuid))
		ORDER BY c.created_at ASC, c.id ASC LIMIT $5`, parentID, viewer, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("list replies: %w", err)
	}
	return collectComments(rows)
}

func collectComments(rows pgx.Rows) ([]Comment, error) {
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Comment, error) {
		var liked bool
		c, err := scanComment(r, &liked)
		if err != nil {
			return Comment{}, err
		}
		c.LikedByMe = liked
		return *c, nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan comments: %w", err)
	}
	return out, nil
}
