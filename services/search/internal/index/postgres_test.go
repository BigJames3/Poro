package index_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/repository"
	"github.com/poro/search/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

func seedVideo(t *testing.T, title string, tags ...string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	require.NoError(t, repository.UpsertReadyVideo(context.Background(), testdb.Pool, repository.Video{
		VideoID: id, AuthorID: uuid.Must(uuid.NewV7()), Title: title, Hashtags: tags, PublishedAt: time.Now(),
	}))
	return id
}

func seedUser(t *testing.T, username string, followers int64) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	require.NoError(t, repository.UpsertProfile(context.Background(), testdb.Pool, repository.Profile{
		UserID: id, Username: &username, UpdatedAt: time.Now(),
	}))
	require.NoError(t, repository.AddFollowers(context.Background(), testdb.Pool, id, followers))
	return id
}

func TestHashtagsPrefixTyposAndWildcards(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	idx := index.NewPostgres(testdb.Pool)
	suffix := uuid.NewString()[:6]
	seedVideo(t, "a", "coupé_décalé"+suffix)
	seedVideo(t, "b", "coupé_décalé"+suffix, "coupe"+suffix)
	seedVideo(t, "c", "coupeXdecale"+suffix)

	page, err := idx.Hashtags(ctx, "#coupe_decale"+suffix, 0, 10)
	require.NoError(t, err)
	require.Equal(t, "coupé_décalé"+suffix, page.Items[0].Tag, "accents are folded and # is ignored")
	require.EqualValues(t, 2, page.Items[0].VideosCount)

	sug, err := idx.Suggest(ctx, "#coupe_", 50)
	require.NoError(t, err)
	tags := map[string]bool{}
	for _, h := range sug.Hashtags {
		tags[h.Tag] = true
	}
	require.True(t, tags["coupé_décalé"+suffix])
	require.False(t, tags["coupeXdecale"+suffix], "an underscore in the prefix is not a wildcard")
}

func TestUserSuggestionsAndPaging(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	idx := index.NewPostgres(testdb.Pool)
	base := "kofi" + uuid.NewString()[:6]
	small := seedUser(t, base+"_a", 1)
	big := seedUser(t, base+"_b", 100)

	sug, err := idx.Suggest(ctx, "@"+base, 5)
	require.NoError(t, err)
	require.Len(t, sug.Users, 2)
	require.Equal(t, big, sug.Users[0].UserID, "more followers first")
	require.Equal(t, small, sug.Users[1].UserID)

	empty, err := idx.Suggest(ctx, " @# ", 5)
	require.NoError(t, err)
	require.Empty(t, empty.Users)
	require.Empty(t, empty.Hashtags)

	word := "paging" + uuid.NewString()[:6]
	for i := range 5 {
		seedVideo(t, fmt.Sprintf("%s %d", word, i))
	}
	first, err := idx.Videos(ctx, word, 0, 3)
	require.NoError(t, err)
	require.Len(t, first.Items, 3)
	require.True(t, first.More)
	second, err := idx.Videos(ctx, word, 3, 3)
	require.NoError(t, err)
	require.Len(t, second.Items, 2)
	require.False(t, second.More)
	for _, a := range first.Items {
		for _, b := range second.Items {
			require.NotEqual(t, a.VideoID, b.VideoID)
		}
	}

	none, err := idx.Videos(ctx, "le la les", 0, 3)
	require.NoError(t, err, "a query of stop words is not an error")
	require.NotNil(t, none.Items)
}

func TestCancelledQueriesFail(t *testing.T) {
	testdb.Available(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	idx := index.NewPostgres(testdb.Pool)
	_, err := idx.Videos(ctx, "x", 0, 1)
	require.Error(t, err)
	_, err = idx.Users(ctx, "x", 0, 1)
	require.Error(t, err)
	_, err = idx.Hashtags(ctx, "x", 0, 1)
	require.Error(t, err)
	_, err = idx.Suggest(ctx, "x", 1)
	require.Error(t, err)
}
