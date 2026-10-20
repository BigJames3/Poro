package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/social/internal/consumer"
	"github.com/poro/social/internal/dto"
	"github.com/poro/social/internal/repository"
	"github.com/poro/social/internal/service"
	"github.com/poro/social/internal/testdb"
)

// world is an erased account (gone) and the accounts around it.
type world struct {
	gone, alice, bob    uuid.UUID
	aliceVideo, goneVid uuid.UUID
	goneTop, bobReply   uuid.UUID // gone's comment on aliceVideo and bob's reply to it
	aliceTop, goneReply uuid.UUID // alice's comment on aliceVideo and gone's reply to it
	goneOnOwnVideo      uuid.UUID // gone's comment on its own video
}

func seedWorld(t *testing.T) world {
	t.Helper()
	testdb.Available(t)
	ctx := context.Background()
	proj := consumer.NewProjections(testdb.Pool, zap.NewNop())
	svc := service.New(testdb.Pool, zap.NewNop())
	w := world{
		gone: uuid.Must(uuid.NewV7()), alice: uuid.Must(uuid.NewV7()), bob: uuid.Must(uuid.NewV7()),
		aliceVideo: uuid.Must(uuid.NewV7()), goneVid: uuid.Must(uuid.NewV7()),
	}
	for _, u := range []uuid.UUID{w.gone, w.alice, w.bob} {
		require.NoError(t, proj(ctx, envelope(t, events.TypeAuthUserCreated, events.AuthUserCreatedV1{
			UserID: u.String(), SignupMethod: "phone", Language: "fr", CreatedAt: time.Now(),
		})))
	}
	require.NoError(t, proj(ctx, envelope(t, events.TypeVideoReady, ready(w.aliceVideo, w.alice))))
	require.NoError(t, proj(ctx, envelope(t, events.TypeVideoReady, ready(w.goneVid, w.gone))))

	for _, u := range []uuid.UUID{w.gone, w.bob} {
		_, err := svc.LikeVideo(ctx, u, w.aliceVideo)
		require.NoError(t, err)
	}
	_, err := svc.LikeVideo(ctx, w.alice, w.goneVid)
	require.NoError(t, err)
	for _, u := range []uuid.UUID{w.gone, w.gone, w.bob} {
		_, err := svc.Share(ctx, u, w.aliceVideo, dto.ShareRequest{Channel: events.ShareChannelWhatsApp})
		require.NoError(t, err)
	}

	comment := func(user, video uuid.UUID, parent *uuid.UUID, content string) uuid.UUID {
		req := dto.CreateCommentRequest{Content: content}
		if parent != nil {
			p := parent.String()
			req.ParentID = &p
		}
		c, err := svc.CreateComment(ctx, user, video, req)
		require.NoError(t, err)
		return uuid.MustParse(c.ID)
	}
	w.goneTop = comment(w.gone, w.aliceVideo, nil, "secret of gone")
	w.bobReply = comment(w.bob, w.aliceVideo, &w.goneTop, "bob answers")
	w.aliceTop = comment(w.alice, w.aliceVideo, nil, "alice speaks")
	w.goneReply = comment(w.gone, w.aliceVideo, &w.aliceTop, "gone answers")
	w.goneOnOwnVideo = comment(w.gone, w.goneVid, nil, "on my video")
	_, err = svc.LikeComment(ctx, w.gone, w.aliceTop)
	require.NoError(t, err)
	_, err = svc.LikeComment(ctx, w.bob, w.goneTop)
	require.NoError(t, err)

	for _, f := range [][2]uuid.UUID{{w.gone, w.alice}, {w.bob, w.gone}, {w.gone, w.bob}, {w.bob, w.alice}} {
		_, err := svc.Follow(ctx, f[0], f[1])
		require.NoError(t, err)
	}
	return w
}

func userDeleted(t *testing.T, user string) events.Envelope {
	t.Helper()
	return envelope(t, events.TypeAuthUserDeleted, events.AuthUserDeletedV1{UserID: user, DeletedAt: time.Now()})
}

func count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, testdb.Pool.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

// published counts the events of topic whose data field equals user.
func published(t *testing.T, topic, field string, user uuid.UUID) int {
	t.Helper()
	return count(t, `SELECT count(*) FROM outbox_events WHERE topic = $1 AND payload->'data'->>$2 = $3`,
		topic, field, user.String())
}

type snapshot struct {
	aliceVideo, goneVideo                  repository.VideoCounters
	alice, bob                             repository.UserCounters
	goneTop, aliceTop                      *repository.Comment
	likeDeleted, followDeleted, comDeleted int
}

func snap(t *testing.T, w world) snapshot {
	t.Helper()
	ctx := context.Background()
	var s snapshot
	var err error
	s.aliceVideo, err = repository.GetVideoCounters(ctx, testdb.Pool, w.aliceVideo)
	require.NoError(t, err)
	s.goneVideo, err = repository.GetVideoCounters(ctx, testdb.Pool, w.goneVid)
	require.NoError(t, err)
	s.alice, err = repository.GetUserCounters(ctx, testdb.Pool, w.alice)
	require.NoError(t, err)
	s.bob, err = repository.GetUserCounters(ctx, testdb.Pool, w.bob)
	require.NoError(t, err)
	s.goneTop, err = repository.GetComment(ctx, testdb.Pool, w.goneTop, uuid.Nil)
	require.NoError(t, err)
	s.aliceTop, err = repository.GetComment(ctx, testdb.Pool, w.aliceTop, uuid.Nil)
	require.NoError(t, err)
	s.likeDeleted = published(t, events.TypeSocialLikeDeleted, "user_id", w.gone)
	s.followDeleted = published(t, events.TypeSocialFollowDeleted, "follower_id", w.gone) +
		published(t, events.TypeSocialFollowDeleted, "following_id", w.gone)
	s.comDeleted = published(t, events.TypeSocialCommentDeleted, "user_id", w.gone)
	return s
}

func TestAccountErasure(t *testing.T) {
	ctx := context.Background()
	w := seedWorld(t)
	before := snap(t, w)
	require.Equal(t, repository.VideoCounters{Likes: 2, Comments: 4, Shares: 3}, before.aliceVideo)
	require.Equal(t, repository.UserCounters{Followers: 2, Following: 0}, before.alice)
	require.Equal(t, repository.UserCounters{Followers: 1, Following: 2}, before.bob)

	h := consumer.NewErasure(testdb.Pool, zap.NewNop())
	env := userDeleted(t, w.gone.String())
	require.NoError(t, h(ctx, env))
	after := snap(t, w)

	// Nothing of the erased account remains but its comments without text.
	for table, query := range map[string]string{
		"likes":            `SELECT count(*) FROM likes WHERE user_id = $1`,
		"shares":           `SELECT count(*) FROM shares WHERE user_id = $1`,
		"comment_likes":    `SELECT count(*) FROM comment_likes WHERE user_id = $1`,
		"follows":          `SELECT count(*) FROM follows WHERE follower_id = $1 OR following_id = $1`,
		"users_projection": `SELECT count(*) FROM users_projection WHERE user_id = $1`,
		"user_counters":    `SELECT count(*) FROM user_counters WHERE user_id = $1`,
		"live comments":    `SELECT count(*) FROM comments WHERE user_id = $1 AND (deleted_at IS NULL OR content <> '')`,
	} {
		require.Zero(t, count(t, query, w.gone), table)
	}
	require.Equal(t, 3, count(t, `SELECT count(*) FROM comments WHERE user_id = $1`, w.gone), "comment rows are kept")
	require.Equal(t, 1, count(t, `SELECT count(*) FROM erased_users WHERE user_id = $1`, w.gone))

	// Every counter of the others drops exactly by what the account did.
	require.Equal(t, repository.VideoCounters{Likes: 1, Comments: 2, Shares: 1}, after.aliceVideo)
	require.Equal(t, repository.VideoCounters{Likes: 1, Comments: 0, Shares: 0}, after.goneVideo)
	require.Equal(t, repository.UserCounters{Followers: 1, Following: 0}, after.alice)
	require.Equal(t, repository.UserCounters{Followers: 0, Following: 1}, after.bob)
	require.Equal(t, 0, after.aliceTop.LikesCount)
	require.Equal(t, 0, after.aliceTop.RepliesCount)
	require.Equal(t, 1, after.goneTop.LikesCount, "bob's like of the erased comment is bob's")
	require.Equal(t, 1, after.goneTop.RepliesCount, "bob's reply keeps its thread")
	require.NotNil(t, after.goneTop.DeletedAt)
	require.Empty(t, after.goneTop.Content)

	// Others' content is untouched.
	reply, err := repository.GetComment(ctx, testdb.Pool, w.bobReply, uuid.Nil)
	require.NoError(t, err)
	require.Nil(t, reply.DeletedAt)
	require.Equal(t, "bob answers", reply.Content)
	require.Equal(t, "alice speaks", after.aliceTop.Content)
	require.Equal(t, 1, count(t, `SELECT count(*) FROM likes WHERE user_id = $1 AND video_id = $2`, w.alice, w.goneVid))
	require.Equal(t, 1, count(t, `SELECT count(*) FROM follows WHERE follower_id = $1 AND following_id = $2`, w.bob, w.alice))

	// One *.deleted per removed like, follow and comment.
	require.Equal(t, before.likeDeleted+1, after.likeDeleted)
	require.Equal(t, before.followDeleted+3, after.followDeleted)
	require.Equal(t, before.comDeleted+3, after.comDeleted)

	// A replay, a second event and late creation events change nothing.
	proj := consumer.NewProjections(testdb.Pool, zap.NewNop())
	require.NoError(t, h(ctx, env))
	require.NoError(t, h(ctx, userDeleted(t, w.gone.String())))
	require.NoError(t, proj(ctx, envelope(t, events.TypeAuthUserCreated, events.AuthUserCreatedV1{
		UserID: w.gone.String(), SignupMethod: "phone", Language: "fr", CreatedAt: time.Now(),
	})))
	require.NoError(t, proj(ctx, envelope(t, events.TypeVideoReady, ready(uuid.Must(uuid.NewV7()), w.gone))))
	require.Equal(t, after, snap(t, w))
	known, err := repository.UserExists(ctx, testdb.Pool, w.gone)
	require.NoError(t, err)
	require.False(t, known, "an erased account is never projected again")
	require.Equal(t, 1, count(t, `SELECT count(*) FROM processed_events WHERE consumer = $1 AND event_id = $2`,
		consumer.ErasureGroup, env.ID))
}

func TestErasureOfAnUnknownAccount(t *testing.T) {
	testdb.Available(t)
	user := uuid.Must(uuid.NewV7())
	require.NoError(t, consumer.NewErasure(testdb.Pool, zap.NewNop())(context.Background(), userDeleted(t, user.String())))
	require.Equal(t, 1, count(t, `SELECT count(*) FROM erased_users WHERE user_id = $1`, user))
}

func TestErasureRejectsBadEvents(t *testing.T) {
	testdb.Available(t)
	h := consumer.NewErasure(testdb.Pool, zap.NewNop())
	v2 := userDeleted(t, uuid.NewString())
	v2.Version = 2
	wrongType := userDeleted(t, uuid.NewString())
	wrongType.Type = events.TypeAuthUserCreated
	notJSON := userDeleted(t, uuid.NewString())
	notJSON.Data = json.RawMessage(`{"user_id":1}`)
	for name, env := range map[string]events.Envelope{
		"version 2":   v2,
		"wrong type":  wrongType,
		"not json":    notJSON,
		"bad user id": userDeleted(t, "nope"),
		"nil user id": userDeleted(t, uuid.Nil.String()),
	} {
		t.Run(name, func(t *testing.T) {
			err := h(context.Background(), env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "%v", err)
			require.Zero(t, count(t, `SELECT count(*) FROM processed_events WHERE consumer = $1 AND event_id = $2`,
				consumer.ErasureGroup, env.ID))
		})
	}
}
