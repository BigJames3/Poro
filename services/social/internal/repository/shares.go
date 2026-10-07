package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Share is one row of shares.
type Share struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	VideoID   uuid.UUID
	Channel   string
	CreatedAt time.Time
}

// InsertShare records a share. Shares are not deduplicated.
func InsertShare(ctx context.Context, db DBTX, s Share) error {
	if _, err := db.Exec(ctx, `INSERT INTO shares (id, user_id, video_id, channel, created_at) VALUES ($1, $2, $3, $4, $5)`,
		s.ID, s.UserID, s.VideoID, s.Channel, s.CreatedAt); err != nil {
		return fmt.Errorf("insert share: %w", err)
	}
	return nil
}
