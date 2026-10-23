package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/hiding"

	"github.com/poro/social/internal/dto"
	"github.com/poro/social/internal/testdb"
)

// While its deletion is pending, an account's videos, comments and follows
// leave the public reads and take no new action; removals still work; and
// everything comes back when the deletion is cancelled.
func TestHiddenAccount(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, owner := seedVideo(t)
	fan := newUser(t)
	other, _ := seedVideo(t)

	_, err := svc.LikeVideo(ctx, fan, video)
	require.NoError(t, err)
	_, err = svc.Follow(ctx, fan, owner)
	require.NoError(t, err)
	_, err = svc.CreateComment(ctx, owner, other, dto.CreateCommentRequest{Content: "merci"})
	require.NoError(t, err)

	setHidden := func(hidden bool) {
		t.Helper()
		_, err := testdb.Pool.Exec(ctx, hiding.Upsert, owner, hidden, time.Now())
		require.NoError(t, err)
	}
	setHidden(true)

	_, err = svc.VideoStats(ctx, video, nil)
	require.ErrorIs(t, err, errVideoNotFound)
	_, err = svc.LikeVideo(ctx, newUser(t), video)
	require.ErrorIs(t, err, errVideoNotFound)
	_, err = svc.Follow(ctx, newUser(t), owner)
	require.ErrorIs(t, err, errUserNotFound)
	stats, err := svc.VideoStatsBatch(ctx, []uuid.UUID{video, other}, nil)
	require.NoError(t, err)
	require.Len(t, stats.Items, 1, "only the other video")
	comments, err := svc.ListComments(ctx, other, nil, "", 20)
	require.NoError(t, err)
	require.Empty(t, comments.Items, "the hidden account's comment is left out")

	_, err = svc.UnlikeVideo(ctx, fan, video)
	require.NoError(t, err, "a fan can still take back a like")
	_, err = svc.Unfollow(ctx, fan, owner)
	require.NoError(t, err, "and unfollow")

	setHidden(false)
	_, err = svc.VideoStats(ctx, video, nil)
	require.NoError(t, err)
	comments, err = svc.ListComments(ctx, other, nil, "", 20)
	require.NoError(t, err)
	require.Len(t, comments.Items, 1)
}
