// Package service builds the following, trending and For You feeds.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/feed/internal/cursor"
	"github.com/poro/feed/internal/dto"
	"github.com/poro/feed/internal/mix"
	"github.com/poro/feed/internal/model"
	"github.com/poro/feed/internal/repository"
	"github.com/poro/feed/internal/session"
)

var (
	errCursor      = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_cursor", "invalid cursor")
	errUnavailable = httpx.NewAPIError(fiber.StatusServiceUnavailable, "unavailable", "feed temporarily unavailable")
	errInternal    = httpx.NewAPIError(fiber.StatusInternalServerError, "internal_error", "internal server error")
)

// Feed is the application service of the feeds.
type Feed struct {
	pool     *pgxpool.Pool
	sessions *session.Store
	cache    *session.Cache
	public   func(key string) string
	log      *zap.Logger
}

// New builds the service. public turns an object key into a client URL.
func New(pool *pgxpool.Pool, sessions *session.Store, cache *session.Cache, public func(string) string, log *zap.Logger) *Feed {
	return &Feed{pool: pool, sessions: sessions, cache: cache, public: public, log: log}
}

// Following returns the videos of followed accounts, newest first. Page 1 is
// cached for a minute per viewer.
func (f *Feed) Following(ctx context.Context, viewer uuid.UUID, rawCursor string, limit int) (*dto.Page, error) {
	after, err := cursor.DecodeTime(rawCursor)
	if err != nil {
		return nil, errCursor
	}
	limit = model.PageSize(limit)
	key := ""
	if after == nil {
		key = "following:" + viewer.String() + ":" + strconv.Itoa(limit)
	}
	return f.cached(ctx, key, func() (*dto.Page, error) {
		rows, err := repository.FollowingPage(ctx, f.pool, viewer, after, limit+1)
		if err != nil {
			return nil, err
		}
		return f.page(rows, limit, func(it repository.Item) string {
			return cursor.EncodeTime(cursor.Time{At: it.PublishedAt, VideoID: it.VideoID})
		}), nil
	})
}

// Trending returns the best scores of the last 72 hours. Page 1 is cached for
// a minute for everyone.
func (f *Feed) Trending(ctx context.Context, rawCursor string, limit int) (*dto.Page, error) {
	after, err := cursor.DecodeScore(rawCursor)
	if err != nil {
		return nil, errCursor
	}
	limit = model.PageSize(limit)
	key := ""
	if after == nil {
		key = "trending:" + strconv.Itoa(limit)
	}
	return f.cached(ctx, key, func() (*dto.Page, error) {
		rows, err := repository.TrendingPage(ctx, f.pool, after, limit+1)
		if err != nil {
			return nil, err
		}
		return f.page(rows, limit, func(it repository.Item) string {
			return cursor.EncodeScore(cursor.Score{Score: it.TrendingScore, VideoID: it.VideoID})
		}), nil
	})
}

// ForYou serves a 200-video session built once and paged from Redis. An
// expired or unknown session starts a new one instead of failing.
func (f *Feed) ForYou(ctx context.Context, viewer uuid.UUID, rawCursor string, limit int) (*dto.Page, error) {
	pos, err := cursor.DecodeSession(rawCursor)
	if err != nil {
		return nil, errCursor
	}
	limit = model.PageSize(limit)

	var ids []uuid.UUID
	var sessionID string
	offset := 0
	if pos != nil {
		loaded, ok, err := f.sessions.Load(ctx, viewer, pos.ID)
		if err != nil {
			return nil, errUnavailable.WithCause(err)
		}
		if ok {
			ids, sessionID, offset = loaded, pos.ID, min(pos.Offset, len(loaded))
		}
	}
	if sessionID == "" {
		if ids, err = f.buildSession(ctx, viewer); err != nil {
			return nil, errInternal.WithCause(err)
		}
		if sessionID, err = f.sessions.Create(ctx, viewer, ids); err != nil {
			return nil, errUnavailable.WithCause(err)
		}
	}

	end := min(offset+limit, len(ids))
	rows, err := repository.Hydrate(ctx, f.pool, ids[offset:end])
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	out := &dto.Page{Items: f.items(rows)}
	if end < len(ids) {
		next := cursor.EncodeSession(cursor.Session{ID: sessionID, Offset: end})
		out.NextCursor = &next
	}
	return out, nil
}

// buildSession mixes 60% following, 30% trending and 10% discovery, or 70%
// trending and 30% discovery when the viewer follows nobody who posted.
func (f *Feed) buildSession(ctx context.Context, viewer uuid.UUID) ([]uuid.UUID, error) {
	following, err := repository.FollowingCandidates(ctx, f.pool, viewer, model.FollowingCandidates)
	if err != nil {
		return nil, err
	}
	trending, err := repository.TrendingCandidates(ctx, f.pool, model.TrendingCandidates)
	if err != nil {
		return nil, err
	}
	discovery, err := repository.DiscoveryCandidates(ctx, f.pool, viewer, model.DiscoveryMaxAge,
		model.DiscoveryMaxFollowers, model.DiscoveryCandidates)
	if err != nil {
		return nil, err
	}
	weights := mix.Default
	if len(following) == 0 {
		weights = mix.ColdStart
	}
	mixed := mix.Build([3][]mix.Candidate{following, trending, discovery}, weights, viewer, model.SessionSize)
	ids := make([]uuid.UUID, len(mixed))
	for i, c := range mixed {
		ids[i] = c.VideoID
	}
	return ids, nil
}

// cached serves key from the page cache, or builds and stores the page. An
// empty key disables caching; a cache failure only costs a database read.
func (f *Feed) cached(ctx context.Context, key string, build func() (*dto.Page, error)) (*dto.Page, error) {
	if key != "" {
		raw, ok, err := f.cache.Get(ctx, key)
		if err != nil {
			f.log.Warn("feed cache read failed", zap.Error(err))
		} else if ok {
			var p dto.Page
			if err := json.Unmarshal(raw, &p); err == nil {
				return &p, nil
			}
		}
	}
	p, err := build()
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	if key != "" {
		raw, err := json.Marshal(p)
		if err != nil {
			return nil, errInternal.WithCause(fmt.Errorf("encode page: %w", err))
		}
		if err := f.cache.Set(ctx, key, raw); err != nil {
			f.log.Warn("feed cache write failed", zap.Error(err))
		}
	}
	return p, nil
}

// page keeps limit rows; the extra row read tells whether a next page exists.
func (f *Feed) page(rows []repository.Item, limit int, next func(repository.Item) string) *dto.Page {
	p := &dto.Page{}
	if len(rows) > limit {
		rows = rows[:limit]
		c := next(rows[limit-1])
		p.NextCursor = &c
	}
	p.Items = f.items(rows)
	return p
}

func (f *Feed) items(rows []repository.Item) []dto.Item {
	out := make([]dto.Item, len(rows))
	for i, r := range rows {
		hashtags := r.Hashtags
		if hashtags == nil {
			hashtags = []string{}
		}
		out[i] = dto.Item{
			VideoID: r.VideoID.String(), Title: r.Title, Hashtags: hashtags, DurationMs: r.DurationMs,
			PublishedAt: r.PublishedAt, ThumbnailURL: f.public(r.ThumbnailKey), HLSURL: f.public(r.HLSKey),
			Stats: dto.Stats{Likes: r.Likes, Comments: r.Comments, Shares: r.Shares},
			Author: dto.Author{
				ID: r.AuthorID.String(), Username: r.Username, DisplayName: r.DisplayName, AvatarURL: r.AvatarURL,
			},
		}
	}
	return out
}
