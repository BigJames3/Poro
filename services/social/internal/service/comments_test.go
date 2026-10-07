package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/events"

	"github.com/poro/social/internal/dto"
	"github.com/poro/social/internal/repository"
	"github.com/poro/social/internal/testdb"
)

func ptr[T any](v T) *T { return &v }

func TestCommentAndReply(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, owner := seedVideo(t)
	awa, kofi := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	top, err := svc.CreateComment(ctx, awa, video, dto.CreateCommentRequest{Content: "  Trop beau 🔥 " + strings.Repeat("x", 200)})
	require.NoError(t, err)
	require.Equal(t, awa.String(), *top.UserID)
	require.Nil(t, top.ParentID)
	require.False(t, top.Edited)

	reply, err := svc.CreateComment(ctx, kofi, video, dto.CreateCommentRequest{Content: "Merci", ParentID: &top.ID})
	require.NoError(t, err)
	require.Equal(t, top.ID, *reply.ParentID)

	_, err = svc.CreateComment(ctx, awa, video, dto.CreateCommentRequest{Content: "deep", ParentID: &reply.ID})
	requireCode(t, err, 422, "comment_reply_depth")

	other, _ := seedVideo(t)
	_, err = svc.CreateComment(ctx, awa, other, dto.CreateCommentRequest{Content: "wrong video", ParentID: &top.ID})
	requireCode(t, err, 422, "parent_invalid")
	_, err = svc.CreateComment(ctx, awa, video, dto.CreateCommentRequest{Content: "x", ParentID: ptr("nope")})
	requireCode(t, err, 422, "parent_invalid")
	_, err = svc.CreateComment(ctx, awa, video, dto.CreateCommentRequest{Content: "x", ParentID: ptr(uuid.NewString())})
	requireCode(t, err, 404, "comment_not_found")

	stats, err := svc.VideoStats(ctx, video, nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), stats.Comments)

	created := outboxed(t, events.TypeSocialCommentCreated, video.String())
	require.Len(t, created, 2)
	first := decodeData[events.SocialCommentCreatedV1](t, created[0])
	require.Nil(t, first.ParentID)
	require.Nil(t, first.ParentAuthorID)
	require.Equal(t, owner.String(), first.VideoOwnerID)
	require.Len(t, []rune(first.Excerpt), 140)
	require.True(t, strings.HasPrefix(first.Excerpt, "Trop beau 🔥"))
	second := decodeData[events.SocialCommentCreatedV1](t, created[1])
	require.Equal(t, top.ID, *second.ParentID)
	require.Equal(t, awa.String(), *second.ParentAuthorID)

	page, err := svc.ListComments(ctx, video, &kofi, "", 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, 1, page.Items[0].RepliesCount)

	replies, err := svc.ListReplies(ctx, uuid.MustParse(top.ID), nil, "", 10)
	require.NoError(t, err)
	require.Len(t, replies.Items, 1)
	require.Equal(t, "Merci", *replies.Items[0].Content)

	_, err = svc.ListReplies(ctx, uuid.MustParse(reply.ID), nil, "", 10)
	requireCode(t, err, 404, "comment_not_found")
}

func TestCommentValidation(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, _ := seedVideo(t)
	user := uuid.Must(uuid.NewV7())
	for _, content := range []string{"", "   ", strings.Repeat("a", 1001), "a\u202eb"} {
		_, err := svc.CreateComment(ctx, user, video, dto.CreateCommentRequest{Content: content})
		requireCode(t, err, 422, "comment_invalid")
	}
	_, err := svc.CreateComment(ctx, user, uuid.Must(uuid.NewV7()), dto.CreateCommentRequest{Content: "hello"})
	requireCode(t, err, 404, "video_not_found")
	require.Zero(t, count(t, `SELECT count(*) FROM comments WHERE user_id = $1`, user))
}

func TestEditCommentWindowAndAuthor(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, _ := seedVideo(t)
	author, stranger := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	c, err := svc.CreateComment(ctx, author, video, dto.CreateCommentRequest{Content: "premier jet"})
	require.NoError(t, err)
	id := uuid.MustParse(c.ID)

	_, err = svc.EditComment(ctx, stranger, id, dto.EditCommentRequest{Content: "hack"})
	requireCode(t, err, 403, "forbidden")
	_, err = svc.EditComment(ctx, author, id, dto.EditCommentRequest{Content: ""})
	requireCode(t, err, 422, "comment_invalid")

	start := time.Now()
	svc.now = func() time.Time { return start.Add(time.Minute) }
	edited, err := svc.EditComment(ctx, author, id, dto.EditCommentRequest{Content: "version finale"})
	require.NoError(t, err)
	require.Equal(t, "version finale", *edited.Content)
	require.True(t, edited.Edited)

	svc.now = func() time.Time { return start.Add(16 * time.Minute) }
	_, err = svc.EditComment(ctx, author, id, dto.EditCommentRequest{Content: "trop tard"})
	requireCode(t, err, 403, "comment_edit_window_closed")

	_, err = svc.EditComment(ctx, author, uuid.Must(uuid.NewV7()), dto.EditCommentRequest{Content: "x"})
	requireCode(t, err, 404, "comment_not_found")
}

func TestDeleteCommentRightsCountersAndThread(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, owner := seedVideo(t)
	author, replier, stranger := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	top, err := svc.CreateComment(ctx, author, video, dto.CreateCommentRequest{Content: "top"})
	require.NoError(t, err)
	reply, err := svc.CreateComment(ctx, replier, video, dto.CreateCommentRequest{Content: "reply", ParentID: &top.ID})
	require.NoError(t, err)
	lonely, err := svc.CreateComment(ctx, author, video, dto.CreateCommentRequest{Content: "lonely"})
	require.NoError(t, err)

	err = svc.DeleteComment(ctx, stranger, uuid.MustParse(top.ID))
	requireCode(t, err, 403, "forbidden")

	require.NoError(t, svc.DeleteComment(ctx, owner, uuid.MustParse(top.ID)), "video owner may delete")
	require.NoError(t, svc.DeleteComment(ctx, author, uuid.MustParse(top.ID)), "deleting twice is accepted")
	require.NoError(t, svc.DeleteComment(ctx, author, uuid.MustParse(lonely.ID)))
	err = svc.DeleteComment(ctx, stranger, uuid.MustParse(lonely.ID))
	requireCode(t, err, 404, "comment_not_found")
	err = svc.DeleteComment(ctx, author, uuid.Must(uuid.NewV7()))
	requireCode(t, err, 404, "comment_not_found")

	deleted := outboxed(t, events.TypeSocialCommentDeleted, video.String())
	require.Len(t, deleted, 2, "one event per state change")
	require.Equal(t, author.String(), decodeData[events.SocialCommentDeletedV1](t, deleted[0]).UserID)

	stats, err := svc.VideoStats(ctx, video, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.Comments, "only the reply is left")

	page, err := svc.ListComments(ctx, video, nil, "", 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1, "a deleted comment without replies disappears")
	require.True(t, page.Items[0].Deleted)
	require.Nil(t, page.Items[0].Content)
	require.Nil(t, page.Items[0].UserID)

	require.NoError(t, svc.DeleteComment(ctx, replier, uuid.MustParse(reply.ID)))
	page, err = svc.ListComments(ctx, video, nil, "", 10)
	require.NoError(t, err)
	require.Empty(t, page.Items, "the deleted parent goes once its last reply is gone")
	stats, err = svc.VideoStats(ctx, video, nil)
	require.NoError(t, err)
	require.Zero(t, stats.Comments)

	_, err = svc.CreateComment(ctx, replier, video, dto.CreateCommentRequest{Content: "late", ParentID: &top.ID})
	requireCode(t, err, 404, "comment_not_found")
}

func TestCommentLikes(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, _ := seedVideo(t)
	author, fan := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	c, err := svc.CreateComment(ctx, author, video, dto.CreateCommentRequest{Content: "like me"})
	require.NoError(t, err)
	id := uuid.MustParse(c.ID)

	for range 2 {
		state, err := svc.LikeComment(ctx, fan, id)
		require.NoError(t, err)
		require.Equal(t, 1, state.LikesCount)
		require.True(t, state.LikedByMe)
	}
	page, err := svc.ListComments(ctx, video, &fan, "", 10)
	require.NoError(t, err)
	require.True(t, page.Items[0].LikedByMe)
	require.Equal(t, 1, page.Items[0].LikesCount)

	for range 2 {
		state, err := svc.UnlikeComment(ctx, fan, id)
		require.NoError(t, err)
		require.Zero(t, state.LikesCount)
		require.False(t, state.LikedByMe)
	}

	require.NoError(t, svc.DeleteComment(ctx, author, id))
	_, err = svc.LikeComment(ctx, fan, id)
	requireCode(t, err, 404, "comment_not_found")
}

func TestCommentPagination(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	video, _ := seedVideo(t)
	user := uuid.Must(uuid.NewV7())
	top, err := svc.CreateComment(ctx, user, video, dto.CreateCommentRequest{Content: "root"})
	require.NoError(t, err)
	base := time.Now()
	for i := range 7 {
		svc.now = func() time.Time { return base.Add(time.Duration(i) * time.Second) }
		_, err := svc.CreateComment(ctx, user, video, dto.CreateCommentRequest{Content: "r", ParentID: &top.ID})
		require.NoError(t, err)
	}
	var all []dto.Comment
	cursor := ""
	for {
		p, err := svc.ListReplies(ctx, uuid.MustParse(top.ID), nil, cursor, 3)
		require.NoError(t, err)
		all = append(all, p.Items...)
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	require.Len(t, all, 7)
	for i := 1; i < len(all); i++ {
		require.True(t, all[i].CreatedAt.After(all[i-1].CreatedAt), "replies oldest first")
	}

	deletedVideo, owner := seedVideo(t)
	c, err := svc.CreateComment(ctx, user, deletedVideo, dto.CreateCommentRequest{Content: "x"})
	require.NoError(t, err)
	require.NoError(t, repository.MarkVideoDeleted(ctx, testdb.Pool, deletedVideo, owner, time.Now()))
	_, err = svc.ListComments(ctx, deletedVideo, nil, "", 10)
	requireCode(t, err, 404, "video_not_found")
	_, err = svc.ListReplies(ctx, uuid.MustParse(c.ID), nil, "", 10)
	requireCode(t, err, 404, "video_not_found")
}
