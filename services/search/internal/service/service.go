// Package service validates search requests and shapes the results.
package service

import (
	"context"
	"slices"
	"strconv"
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
	errType = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_type",
		"type must be videos, users, hashtags, products or shops")
	errFilter = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_filter",
		"country and currency apply to products and shops, min_price and max_price to products; "+
			"prices are whole minor units with min_price <= max_price")
	errCursor   = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_cursor", "invalid cursor")
	errInternal = httpx.NewAPIError(fiber.StatusInternalServerError, "internal_error", "internal server error")
)

// Filters are the raw catalogue filters of a search; empty means unset.
type Filters struct {
	Country  string
	Currency string
	MinPrice string
	MaxPrice string
}

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
func (s *Search) Search(ctx context.Context, query, kind, rawCursor string, limit int, raw Filters) (*dto.Results, error) {
	q, err := cleanQuery(query)
	if err != nil {
		return nil, err
	}
	if kind == "" {
		kind = model.TypeVideos
	}
	filter, err := catalogFilter(kind, raw)
	if err != nil {
		return nil, err
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
	case model.TypeProducts:
		page, err := s.idx.Products(ctx, q, filter, offset, limit)
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		out.Items, more = products(page.Items), page.More
	case model.TypeShops:
		page, err := s.idx.Shops(ctx, q, filter, offset, limit)
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		out.Items, more = shops(page.Items), page.More
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

// catalogFilter parses the filters; any filter the kind cannot apply is an
// error rather than silently ignored.
func catalogFilter(kind string, raw Filters) (index.CatalogFilter, error) {
	var f index.CatalogFilter
	catalog := kind == model.TypeProducts || kind == model.TypeShops
	if !catalog && raw != (Filters{}) {
		return f, errFilter
	}
	if kind != model.TypeProducts && (raw.MinPrice != "" || raw.MaxPrice != "") {
		return f, errFilter
	}
	if raw.Country != "" {
		c := strings.ToUpper(raw.Country)
		if !slices.Contains(model.CountryCodes, c) {
			return f, errFilter
		}
		f.CountryCode = &c
	}
	if raw.Currency != "" {
		c := strings.ToUpper(raw.Currency)
		if !slices.Contains(model.Currencies, c) {
			return f, errFilter
		}
		f.Currency = &c
	}
	for _, p := range []struct {
		raw string
		dst **int64
	}{{raw.MinPrice, &f.MinPrice}, {raw.MaxPrice, &f.MaxPrice}} {
		if p.raw == "" {
			continue
		}
		v, err := strconv.ParseInt(p.raw, 10, 64)
		if err != nil || v < 0 {
			return f, errFilter
		}
		*p.dst = &v
	}
	if f.MinPrice != nil && f.MaxPrice != nil && *f.MinPrice > *f.MaxPrice {
		return f, errFilter
	}
	return f, nil
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

func products(hits []index.ProductHit) []dto.Product {
	out := make([]dto.Product, 0, len(hits))
	for _, h := range hits {
		out = append(out, dto.Product{ProductID: h.ProductID.String(), Title: h.Title, ImageURL: h.ImageURL,
			MinPrice: h.MinPrice, Currency: h.Currency, InStock: h.InStock, Shop: shopRef(h.Shop)})
	}
	return out
}

func shops(hits []index.ShopHit) []dto.Shop {
	out := make([]dto.Shop, 0, len(hits))
	for _, h := range hits {
		out = append(out, dto.Shop{ShopRef: shopRef(h.ShopRef), Currency: h.Currency})
	}
	return out
}

func shopRef(r index.ShopRef) dto.ShopRef {
	return dto.ShopRef{ShopID: r.ShopID.String(), Name: r.Name, Handle: r.Handle, LogoURL: r.LogoURL,
		CountryCode: r.CountryCode}
}
