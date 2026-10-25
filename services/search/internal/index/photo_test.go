package index_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/repository"
	"github.com/poro/search/internal/testdb"
)

func seedPhoto(t *testing.T, caption string, tags ...string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	require.NoError(t, repository.UpsertReadyVideo(context.Background(), testdb.Pool, repository.Video{
		VideoID: id, AuthorID: uuid.Must(uuid.NewV7()), Description: caption, Hashtags: tags,
		ThumbnailKey: "p/thumb-1.jpg", PublishedAt: time.Now(), MediaType: repository.MediaPhoto,
	}))
	return id
}

// ADR-0020 § 5: without photo-posts, search returns videos only and counts
// hashtags on videos only; a tag carried by photo posts alone is not found.
func TestPhotoPostsAreSearchedOnlyWithTheCapability(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	idx := index.NewPostgres(testdb.Pool)
	word := "treichville" + uuid.NewString()[:6]
	shared, only := "marche"+word, "photoonly"+word
	vid := seedVideo(t, word, shared)
	pic := seedPhoto(t, "Le marché de "+word, shared, only)

	videos, err := idx.Videos(ctx, word, false, 0, 10)
	require.NoError(t, err)
	require.Len(t, videos.Items, 1)
	require.Equal(t, vid, videos.Items[0].VideoID)
	require.Equal(t, "video", videos.Items[0].MediaType)

	all, err := idx.Videos(ctx, word, true, 0, 10)
	require.NoError(t, err)
	found := map[uuid.UUID]string{}
	for _, h := range all.Items {
		found[h.VideoID] = h.MediaType
	}
	require.Equal(t, map[uuid.UUID]string{vid: "video", pic: "photo"}, found, "the caption of a photo post is searched")

	tags, err := idx.Hashtags(ctx, shared, false, 0, 10)
	require.NoError(t, err)
	require.Equal(t, shared, tags.Items[0].Tag)
	require.EqualValues(t, 1, tags.Items[0].VideosCount, "videos only")
	tags, err = idx.Hashtags(ctx, shared, true, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 2, tags.Items[0].VideosCount, "videos and photo posts")

	hidden, err := idx.Hashtags(ctx, only, false, 0, 10)
	require.NoError(t, err)
	for _, h := range hidden.Items {
		require.NotEqual(t, only, h.Tag, "a tag of photo posts alone is hidden without the capability")
	}
	sug, err := idx.Suggest(ctx, "#"+only, false, 10)
	require.NoError(t, err)
	require.Empty(t, sug.Hashtags)
	sug, err = idx.Suggest(ctx, "#"+only, true, 10)
	require.NoError(t, err)
	require.Len(t, sug.Hashtags, 1)

	// Deleting the photo post uncounts it from photos, never from videos.
	require.NoError(t, repository.DeleteVideo(ctx, testdb.Pool, pic, uuid.Must(uuid.NewV7()), time.Now()))
	tags, err = idx.Hashtags(ctx, shared, true, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, tags.Items[0].VideosCount)
	var videosCount, photosCount int64
	require.NoError(t, testdb.Pool.QueryRow(ctx, `SELECT videos_count, photos_count FROM hashtags WHERE tag = $1`, only).
		Scan(&videosCount, &photosCount))
	require.Zero(t, videosCount)
	require.Zero(t, photosCount)
}
