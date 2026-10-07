package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// scoreSQL is trending.Score in SQL over video_stats s and videos v:
// (likes + 2·comments + 3·shares) / (age_h + 2)^1.5, zero outside 72 hours
// and for deleted or hidden videos.
const scoreSQL = `CASE
	WHEN v.deleted_at IS NOT NULL OR v.moderation_status <> 'approved'
	  OR now() - v.published_at > interval '72 hours' THEN 0
	ELSE (s.likes_count + 2 * s.comments_count + 3 * s.shares_count)::double precision
	   / power(GREATEST(extract(epoch FROM now() - v.published_at) / 3600.0, 0) + 2, 1.5)
END`

// Rescore recomputes the trending score of one video. A video not projected
// yet keeps its counters and scores once poro.video.ready arrives.
func Rescore(ctx context.Context, db DBTX, videoID uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE video_stats s SET trending_score = `+scoreSQL+`
		FROM videos v WHERE v.video_id = s.video_id AND s.video_id = $1`, videoID)
	if err != nil {
		return fmt.Errorf("rescore: %w", err)
	}
	return nil
}

// RescoreAll refreshes the scores that age: videos inside the window (plus a
// margin for the ticker period) and any score still above zero. It returns
// how many rows changed.
func RescoreAll(ctx context.Context, db DBTX) (int64, error) {
	tag, err := db.Exec(ctx, `UPDATE video_stats s SET trending_score = `+scoreSQL+`
		FROM videos v
		WHERE v.video_id = s.video_id
		  AND (v.published_at > now() - interval '73 hours' OR s.trending_score > 0)`)
	if err != nil {
		return 0, fmt.Errorf("rescore all: %w", err)
	}
	return tag.RowsAffected(), nil
}
