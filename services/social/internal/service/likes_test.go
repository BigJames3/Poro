package service

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/events"

	"github.com/poro/social/internal/repository"
	"github.com/poro/social/internal/testdb"
)

func TestLikeIsIdempotentAndPublishesOnce(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, owner := seedVideo(t)
	user := uuid.Must(uuid.NewV7())

	stats, err := svc.LikeVideo(ctx, user, video)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.Likes)
	require.True(t, stats.LikedByMe)

	stats, err = svc.LikeVideo(ctx, user, video)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.Likes, "second like changes nothing")

	created := outboxed(t, events.TypeSocialLikeCreated, video.String())
	require.Len(t, created, 1)
	data := decodeData[events.SocialLikeCreatedV1](t, created[0])
	require.Equal(t, user.String(), data.UserID)
	require.Equal(t, owner.String(), data.VideoOwnerID)
	require.Equal(t, "social", created[0].Source)

	exists, err := repository.UserExists(ctx, testdb.Pool, user)
	require.NoError(t, err)
	require.True(t, exists, "a caller with a valid token becomes a known account")

	for range 2 {
		stats, err = svc.UnlikeVideo(ctx, user, video)
		require.NoError(t, err)
		require.Equal(t, int64(0), stats.Likes)
		require.False(t, stats.LikedByMe)
	}
	deleted := outboxed(t, events.TypeSocialLikeDeleted, video.String())
	require.Len(t, deleted, 1)
	require.Equal(t, owner.String(), decodeData[events.SocialLikeDeletedV1](t, deleted[0]).VideoOwnerID)
}

func TestLikeRequiresReadyVideo(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	user := uuid.Must(uuid.NewV7())

	unknown := uuid.Must(uuid.NewV7())
	_, err := svc.LikeVideo(ctx, user, unknown)
	requireCode(t, err, 404, "video_not_found")

	video, owner := seedVideo(t)
	require.NoError(t, repository.MarkVideoDeleted(ctx, testdb.Pool, video, owner, nowUTC()))
	_, err = svc.LikeVideo(ctx, user, video)
	requireCode(t, err, 404, "video_not_found")
	_, err = svc.UnlikeVideo(ctx, user, video)
	requireCode(t, err, 404, "video_not_found")
	_, err = svc.VideoStats(ctx, video, nil)
	requireCode(t, err, 404, "video_not_found")

	require.Zero(t, count(t, `SELECT count(*) FROM likes WHERE video_id = ANY($1)`, []uuid.UUID{unknown, video}))
	require.Empty(t, outboxed(t, events.TypeSocialLikeCreated, video.String()))
}

func TestLikeCountersUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, _ := seedVideo(t)

	const users = 40
	var wg sync.WaitGroup
	errs := make(chan error, users*2)
	for range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u := uuid.Must(uuid.NewV7())
			_, err := svc.LikeVideo(ctx, u, video)
			errs <- err
			_, err = svc.LikeVideo(ctx, u, video)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	stats, err := svc.VideoStats(ctx, video, nil)
	require.NoError(t, err)
	require.Equal(t, int64(users), stats.Likes)
	require.Equal(t, users, count(t, `SELECT count(*) FROM likes WHERE video_id = $1`, video))
	require.Len(t, outboxed(t, events.TypeSocialLikeCreated, video.String()), users)
}

// One user toggling like and unlike from many goroutines: the counter must
// always equal the rows, and the events must net out to the final state.
func TestLikeToggleRaceKeepsCounterExact(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, _ := seedVideo(t)
	user := uuid.Must(uuid.NewV7())

	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			if i%2 == 0 {
				_, err = svc.LikeVideo(ctx, user, video)
			} else {
				_, err = svc.UnlikeVideo(ctx, user, video)
			}
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	rows := count(t, `SELECT count(*) FROM likes WHERE video_id = $1`, video)
	stats, err := svc.VideoStats(ctx, video, nil)
	require.NoError(t, err)
	require.Equal(t, int64(rows), stats.Likes)
	created := len(outboxed(t, events.TypeSocialLikeCreated, video.String()))
	deleted := len(outboxed(t, events.TypeSocialLikeDeleted, video.String()))
	require.Equal(t, rows, created-deleted)
}

// A failure after the like row is written must leave no like, no counter
// change and no event: the outbox is in the same transaction.
func TestLikeRollsBackWithOutbox(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, _ := seedVideo(t)
	user := uuid.Must(uuid.NewV7())

	_, err := svc.LikeVideo(ctx, user, video)
	require.NoError(t, err)
	// Corrupt the counter so the decrement violates its CHECK constraint.
	_, err = testdb.Pool.Exec(ctx, `UPDATE video_counters SET likes_count = 0 WHERE video_id = $1`, video)
	require.NoError(t, err)

	_, err = svc.UnlikeVideo(ctx, user, video)
	requireCode(t, err, 500, "internal_error")

	require.Equal(t, 1, count(t, `SELECT count(*) FROM likes WHERE video_id = $1 AND user_id = $2`, video, user),
		"the delete rolled back")
	require.Empty(t, outboxed(t, events.TypeSocialLikeDeleted, video.String()), "no event left behind")
}

func TestListLikesPagesWithoutGapOrDuplicate(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, _ := seedVideo(t)
	liker := uuid.Must(uuid.NewV7())
	want := map[string]bool{}
	for range 23 {
		u := uuid.Must(uuid.NewV7())
		_, err := svc.LikeVideo(ctx, u, video)
		require.NoError(t, err)
		want[u.String()] = true
	}
	_, err := svc.LikeVideo(ctx, liker, video)
	require.NoError(t, err)
	want[liker.String()] = true

	seen := map[string]bool{}
	cursor, pages := "", 0
	for {
		p, err := svc.ListVideoLikes(ctx, video, cursor, 10)
		require.NoError(t, err)
		pages++
		for i, item := range p.Items {
			require.False(t, seen[item.UserID], "duplicate %s", item.UserID)
			seen[item.UserID] = true
			if i > 0 {
				require.False(t, item.LikedAt.After(p.Items[i-1].LikedAt), "newest first")
			}
		}
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	require.Equal(t, 3, pages)
	require.Equal(t, want, seen)

	liked, err := svc.ListUserLikes(ctx, liker, "", 0)
	require.NoError(t, err)
	require.Len(t, liked.Items, 1)
	require.Equal(t, video.String(), liked.Items[0].VideoID)

	_, err = svc.ListVideoLikes(ctx, video, "not-a-cursor", 10)
	requireCode(t, err, 400, "invalid_cursor")
	_, err = svc.ListUserLikes(ctx, uuid.Must(uuid.NewV7()), "", 10)
	requireCode(t, err, 404, "user_not_found")
}
