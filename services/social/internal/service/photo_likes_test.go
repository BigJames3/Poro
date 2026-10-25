package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/social/internal/repository"
	"github.com/poro/social/internal/testdb"
)

// ADR-0020 § 5: liked photo posts are listed only to a client that declared
// photo posts; a cursor of such a list is refused without the capability.
func TestUserLikesHidePhotoPostsWithoutTheCapability(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	liker := newUser(t)
	video, _ := seedVideo(t)
	owner := newUser(t)
	photo := uuid.Must(uuid.NewV7())
	require.NoError(t, repository.MarkVideoReady(ctx, testdb.Pool, photo, owner, "photo", time.Now()))
	_, err := svc.LikeVideo(ctx, liker, video)
	require.NoError(t, err)
	_, err = svc.LikeVideo(ctx, liker, photo)
	require.NoError(t, err, "a photo post is liked like a video")

	videosOnly, err := svc.ListUserLikes(ctx, liker, false, "", 10)
	require.NoError(t, err)
	require.Len(t, videosOnly.Items, 1)
	require.Equal(t, video.String(), videosOnly.Items[0].VideoID)

	all, err := svc.ListUserLikes(ctx, liker, true, "", 1)
	require.NoError(t, err)
	require.Equal(t, photo.String(), all.Items[0].VideoID, "newest like first")
	require.NotNil(t, all.NextCursor)
	_, err = svc.ListUserLikes(ctx, liker, false, *all.NextCursor, 1)
	requireCode(t, err, 400, "invalid_cursor")
	rest, err := svc.ListUserLikes(ctx, liker, true, *all.NextCursor, 1)
	require.NoError(t, err)
	require.Equal(t, video.String(), rest.Items[0].VideoID)
}
