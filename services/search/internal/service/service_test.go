package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/search/internal/cursor"
	"github.com/poro/search/internal/dto"
	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/model"
)

type fakeIndex struct {
	err        error
	gotOffset  int
	gotLimit   int
	more       bool
	thumbnail  *string
	suggestion index.Suggestions
	gotFilter  index.CatalogFilter
}

func (f *fakeIndex) Videos(_ context.Context, _ string, offset, limit int) (index.Page[index.VideoHit], error) {
	f.gotOffset, f.gotLimit = offset, limit
	return index.Page[index.VideoHit]{Items: []index.VideoHit{{VideoID: uuid.New(), ThumbnailKey: f.thumbnail}}, More: f.more}, f.err
}

func (f *fakeIndex) Users(_ context.Context, _ string, offset, limit int) (index.Page[index.UserHit], error) {
	f.gotOffset, f.gotLimit = offset, limit
	return index.Page[index.UserHit]{Items: []index.UserHit{{UserID: uuid.New(), Username: "awa"}}, More: f.more}, f.err
}

func (f *fakeIndex) Hashtags(_ context.Context, _ string, offset, limit int) (index.Page[index.HashtagHit], error) {
	f.gotOffset, f.gotLimit = offset, limit
	return index.Page[index.HashtagHit]{Items: []index.HashtagHit{{Tag: "abidjan", VideosCount: 2}}, More: f.more}, f.err
}

func (f *fakeIndex) Products(_ context.Context, _ string, filter index.CatalogFilter, offset, limit int) (index.Page[index.ProductHit], error) {
	f.gotOffset, f.gotLimit, f.gotFilter = offset, limit, filter
	price := int64(15000)
	return index.Page[index.ProductHit]{Items: []index.ProductHit{{ProductID: uuid.New(), Title: "Pagne", MinPrice: &price,
		Currency: "XOF", InStock: true, Shop: index.ShopRef{ShopID: uuid.New(), Name: "Chez Awa", Handle: "chez.awa", CountryCode: "CI"}}}}, f.err
}

func (f *fakeIndex) Shops(_ context.Context, _ string, filter index.CatalogFilter, offset, limit int) (index.Page[index.ShopHit], error) {
	f.gotOffset, f.gotLimit, f.gotFilter = offset, limit, filter
	return index.Page[index.ShopHit]{Items: []index.ShopHit{{ShopRef: index.ShopRef{ShopID: uuid.New(), Name: "Chez Awa",
		Handle: "chez.awa", CountryCode: "CI"}, Currency: "XOF"}}}, f.err
}

func (f *fakeIndex) Suggest(context.Context, string, int) (index.Suggestions, error) {
	return f.suggestion, f.err
}

func code(t *testing.T, err error) string {
	t.Helper()
	var apiErr *httpx.APIError
	require.True(t, errors.As(err, &apiErr), "%v", err)
	return apiErr.Code
}

func TestValidation(t *testing.T) {
	svc := New(&fakeIndex{}, func(k string) string { return k })
	ctx := context.Background()
	for _, q := range []string{"", "   ", strings.Repeat("a", 101), "a\u202eb", "a\x00b"} {
		_, err := svc.Search(ctx, q, "", "", 0, Filters{})
		require.Equal(t, "invalid_query", code(t, err), "%q", q)
	}
	_, err := svc.Search(ctx, "x", "places", "", 0, Filters{})
	require.Equal(t, "invalid_type", code(t, err))
	_, err = svc.Search(ctx, "x", "", "bad!", 0, Filters{})
	require.Equal(t, "invalid_cursor", code(t, err))
	_, err = svc.Suggest(ctx, "")
	require.Equal(t, "invalid_query", code(t, err))
}

func TestPagingAndShapes(t *testing.T) {
	thumb := "videos/a/v/thumb.jpg"
	f := &fakeIndex{more: true, thumbnail: &thumb}
	svc := New(f, func(k string) string { return "https://cdn/" + k })
	ctx := context.Background()

	out, err := svc.Search(ctx, " abidjan ", "", "", 0, Filters{})
	require.NoError(t, err)
	require.Equal(t, model.TypeVideos, out.Type)
	require.Equal(t, model.DefaultPageSize, f.gotLimit)
	require.NotNil(t, out.NextCursor)
	off, err := cursor.Decode(*out.NextCursor, model.MaxResults)
	require.NoError(t, err)
	require.Equal(t, model.DefaultPageSize, off)

	_, err = svc.Search(ctx, "x", model.TypeUsers, cursor.Encode(490), 50, Filters{})
	require.NoError(t, err)
	require.Equal(t, 490, f.gotOffset)
	require.Equal(t, 10, f.gotLimit, "never past MaxResults")
	last, err := svc.Search(ctx, "x", model.TypeHashtags, cursor.Encode(490), 50, Filters{})
	require.NoError(t, err)
	require.Nil(t, last.NextCursor, "no cursor past MaxResults")

	f.more = false
	out, err = svc.Search(ctx, "x", model.TypeVideos, "", 5, Filters{})
	require.NoError(t, err)
	require.Nil(t, out.NextCursor)

	f.suggestion = index.Suggestions{Users: []index.UserHit{{UserID: uuid.New(), Username: "awa"}}}
	sug, err := svc.Suggest(ctx, "aw")
	require.NoError(t, err)
	require.Len(t, sug.Users, 1)
	require.Empty(t, sug.Hashtags)
	require.NotNil(t, sug.Hashtags, "always a JSON array")
}

func TestIndexFailuresAre500(t *testing.T) {
	svc := New(&fakeIndex{err: errors.New("db down")}, func(k string) string { return k })
	ctx := context.Background()
	for _, kind := range []string{model.TypeVideos, model.TypeUsers, model.TypeHashtags, model.TypeProducts, model.TypeShops} {
		_, err := svc.Search(ctx, "x", kind, "", 0, Filters{})
		require.Equal(t, "internal_error", code(t, err), kind)
	}
	_, err := svc.Suggest(ctx, "x")
	require.Equal(t, "internal_error", code(t, err))
}

func TestCatalogFilters(t *testing.T) {
	idx := &fakeIndex{}
	svc := New(idx, func(k string) string { return k })
	ctx := context.Background()

	out, err := svc.Search(ctx, "pagne", model.TypeProducts, "", 0,
		Filters{Country: "ci", Currency: "xof", MinPrice: "1000", MaxPrice: "20000"})
	require.NoError(t, err)
	require.Equal(t, "CI", *idx.gotFilter.CountryCode)
	require.Equal(t, "XOF", *idx.gotFilter.Currency)
	require.Equal(t, int64(1000), *idx.gotFilter.MinPrice)
	require.Equal(t, int64(20000), *idx.gotFilter.MaxPrice)
	items := out.Items.([]dto.Product)
	require.Len(t, items, 1)
	require.Equal(t, "chez.awa", items[0].Shop.Handle)

	shopsOut, err := svc.Search(ctx, "awa", model.TypeShops, "", 0, Filters{Country: "SN"})
	require.NoError(t, err)
	require.Len(t, shopsOut.Items.([]dto.Shop), 1)
	require.Equal(t, "SN", *idx.gotFilter.CountryCode)
	require.Nil(t, idx.gotFilter.MinPrice)

	for _, c := range []struct {
		kind string
		f    Filters
	}{
		{model.TypeVideos, Filters{Country: "CI"}},
		{model.TypeShops, Filters{MinPrice: "10"}},
		{model.TypeProducts, Filters{Country: "FR"}},
		{model.TypeProducts, Filters{Currency: "EUR"}},
		{model.TypeProducts, Filters{MinPrice: "-1"}},
		{model.TypeProducts, Filters{MaxPrice: "12.5"}},
		{model.TypeProducts, Filters{MinPrice: "200", MaxPrice: "100"}},
	} {
		_, err := svc.Search(ctx, "x", c.kind, "", 0, c.f)
		require.Equal(t, "invalid_filter", code(t, err), "%+v", c)
	}
}
