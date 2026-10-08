// Package service validates search requests and shapes the results.
package service

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/search/internal/cursor"
	"github.com/poro/search/internal/dto"
	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/model"
)

var (
	errQuery = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_query",
		"q must be 1 to 100 characters without control characters")
	errType     = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_type", "type must be videos, users or hashtags")
	errCursor   = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_cursor", "invalid cursor")
	errInternal = httpx.NewAPIError(fiber.StatusInternalServerError, "internal_error", "internal server error")
)

// Search is the application service of the search API.
type Search struct {
	idx       index.Index
	publicURL func(key string) string
}

// New builds the service; publicURL turns an object key into a client URL.
func New(idx index.Index, publicURL func(string) string) *Search {
	return &Search{idx: idx, publicURL: publicURL}
}

// Search returns one ranked page of kind for query.
func (s *Search) Search(ctx context.Context, query, kind, rawCursor string, limit int) (*dto.Results, error) {
	q, err := cleanQuery(query)
	if err != nil {
		return nil, err
	}
	if kind == "" {
		kind = model.TypeVideos
	}
	offset, err := cursor.Decode(rawCursor, model.MaxResults)
	if err != nil {
		return nil, errCursor
	}
	limit = min(model.PageSize(limit), model.MaxResults-offset)
	out := &dto.Results{Type: kind}
	more := false
	switch kind {
	case model.TypeVideos:
		page, err := s.idx.Videos(ctx, q, offset, limit)
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		items := make([]dto.Video, 0, len(page.Items))
		for _, h := range page.Items {
			items = append(items, s.video(h))
		}
		out.Items, more = items, page.More
	case model.TypeUsers:
		page, err := s.idx.Users(ctx, q, offset, limit)
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		out.Items, more = users(page.Items), page.More
	case model.TypeHashtags:
		page, err := s.idx.Hashtags(ctx, q, offset, limit)
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		out.Items, more = hashtags(page.Items), page.More
	default:
		return nil, errType
	}
	if more && offset+limit < model.MaxResults {
		next := cursor.Encode(offset + limit)
		out.NextCursor = &next
	}
	return out, nil
}

// Suggest completes usernames and hashtags for the search box.
func (s *Search) Suggest(ctx context.Context, prefix string) (*dto.Suggestions, error) {
	q, err := cleanQuery(prefix)
	if err != nil {
		return nil, err
	}
	got, err := s.idx.Suggest(ctx, q, model.SuggestPerKind)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	return &dto.Suggestions{Users: users(got.Users), Hashtags: hashtags(got.Hashtags)}, nil
}

func cleanQuery(raw string) (string, error) {
	q := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(q)
	if n == 0 || n > model.MaxQueryRunes || !utf8.ValidString(q) {
		return "", errQuery
	}
	for _, r := range q {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return "", errQuery
		}
	}
	return q, nil
}

func (s *Search) video(h index.VideoHit) dto.Video {
	v := dto.Video{
		VideoID: h.VideoID.String(), Title: h.Title, Hashtags: h.Hashtags, DurationMs: h.DurationMs,
		PublishedAt: h.PublishedAt, Stats: dto.Stats{Likes: h.Likes, Comments: h.Comments, Shares: h.Shares},
		Author: dto.Author{UserID: h.Author.UserID.String(), Username: h.Author.Username,
			DisplayName: h.Author.DisplayName, AvatarURL: h.Author.AvatarURL},
	}
	if v.Hashtags == nil {
		v.Hashtags = []string{}
	}
	if h.ThumbnailKey != nil && *h.ThumbnailKey != "" {
		u := s.publicURL(*h.ThumbnailKey)
		v.ThumbnailURL = &u
	}
	return v
}

func users(hits []index.UserHit) []dto.User {
	out := make([]dto.User, 0, len(hits))
	for _, h := range hits {
		out = append(out, dto.User{UserID: h.UserID.String(), Username: h.Username, DisplayName: h.DisplayName,
			AvatarURL: h.AvatarURL, IsCreator: h.IsCreator, Followers: h.Followers})
	}
	return out
}

func hashtags(hits []index.HashtagHit) []dto.Hashtag {
	out := make([]dto.Hashtag, 0, len(hits))
	for _, h := range hits {
		out = append(out, dto.Hashtag{Tag: h.Tag, VideosCount: h.VideosCount})
	}
	return out
}
