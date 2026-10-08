package index

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres ranks with full-text search (search_fr: French stems without
// accents) and trigram similarity for typos, both defined in the migration.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres returns the Postgres index.
func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

var _ Index = (*Postgres)(nil)

// Videos: relevance (text rank plus the best word match in the title, which
// tolerates typos), lifted by engagement and decayed over months, so a fresh
// popular match beats a stale one.
func (p *Postgres) Videos(ctx context.Context, query string, offset, limit int) (Page[VideoHit], error) {
	rows, err := p.pool.Query(ctx, `
		WITH q AS (SELECT websearch_to_tsquery('search_fr', $1) AS tsq, search_fold($1) AS folded)
		SELECT v.video_id, v.title, v.hashtags, v.thumbnail_key, v.duration_ms, v.published_at,
			v.likes_count, v.comments_count, v.shares_count,
			v.author_id, u.username, u.display_name, u.avatar_url
		FROM videos v CROSS JOIN q LEFT JOIN users u ON u.user_id = v.author_id
		WHERE v.published_at IS NOT NULL AND v.deleted_at IS NULL AND v.moderation_status = 'approved'
			AND (v.document @@ q.tsq OR q.folded <% search_fold(v.title))
		ORDER BY (ts_rank_cd(v.document, q.tsq, 32) + word_similarity(q.folded, search_fold(v.title)))
			* (1 + ln(1 + v.likes_count + 2 * v.comments_count + 3 * v.shares_count) / 10)
			/ (1 + extract(epoch FROM now() - v.published_at) / 2592000) DESC,
			v.video_id DESC
		OFFSET $2 LIMIT $3`, query, offset, limit+1)
	if err != nil {
		return Page[VideoHit]{}, fmt.Errorf("search videos: %w", err)
	}
	hits, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (VideoHit, error) {
		var h VideoHit
		err := row.Scan(&h.VideoID, &h.Title, &h.Hashtags, &h.ThumbnailKey, &h.DurationMs, &h.PublishedAt,
			&h.Likes, &h.Comments, &h.Shares,
			&h.Author.UserID, &h.Author.Username, &h.Author.DisplayName, &h.Author.AvatarURL)
		return h, err
	})
	if err != nil {
		return Page[VideoHit]{}, fmt.Errorf("scan videos: %w", err)
	}
	return page(hits, limit), nil
}

// Users: an exact username first, then relevance lifted by followers, with
// a small boost for creators. Accounts without a username are not listed.
func (p *Postgres) Users(ctx context.Context, query string, offset, limit int) (Page[UserHit], error) {
	rows, err := p.pool.Query(ctx, `
		WITH q AS (
			SELECT websearch_to_tsquery('search_fr', $1) || websearch_to_tsquery('simple', search_fold($1)) AS tsq,
				search_fold($1) AS folded, lower(ltrim(btrim($1), '@')) AS handle)
		SELECT u.user_id, u.username, u.display_name, u.avatar_url, u.is_creator, u.followers_count
		FROM users u CROSS JOIN q
		WHERE u.username IS NOT NULL
			AND (u.document @@ q.tsq
				OR q.folded <% search_fold(coalesce(u.username, '') || ' ' || coalesce(u.display_name, ''))
				OR lower(u.username) LIKE `+likePrefix("q.handle")+`)
		ORDER BY (lower(u.username) = q.handle) DESC,
			(ts_rank_cd(u.document, q.tsq, 32)
				+ word_similarity(q.folded, search_fold(coalesce(u.username, '') || ' ' || coalesce(u.display_name, ''))))
			* (1 + ln(1 + u.followers_count) / 10) * (CASE WHEN u.is_creator THEN 1.2 ELSE 1 END) DESC,
			u.user_id DESC
		OFFSET $2 LIMIT $3`, query, offset, limit+1)
	if err != nil {
		return Page[UserHit]{}, fmt.Errorf("search users: %w", err)
	}
	hits, err := pgx.CollectRows(rows, scanUser)
	if err != nil {
		return Page[UserHit]{}, fmt.Errorf("scan users: %w", err)
	}
	return page(hits, limit), nil
}

// Hashtags: prefix matches first, then typos, then the most used.
func (p *Postgres) Hashtags(ctx context.Context, query string, offset, limit int) (Page[HashtagHit], error) {
	rows, err := p.pool.Query(ctx, `
		WITH q AS (SELECT search_fold(ltrim(btrim($1), '#')) AS folded)
		SELECT h.tag, h.videos_count
		FROM hashtags h CROSS JOIN q
		WHERE h.videos_count > 0 AND (search_fold(h.tag) LIKE `+likePrefix("q.folded")+` OR search_fold(h.tag) % q.folded)
		ORDER BY (search_fold(h.tag) LIKE `+likePrefix("q.folded")+`) DESC, similarity(search_fold(h.tag), q.folded) DESC,
			h.videos_count DESC, h.tag
		OFFSET $2 LIMIT $3`, query, offset, limit+1)
	if err != nil {
		return Page[HashtagHit]{}, fmt.Errorf("search hashtags: %w", err)
	}
	hits, err := pgx.CollectRows(rows, scanHashtag)
	if err != nil {
		return Page[HashtagHit]{}, fmt.Errorf("scan hashtags: %w", err)
	}
	return page(hits, limit), nil
}

// Suggest completes a username or a hashtag from its first characters.
func (p *Postgres) Suggest(ctx context.Context, prefix string, perKind int) (Suggestions, error) {
	clean := strings.ToLower(strings.TrimLeft(strings.TrimSpace(prefix), "@#"))
	if clean == "" {
		return Suggestions{Users: []UserHit{}, Hashtags: []HashtagHit{}}, nil
	}
	pattern := escapeLike(clean) + "%"
	rows, err := p.pool.Query(ctx, `
		SELECT user_id, username, display_name, avatar_url, is_creator, followers_count FROM users
		WHERE username IS NOT NULL AND lower(username) LIKE $1
		ORDER BY followers_count DESC, username LIMIT $2`, pattern, perKind)
	if err != nil {
		return Suggestions{}, fmt.Errorf("suggest users: %w", err)
	}
	users, err := pgx.CollectRows(rows, scanUser)
	if err != nil {
		return Suggestions{}, fmt.Errorf("scan users: %w", err)
	}
	rows, err = p.pool.Query(ctx, `
		SELECT tag, videos_count FROM hashtags
		WHERE videos_count > 0 AND search_fold(tag) LIKE search_fold($1)
		ORDER BY videos_count DESC, tag LIMIT $2`, pattern, perKind)
	if err != nil {
		return Suggestions{}, fmt.Errorf("suggest hashtags: %w", err)
	}
	tags, err := pgx.CollectRows(rows, scanHashtag)
	if err != nil {
		return Suggestions{}, fmt.Errorf("scan hashtags: %w", err)
	}
	return Suggestions{Users: users, Hashtags: tags}, nil
}

func scanUser(row pgx.CollectableRow) (UserHit, error) {
	var h UserHit
	err := row.Scan(&h.UserID, &h.Username, &h.DisplayName, &h.AvatarURL, &h.IsCreator, &h.Followers)
	return h, err
}

func scanHashtag(row pgx.CollectableRow) (HashtagHit, error) {
	var h HashtagHit
	err := row.Scan(&h.Tag, &h.VideosCount)
	return h, err
}

// likePrefix is the SQL for "starts with expr", with LIKE wildcards escaped.
func likePrefix(expr string) string {
	return `replace(replace(replace(` + expr + `, '\', '\\'), '%', '\%'), '_', '\_') || '%'`
}

// escapeLike escapes LIKE wildcards: hashtags often contain underscores.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func page[T any](hits []T, limit int) Page[T] {
	if hits == nil {
		hits = []T{}
	}
	if len(hits) > limit {
		return Page[T]{Items: hits[:limit], More: true}
	}
	return Page[T]{Items: hits}
}
