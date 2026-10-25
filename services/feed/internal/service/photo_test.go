package service_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/feed/internal/dto"
	"github.com/poro/feed/internal/repository"
	"github.com/poro/feed/internal/testdb"
)

func photo(t *testing.T, author uuid.UUID, published time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	require.NoError(t, repository.UpsertReadyVideo(ctx, testdb.Pool, repository.Video{
		VideoID: id, AuthorID: author, Title: "", ThumbnailKey: "p/thumb-1.jpg", PublishedAt: published,
		MediaType: repository.MediaPhoto, Images: []repository.Image{
			{ImageKey: "p/image-1.jpg", ThumbnailKey: "p/thumb-1.jpg", Width: 1536, Height: 2048},
			{ImageKey: "p/image-2.jpg", ThumbnailKey: "p/thumb-2.jpg", Width: 2048, Height: 1536},
		},
	}))
	return id
}

func find(items []dto.Item, id uuid.UUID) *dto.Item {
	for i := range items {
		if items[i].VideoID == id.String() {
			return &items[i]
		}
	}
	return nil
}

// ADR-0020 § 5: a client without photo-posts never receives a photo post,
// on any feed, page 1 cached or not; with it, photo posts come with their
// images and no HLS.
func TestPhotoPostsOnlyReachClientsThatDeclareThem(t *testing.T) {
	e := newEnv(t)
	viewer, author := uuid.New(), uuid.New()
	follow(t, viewer, author)
	now := time.Now().UTC()
	vid := video(t, author, now.Add(-2*time.Minute))
	pic := photo(t, author, now.Add(-time.Minute))
	engage(t, vid, 5)
	engage(t, pic, 50)

	for _, round := range []string{"database", "cache"} {
		following, err := e.svc.Following(ctx, viewer, false, "", 10)
		require.NoError(t, err)
		require.Equal(t, []string{vid.String()}, ids(following.Items), "following without capability (%s)", round)
		trending, err := e.svc.Trending(ctx, false, "", 10)
		require.NoError(t, err)
		require.Equal(t, []string{vid.String()}, ids(trending.Items), "trending without capability (%s)", round)

		following, err = e.svc.Following(ctx, viewer, true, "", 10)
		require.NoError(t, err)
		require.Equal(t, []string{pic.String(), vid.String()}, ids(following.Items), "following with capability (%s)", round)
		trending, err = e.svc.Trending(ctx, true, "", 10)
		require.NoError(t, err)
		require.Equal(t, []string{pic.String(), vid.String()}, ids(trending.Items), "trending with capability (%s)", round)
	}

	forYou, err := e.svc.ForYou(ctx, viewer, false, "", 30)
	require.NoError(t, err)
	require.Nil(t, find(forYou.Items, pic), "for you without capability")
	require.NotNil(t, find(forYou.Items, vid))

	forYou, err = e.svc.ForYou(ctx, viewer, true, "", 30)
	require.NoError(t, err)
	item := find(forYou.Items, pic)
	require.NotNil(t, item, "for you with capability")
	require.Equal(t, "photo", item.MediaType)
	require.Empty(t, item.HLSURL, "a photo post has no HLS")
	require.Zero(t, item.DurationMs)
	require.Equal(t, "https://cdn.poro.test/p/thumb-1.jpg", item.ThumbnailURL)
	require.Equal(t, []dto.Image{
		{URL: "https://cdn.poro.test/p/image-1.jpg", ThumbnailURL: "https://cdn.poro.test/p/thumb-1.jpg", Width: 1536, Height: 2048},
		{URL: "https://cdn.poro.test/p/image-2.jpg", ThumbnailURL: "https://cdn.poro.test/p/thumb-2.jpg", Width: 2048, Height: 1536},
	}, item.Images)
	videoItem := find(forYou.Items, vid)
	require.Equal(t, "video", videoItem.MediaType)
	require.NotNil(t, videoItem.Images, "images is an empty array for a video, never null")
	require.Empty(t, videoItem.Images)
	require.Equal(t, "https://cdn.poro.test/master.m3u8", videoItem.HLSURL)
}

// A cursor issued with photo posts and replayed without the header is
// refused: the next page of a photo session is never served to a client
// that cannot display it.
func TestPhotoCursorsAreRefusedWithoutTheCapability(t *testing.T) {
	e := newEnv(t)
	viewer, author := uuid.New(), uuid.New()
	follow(t, viewer, author)
	now := time.Now().UTC()
	for i := range 3 {
		engage(t, photo(t, author, now.Add(-time.Duration(i)*time.Minute)), int64(10+i))
	}

	following, err := e.svc.Following(ctx, viewer, true, "", 1)
	require.NoError(t, err)
	require.NotNil(t, following.NextCursor)
	_, err = e.svc.Following(ctx, viewer, false, *following.NextCursor, 1)
	requireCode(t, err, "invalid_cursor")

	trending, err := e.svc.Trending(ctx, true, "", 1)
	require.NoError(t, err)
	require.NotNil(t, trending.NextCursor)
	_, err = e.svc.Trending(ctx, false, *trending.NextCursor, 1)
	requireCode(t, err, "invalid_cursor")

	forYou, err := e.svc.ForYou(ctx, viewer, true, "", 1)
	require.NoError(t, err)
	require.NotNil(t, forYou.NextCursor)
	_, err = e.svc.ForYou(ctx, viewer, false, *forYou.NextCursor, 1)
	requireCode(t, err, "invalid_cursor")

	next, err := e.svc.ForYou(ctx, viewer, true, *forYou.NextCursor, 1)
	require.NoError(t, err)
	require.Len(t, next.Items, 1)
}
