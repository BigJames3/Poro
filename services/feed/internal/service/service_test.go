package service_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/feed/internal/cursor"
	"github.com/poro/feed/internal/dto"
	"github.com/poro/feed/internal/model"
	"github.com/poro/feed/internal/repository"
	"github.com/poro/feed/internal/service"
	"github.com/poro/feed/internal/session"
	"github.com/poro/feed/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

type env struct {
	svc *service.Feed
	mr  *miniredis.Miniredis
}

func newEnv(t *testing.T) env {
	t.Helper()
	testdb.Available(t)
	ctx := context.Background()
	// Each test sees only its own rows.
	_, err := testdb.Pool.Exec(ctx, `TRUNCATE videos, follows, author_followers, video_stats, authors`)
	require.NoError(t, err)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	svc := service.New(testdb.Pool, session.NewStore(rdb, model.SessionTTL), session.NewCache(rdb, model.FirstPageTTL),
		func(key string) string { return "https://cdn.poro.test/" + key }, zap.NewNop())
	return env{svc: svc, mr: mr}
}

var ctx = context.Background()

func video(t *testing.T, author uuid.UUID, published time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	require.NoError(t, repository.UpsertReadyVideo(ctx, testdb.Pool, repository.Video{
		VideoID: id, AuthorID: author, Title: "t", ThumbnailKey: "thumb.jpg", HLSKey: "master.m3u8",
		DurationMs: 1000, PublishedAt: published,
	}))
	return id
}

func follow(t *testing.T, follower, following uuid.UUID) {
	t.Helper()
	require.NoError(t, repository.AddFollow(ctx, testdb.Pool, follower, following, time.Now()))
}

func engage(t *testing.T, id uuid.UUID, likes int64) {
	t.Helper()
	require.NoError(t, repository.AddStats(ctx, testdb.Pool, id, likes, 0, 0))
}

func ids(items []dto.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.VideoID
	}
	return out
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var apiErr *httpx.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, code, apiErr.Code)
}

func TestFollowingKeysetHasNoGapOrDuplicate(t *testing.T) {
	e := newEnv(t)
	viewer, a, b, stranger := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	follow(t, viewer, a)
	follow(t, viewer, b)
	same := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	want := map[string]bool{}
	for i := range 13 {
		at := same
		if i%3 == 0 {
			at = same.Add(-time.Duration(i) * time.Minute)
		}
		want[video(t, []uuid.UUID{a, b}[i%2], at).String()] = true
	}
	video(t, stranger, same)
	gone := video(t, a, same.Add(time.Minute))
	require.NoError(t, repository.DeleteVideo(ctx, testdb.Pool, gone, a, time.Now()))

	seen := map[string]bool{}
	var last *dto.Item
	cur, pages := "", 0
	for {
		p, err := e.svc.Following(ctx, viewer, cur, 4)
		require.NoError(t, err)
		pages++
		for i := range p.Items {
			it := p.Items[i]
			require.False(t, seen[it.VideoID], "duplicate across pages")
			seen[it.VideoID] = true
			if last != nil {
				require.False(t, it.PublishedAt.After(last.PublishedAt), "newest first")
			}
			last = &it
		}
		if p.NextCursor == nil {
			break
		}
		cur = *p.NextCursor
	}
	require.Equal(t, want, seen, "every followed video exactly once, nothing else")
	require.Equal(t, 4, pages)
}

func TestFollowingFirstPageIsCached(t *testing.T) {
	e := newEnv(t)
	viewer, a := uuid.New(), uuid.New()
	follow(t, viewer, a)
	video(t, a, time.Now())
	p, err := e.svc.Following(ctx, viewer, "", 10)
	require.NoError(t, err)
	require.Len(t, p.Items, 1)
	require.Equal(t, "https://cdn.poro.test/thumb.jpg", p.Items[0].ThumbnailURL)

	video(t, a, time.Now())
	p, err = e.svc.Following(ctx, viewer, "", 10)
	require.NoError(t, err)
	require.Len(t, p.Items, 1, "served from cache within 60 seconds")

	e.mr.FastForward(61 * time.Second)
	p, err = e.svc.Following(ctx, viewer, "", 10)
	require.NoError(t, err)
	require.Len(t, p.Items, 2)

	_, err = e.svc.Following(ctx, viewer, "garbage", 10)
	requireCode(t, err, "invalid_cursor")
}

func TestFeedsWorkWhenTheCacheIsDown(t *testing.T) {
	e := newEnv(t)
	viewer, a := uuid.New(), uuid.New()
	follow(t, viewer, a)
	video(t, a, time.Now())
	e.mr.Close()
	p, err := e.svc.Following(ctx, viewer, "", 10)
	require.NoError(t, err, "a cache outage costs a database read, not an error")
	require.Len(t, p.Items, 1)
	_, err = e.svc.ForYou(ctx, viewer, "", 10)
	requireCode(t, err, "unavailable")
}

func TestTrendingOrderWindowAndDecay(t *testing.T) {
	e := newEnv(t)
	a := uuid.New()
	hot := video(t, a, time.Now().Add(-time.Hour))
	warm := video(t, a, time.Now().Add(-time.Hour))
	old := video(t, a, time.Now().Add(-80*time.Hour))
	video(t, a, time.Now())
	engage(t, hot, 50)
	engage(t, warm, 5)
	engage(t, old, 1000)

	p, err := e.svc.Trending(ctx, "", 1)
	require.NoError(t, err)
	require.Equal(t, []string{hot.String()}, ids(p.Items))
	p, err = e.svc.Trending(ctx, *p.NextCursor, 10)
	require.NoError(t, err)
	require.Equal(t, []string{warm.String()}, ids(p.Items), "outside 72 h or without engagement: not trending")
	require.Nil(t, p.NextCursor)

	// The ticker applies the decay that time alone causes.
	_, err = testdb.Pool.Exec(ctx, `UPDATE videos SET published_at = now() - interval '73 hours' WHERE video_id = $1`, hot)
	require.NoError(t, err)
	n, err := repository.RescoreAll(ctx, testdb.Pool)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, int64(2))
	e.mr.FastForward(61 * time.Second)
	p, err = e.svc.Trending(ctx, "", 10)
	require.NoError(t, err)
	require.Equal(t, []string{warm.String()}, ids(p.Items))

	_, err = e.svc.Trending(ctx, cursor.EncodeTime(cursor.Time{At: time.Now(), VideoID: hot}), 10)
	requireCode(t, err, "invalid_cursor")
}

// seed builds a world with plenty of each source for one viewer.
func seed(t *testing.T, viewer uuid.UUID, followed int) {
	t.Helper()
	now := time.Now()
	for i := range followed {
		a := uuid.New()
		follow(t, viewer, a)
		for j := range 15 {
			video(t, a, now.Add(-time.Duration(i*15+j+1)*time.Minute))
		}
	}
	big := make([]uuid.UUID, 30)
	for i := range big {
		big[i] = uuid.New()
		require.NoError(t, repository.AddFollow(ctx, testdb.Pool, uuid.New(), big[i], now))
		_, err := testdb.Pool.Exec(ctx, `UPDATE author_followers SET followers_count = 5000 WHERE author_id = $1`, big[i])
		require.NoError(t, err)
		for j := range 5 {
			engage(t, video(t, big[i], now.Add(-time.Duration(j+1)*time.Hour)), int64(100+j))
		}
	}
	for range 40 {
		video(t, uuid.New(), now.Add(-time.Duration(30)*time.Minute))
	}
	video(t, viewer, now)
}

func collect(t *testing.T, svc *service.Feed, viewer uuid.UUID) []dto.Item {
	t.Helper()
	var all []dto.Item
	cur := ""
	for {
		p, err := svc.ForYou(ctx, viewer, cur, 30)
		require.NoError(t, err)
		all = append(all, p.Items...)
		if p.NextCursor == nil {
			return all
		}
		cur = *p.NextCursor
	}
}

func classify(t *testing.T, viewer uuid.UUID, items []dto.Item) (following, trendingCount, discovery int) {
	t.Helper()
	for _, it := range items {
		var followed bool
		require.NoError(t, testdb.Pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM follows WHERE follower_id = $1 AND following_id = $2)`,
			viewer, uuid.MustParse(it.Author.ID)).Scan(&followed))
		switch {
		case followed:
			following++
		case it.Stats.Likes > 0:
			trendingCount++
		default:
			discovery++
		}
	}
	return following, trendingCount, discovery
}

func TestForYouMixAndConstraints(t *testing.T) {
	e := newEnv(t)
	viewer := uuid.New()
	seed(t, viewer, 10)

	items := collect(t, e.svc, viewer)
	require.Len(t, items, model.SessionSize)
	f, tr, d := classify(t, viewer, items)
	require.Equal(t, 120, f, "60% following")
	require.Equal(t, 60, tr, "30% trending")
	require.Equal(t, 20, d, "10% discovery")

	seen := map[string]bool{}
	for i, it := range items {
		require.False(t, seen[it.VideoID], "duplicate")
		seen[it.VideoID] = true
		require.NotEqual(t, viewer.String(), it.Author.ID, "own video")
		if i >= 2 {
			require.False(t, it.Author.ID == items[i-1].Author.ID && it.Author.ID == items[i-2].Author.ID,
				"three in a row from one author at %d", i)
		}
	}
}

func TestForYouColdStart(t *testing.T) {
	e := newEnv(t)
	viewer := uuid.New()
	seed(t, viewer, 0) // 150 trending and 40 discovery candidates, nothing followed

	items := collect(t, e.svc, viewer)
	require.Len(t, items, 190, "fewer candidates than a session: every one is used once")
	f, tr, d := classify(t, viewer, items)
	require.Equal(t, [3]int{0, 150, 40}, [3]int{f, tr, d})

	f, tr, d = classify(t, viewer, items[:10])
	require.Equal(t, [3]int{0, 7, 3}, [3]int{f, tr, d}, "70% trending and 30% discovery")
}

func TestForYouSessionPagingAndExpiry(t *testing.T) {
	e := newEnv(t)
	viewer := uuid.New()
	seed(t, viewer, 2)

	p1, err := e.svc.ForYou(ctx, viewer, "", 10)
	require.NoError(t, err)
	require.Len(t, p1.Items, 10)
	p2, err := e.svc.ForYou(ctx, viewer, *p1.NextCursor, 10)
	require.NoError(t, err)
	require.NotEqual(t, ids(p1.Items), ids(p2.Items))

	again, err := e.svc.ForYou(ctx, viewer, *p1.NextCursor, 10)
	require.NoError(t, err)
	require.Equal(t, ids(p2.Items), ids(again.Items), "a cursor replays the same page")

	// A video deleted after the session was built disappears from the next page.
	pos, err := cursor.DecodeSession(*p2.NextCursor)
	require.NoError(t, err)
	p3, err := e.svc.ForYou(ctx, viewer, *p2.NextCursor, 10)
	require.NoError(t, err)
	victim := uuid.MustParse(p3.Items[0].VideoID)
	require.NoError(t, repository.DeleteVideo(ctx, testdb.Pool, victim, uuid.MustParse(p3.Items[0].Author.ID), time.Now()))
	p3, err = e.svc.ForYou(ctx, viewer, cursor.EncodeSession(*pos), 10)
	require.NoError(t, err)
	require.NotContains(t, ids(p3.Items), victim.String())

	other := uuid.New()
	fresh, err := e.svc.ForYou(ctx, other, *p1.NextCursor, 10)
	require.NoError(t, err, "someone else's cursor starts a new session")
	require.NotNil(t, fresh.NextCursor)

	e.mr.FastForward(31 * time.Minute)
	restarted, err := e.svc.ForYou(ctx, viewer, *p2.NextCursor, 10)
	require.NoError(t, err, "an expired session restarts without error")
	require.Len(t, restarted.Items, 10)
	next, err := cursor.DecodeSession(*restarted.NextCursor)
	require.NoError(t, err)
	require.Equal(t, 10, next.Offset, "the new session starts at its first page")

	_, err = e.svc.ForYou(ctx, viewer, "nope", 10)
	requireCode(t, err, "invalid_cursor")
}

func TestForYouWithNothingToShow(t *testing.T) {
	e := newEnv(t)
	p, err := e.svc.ForYou(ctx, uuid.New(), "", 10)
	require.NoError(t, err)
	require.Empty(t, p.Items)
	require.Nil(t, p.NextCursor)
}
