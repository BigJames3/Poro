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
		_, err := svc.Search(ctx, q, "", "", 0)
		require.Equal(t, "invalid_query", code(t, err), "%q", q)
	}
	_, err := svc.Search(ctx, "x", "shops", "", 0)
	require.Equal(t, "invalid_type", code(t, err))
	_, err = svc.Search(ctx, "x", "", "bad!", 0)
	require.Equal(t, "invalid_cursor", code(t, err))
	_, err = svc.Suggest(ctx, "")
	require.Equal(t, "invalid_query", code(t, err))
}

func TestPagingAndShapes(t *testing.T) {
	thumb := "videos/a/v/thumb.jpg"
	f := &fakeIndex{more: true, thumbnail: &thumb}
	svc := New(f, func(k string) string { return "https://cdn/" + k })
	ctx := context.Background()

	out, err := svc.Search(ctx, " abidjan ", "", "", 0)
	require.NoError(t, err)
	require.Equal(t, model.TypeVideos, out.Type)
	require.Equal(t, model.DefaultPageSize, f.gotLimit)
	require.NotNil(t, out.NextCursor)
	off, err := cursor.Decode(*out.NextCursor, model.MaxResults)
	require.NoError(t, err)
	require.Equal(t, model.DefaultPageSize, off)

	_, err = svc.Search(ctx, "x", model.TypeUsers, cursor.Encode(490), 50)
	require.NoError(t, err)
	require.Equal(t, 490, f.gotOffset)
	require.Equal(t, 10, f.gotLimit, "never past MaxResults")
	last, err := svc.Search(ctx, "x", model.TypeHashtags, cursor.Encode(490), 50)
	require.NoError(t, err)
	require.Nil(t, last.NextCursor, "no cursor past MaxResults")

	f.more = false
	out, err = svc.Search(ctx, "x", model.TypeVideos, "", 5)
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
	for _, kind := range []string{model.TypeVideos, model.TypeUsers, model.TypeHashtags} {
		_, err := svc.Search(ctx, "x", kind, "", 0)
		require.Equal(t, "internal_error", code(t, err), kind)
	}
	_, err := svc.Suggest(ctx, "x")
	require.Equal(t, "internal_error", code(t, err))
}
