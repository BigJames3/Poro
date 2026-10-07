package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// VideoCounters are the public totals of a video.
type VideoCounters struct {
	Likes    int64
	Comments int64
	Shares   int64
}

// UserCounters are the public totals of an account.
type UserCounters struct {
	Followers int64
	Following int64
}

// AddVideoCounters applies deltas in the caller's transaction. Increments
// upsert the row; decrements update it, since Postgres checks the
// non-negative constraint on an INSERT candidate before resolving the
// conflict. A decrement always follows an increment, so the row exists.
func AddVideoCounters(ctx context.Context, db DBTX, videoID uuid.UUID, likes, comments, shares int64) error {
	var err error
	if likes >= 0 && comments >= 0 && shares >= 0 {
		_, err = db.Exec(ctx, `INSERT INTO video_counters (video_id, likes_count, comments_count, shares_count, updated_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (video_id) DO UPDATE SET
				likes_count = video_counters.likes_count + EXCLUDED.likes_count,
				comments_count = video_counters.comments_count + EXCLUDED.comments_count,
				shares_count = video_counters.shares_count + EXCLUDED.shares_count,
				updated_at = now()`,
			videoID, likes, comments, shares)
	} else {
		_, err = db.Exec(ctx, `UPDATE video_counters SET
				likes_count = likes_count + $2, comments_count = comments_count + $3,
				shares_count = shares_count + $4, updated_at = now()
			WHERE video_id = $1`,
			videoID, likes, comments, shares)
	}
	if err != nil {
		return fmt.Errorf("update video counters: %w", err)
	}
	return nil
}

// GetVideoCounters returns zero totals for a video nobody interacted with.
func GetVideoCounters(ctx context.Context, db DBTX, videoID uuid.UUID) (VideoCounters, error) {
	var c VideoCounters
	err := db.QueryRow(ctx, `SELECT likes_count, comments_count, shares_count FROM video_counters WHERE video_id = $1`,
		videoID).Scan(&c.Likes, &c.Comments, &c.Shares)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return c, fmt.Errorf("get video counters: %w", err)
	}
	return c, nil
}

// AddFollowCounters updates both accounts of a follow, one row at a time in
// user id order so two opposite follows cannot deadlock.
func AddFollowCounters(ctx context.Context, db DBTX, followerID, followingID uuid.UUID, delta int64) error {
	type change struct {
		id                   uuid.UUID
		followers, following int64
	}
	changes := []change{{id: followingID, followers: delta}, {id: followerID, following: delta}}
	if changes[1].id.String() < changes[0].id.String() {
		changes[0], changes[1] = changes[1], changes[0]
	}
	for _, c := range changes {
		var err error
		if delta >= 0 {
			_, err = db.Exec(ctx, `INSERT INTO user_counters (user_id, followers_count, following_count, updated_at)
				VALUES ($1, $2, $3, now())
				ON CONFLICT (user_id) DO UPDATE SET
					followers_count = user_counters.followers_count + EXCLUDED.followers_count,
					following_count = user_counters.following_count + EXCLUDED.following_count,
					updated_at = now()`,
				c.id, c.followers, c.following)
		} else {
			_, err = db.Exec(ctx, `UPDATE user_counters SET
					followers_count = followers_count + $2, following_count = following_count + $3, updated_at = now()
				WHERE user_id = $1`,
				c.id, c.followers, c.following)
		}
		if err != nil {
			return fmt.Errorf("update user counters: %w", err)
		}
	}
	return nil
}

// GetUserCounters returns zero totals for an account with no follow.
func GetUserCounters(ctx context.Context, db DBTX, userID uuid.UUID) (UserCounters, error) {
	var c UserCounters
	err := db.QueryRow(ctx, `SELECT followers_count, following_count FROM user_counters WHERE user_id = $1`,
		userID).Scan(&c.Followers, &c.Following)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return c, fmt.Errorf("get user counters: %w", err)
	}
	return c, nil
}
