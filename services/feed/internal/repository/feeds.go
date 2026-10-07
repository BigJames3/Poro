package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/poro/feed/internal/cursor"
	"github.com/poro/feed/internal/mix"
)

// Item is a visible video with its counters and its author's profile.
type Item struct {
	Video
	Likes         int64
	Comments      int64
	Shares        int64
	TrendingScore float64
	Username      *string
	DisplayName   *string
	AvatarURL     *string
}

// visible keeps videos a feed may show.
const visible = `v.deleted_at IS NULL AND v.moderation_status = 'approved'`

const itemColumns = `v.video_id, v.author_id, v.title, v.hashtags, v.thumbnail_key, v.hls_key, v.duration_ms, v.published_at,
	COALESCE(s.likes_count, 0), COALESCE(s.comments_count, 0), COALESCE(s.shares_count, 0), COALESCE(s.trending_score, 0),
	a.username, a.display_name, a.avatar_url`

const itemJoins = `FROM videos v
	LEFT JOIN video_stats s ON s.video_id = v.video_id
	LEFT JOIN authors a ON a.author_id = v.author_id`

func collectItems(rows pgx.Rows) ([]Item, error) {
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Item, error) {
		var it Item
		err := r.Scan(&it.VideoID, &it.AuthorID, &it.Title, &it.Hashtags, &it.ThumbnailKey, &it.HLSKey,
			&it.DurationMs, &it.PublishedAt, &it.Likes, &it.Comments, &it.Shares, &it.TrendingScore,
			&it.Username, &it.DisplayName, &it.AvatarURL)
		return it, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan items: %w", err)
	}
	return out, nil
}

// FollowingPage reads the videos of the accounts viewer follows, newest
// first. Fan-out on read: one join of follows and videos per request.
func FollowingPage(ctx context.Context, db DBTX, viewer uuid.UUID, after *cursor.Time, limit int) ([]Item, error) {
	var at *time.Time
	var id *uuid.UUID
	if after != nil {
		at, id = &after.At, &after.VideoID
	}
	rows, err := db.Query(ctx, `SELECT `+itemColumns+` `+itemJoins+`
		JOIN follows f ON f.following_id = v.author_id AND f.follower_id = $1
		WHERE `+visible+`
		  AND ($2::timestamptz IS NULL OR (v.published_at, v.video_id) < ($2::timestamptz, $3::uuid))
		ORDER BY v.published_at DESC, v.video_id DESC
		LIMIT $4`, viewer, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("following page: %w", err)
	}
	return collectItems(rows)
}

// TrendingPage reads videos of the last 72 hours by trending score.
func TrendingPage(ctx context.Context, db DBTX, after *cursor.Score, limit int) ([]Item, error) {
	var score *float64
	var id *uuid.UUID
	if after != nil {
		score, id = &after.Score, &after.VideoID
	}
	rows, err := db.Query(ctx, `SELECT `+itemColumns+` `+itemJoins+`
		WHERE `+visible+` AND s.trending_score > 0
		  AND ($1::double precision IS NULL OR (s.trending_score, v.video_id) < ($1::double precision, $2::uuid))
		ORDER BY s.trending_score DESC, v.video_id DESC
		LIMIT $3`, score, id, limit)
	if err != nil {
		return nil, fmt.Errorf("trending page: %w", err)
	}
	return collectItems(rows)
}

// Hydrate reads the visible videos among ids, in the order of ids.
func Hydrate(ctx context.Context, db DBTX, ids []uuid.UUID) ([]Item, error) {
	if len(ids) == 0 {
		return []Item{}, nil
	}
	rows, err := db.Query(ctx, `SELECT `+itemColumns+` `+itemJoins+`
		WHERE v.video_id = ANY($1) AND `+visible, ids)
	if err != nil {
		return nil, fmt.Errorf("hydrate: %w", err)
	}
	found, err := collectItems(rows)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]Item, len(found))
	for _, it := range found {
		byID[it.VideoID] = it
	}
	out := make([]Item, 0, len(found))
	for _, id := range ids {
		if it, ok := byID[id]; ok {
			out = append(out, it)
		}
	}
	return out, nil
}

func collectCandidates(rows pgx.Rows) ([]mix.Candidate, error) {
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (mix.Candidate, error) {
		var c mix.Candidate
		err := r.Scan(&c.VideoID, &c.AuthorID)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan candidates: %w", err)
	}
	return out, nil
}

// FollowingCandidates returns the newest videos of followed accounts.
func FollowingCandidates(ctx context.Context, db DBTX, viewer uuid.UUID, limit int) ([]mix.Candidate, error) {
	rows, err := db.Query(ctx, `SELECT v.video_id, v.author_id FROM videos v
		JOIN follows f ON f.following_id = v.author_id AND f.follower_id = $1
		WHERE `+visible+`
		ORDER BY v.published_at DESC, v.video_id DESC LIMIT $2`, viewer, limit)
	if err != nil {
		return nil, fmt.Errorf("following candidates: %w", err)
	}
	return collectCandidates(rows)
}

// TrendingCandidates returns the highest scores of the window.
func TrendingCandidates(ctx context.Context, db DBTX, limit int) ([]mix.Candidate, error) {
	rows, err := db.Query(ctx, `SELECT v.video_id, v.author_id FROM videos v
		JOIN video_stats s ON s.video_id = v.video_id
		WHERE `+visible+` AND s.trending_score > 0
		ORDER BY s.trending_score DESC, v.video_id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("trending candidates: %w", err)
	}
	return collectCandidates(rows)
}

// DiscoveryCandidates returns recent videos of small authors the viewer does
// not follow yet, newest first.
func DiscoveryCandidates(ctx context.Context, db DBTX, viewer uuid.UUID, maxAge time.Duration, maxFollowers int64, limit int) ([]mix.Candidate, error) {
	rows, err := db.Query(ctx, `SELECT v.video_id, v.author_id FROM videos v
		LEFT JOIN author_followers af ON af.author_id = v.author_id
		WHERE `+visible+`
		  AND v.published_at > now() - make_interval(secs => $2)
		  AND COALESCE(af.followers_count, 0) < $3
		  AND v.author_id <> $1
		  AND NOT EXISTS (SELECT 1 FROM follows f WHERE f.follower_id = $1 AND f.following_id = v.author_id)
		ORDER BY v.published_at DESC, v.video_id DESC LIMIT $4`, viewer, maxAge.Seconds(), maxFollowers, limit)
	if err != nil {
		return nil, fmt.Errorf("discovery candidates: %w", err)
	}
	return collectCandidates(rows)
}
