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

func TestFollowRules(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	awa, kofi := newUser(t), uuid.Must(uuid.NewV7())

	_, err := svc.Follow(ctx, awa, awa)
	requireCode(t, err, 400, "cannot_follow_self")
	_, err = svc.Unfollow(ctx, awa, awa)
	requireCode(t, err, 400, "cannot_follow_self")
	_, err = svc.Follow(ctx, awa, uuid.Must(uuid.NewV7()))
	requireCode(t, err, 404, "user_not_found")

	for range 2 {
		stats, err := svc.Follow(ctx, kofi, awa)
		require.NoError(t, err)
		require.Equal(t, int64(1), stats.Followers)
		require.True(t, stats.FollowedByMe)
	}
	created := outboxed(t, events.TypeSocialFollowCreated, kofi.String())
	require.Len(t, created, 1)
	data := decodeData[events.SocialFollowCreatedV1](t, created[0])
	require.Equal(t, awa.String(), data.FollowingID)

	mine, err := svc.UserStats(ctx, kofi, &kofi)
	require.NoError(t, err)
	require.Equal(t, int64(1), mine.Following, "the follower became a known account and gained a following")
	require.False(t, mine.FollowedByMe, "never followed by yourself")

	followers, err := svc.ListFollowers(ctx, awa, "", 10)
	require.NoError(t, err)
	require.Len(t, followers.Items, 1)
	require.Equal(t, kofi.String(), followers.Items[0].UserID)
	following, err := svc.ListFollowing(ctx, kofi, "", 10)
	require.NoError(t, err)
	require.Equal(t, awa.String(), following.Items[0].UserID)

	for range 2 {
		stats, err := svc.Unfollow(ctx, kofi, awa)
		require.NoError(t, err)
		require.Zero(t, stats.Followers)
		require.False(t, stats.FollowedByMe)
	}
	require.Len(t, outboxed(t, events.TypeSocialFollowDeleted, kofi.String()), 1)

	_, err = svc.ListFollowers(ctx, uuid.Must(uuid.NewV7()), "", 10)
	requireCode(t, err, 404, "user_not_found")
	_, err = svc.ListFollowing(ctx, awa, "bad", 10)
	requireCode(t, err, 400, "invalid_cursor")
}

// Pairs following each other at the same time update the same two counter
// rows in opposite directions; ordered writes keep them from deadlocking.
func TestMutualFollowsDoNotDeadlock(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	const pairs = 15
	users := make([][2]uuid.UUID, pairs)
	for i := range users {
		users[i] = [2]uuid.UUID{newUser(t), newUser(t)}
	}
	var wg sync.WaitGroup
	for _, p := range users {
		for _, dir := range [][2]uuid.UUID{{p[0], p[1]}, {p[1], p[0]}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := svc.Follow(ctx, dir[0], dir[1])
				require.NoError(t, err)
			}()
		}
	}
	wg.Wait()
	for _, p := range users {
		for _, u := range p {
			c, err := repository.GetUserCounters(ctx, testdb.Pool, u)
			require.NoError(t, err)
			require.Equal(t, repository.UserCounters{Followers: 1, Following: 1}, c)
		}
	}
}

func TestShareCountsEveryShare(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, owner := seedVideo(t)
	user := uuid.Must(uuid.NewV7())

	_, err := svc.Share(ctx, user, video, shareReq("telegram"))
	requireCode(t, err, 422, "channel_invalid")
	_, err = svc.Share(ctx, user, uuid.Must(uuid.NewV7()), shareReq("whatsapp"))
	requireCode(t, err, 404, "video_not_found")

	for i, ch := range []string{"whatsapp", "copy_link", "whatsapp"} {
		res, err := svc.Share(ctx, user, video, shareReq(ch))
		require.NoError(t, err)
		require.Equal(t, int64(i+1), res.SharesCount)
		require.Equal(t, ch, res.Channel)
	}
	shares := outboxed(t, events.TypeSocialShareCreated, video.String())
	require.Len(t, shares, 3)
	data := decodeData[events.SocialShareCreatedV1](t, shares[0])
	require.Equal(t, owner.String(), data.VideoOwnerID)
	require.Equal(t, "whatsapp", data.Channel)
}
