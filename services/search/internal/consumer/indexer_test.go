package consumer_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/search/internal/consumer"
	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

func send(t *testing.T, typ string, data any) {
	t.Helper()
	env, err := events.New(typ, 1, "test", uuid.NewString(), data, time.Now())
	require.NoError(t, err)
	require.NoError(t, consumer.NewIndexer(testdb.Pool, zap.NewNop())(context.Background(), env))
}

func ready(video, author uuid.UUID, title string, tags ...string) events.VideoReadyV1 {
	now := time.Now().UTC()
	return events.VideoReadyV1{VideoID: video.String(), UserID: author.String(), DurationMs: 15000, Width: 720, Height: 1280,
		HLSKey: "h", ThumbnailKey: "videos/a/v/thumb.jpg", ReadyAt: now, Title: title, Hashtags: tags, PublishedAt: now}
}

func videoIDs(t *testing.T, q string) []uuid.UUID {
	t.Helper()
	page, err := index.NewPostgres(testdb.Pool).Videos(context.Background(), q, 0, 50)
	require.NoError(t, err)
	ids := make([]uuid.UUID, 0, len(page.Items))
	for _, h := range page.Items {
		ids = append(ids, h.VideoID)
	}
	return ids
}

func tagCount(t *testing.T, tag string) int64 {
	t.Helper()
	var n int64
	err := testdb.Pool.QueryRow(context.Background(), `SELECT videos_count FROM hashtags WHERE tag = $1`, tag).Scan(&n)
	if err != nil {
		return -1
	}
	return n
}

func TestVideoLifecycleKeepsSearchAndHashtagsInStep(t *testing.T) {
	testdb.Available(t)
	video, author := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	tag := "lifecycle_" + video.String()[:8]
	send(t, events.TypeVideoReady, ready(video, author, "Danse coupé-décalé au marché de Treichville", tag))
	send(t, events.TypeVideoReady, ready(video, author, "Danse coupé-décalé au marché de Treichville", tag))

	require.Contains(t, videoIDs(t, "treichville"), video)
	require.Contains(t, videoIDs(t, "coupe decale"), video, "accents are ignored")
	require.Contains(t, videoIDs(t, "danses"), video, "French stems match plurals")
	require.Contains(t, videoIDs(t, "treichvile"), video, "a typo still finds the title")
	require.EqualValues(t, 1, tagCount(t, tag), "a redelivered ready counts once")

	removed := events.ModerationContentRemovedV1{CaseID: uuid.NewString(), TargetType: events.ModerationTargetVideo,
		TargetID: video.String(), OwnerID: author.String(), Reason: events.ModerationReasonSpam,
		DecidedBy: events.ModerationDecidedByAuto, RemovedAt: time.Now()}
	send(t, events.TypeModerationContentRemoved, removed)
	require.NotContains(t, videoIDs(t, "treichville"), video)
	require.EqualValues(t, 0, tagCount(t, tag))

	send(t, events.TypeModerationContentRestored, events.ModerationContentRestoredV1{CaseID: uuid.NewString(),
		TargetType: events.ModerationTargetVideo, TargetID: video.String(), OwnerID: author.String(), RestoredAt: time.Now()})
	require.Contains(t, videoIDs(t, "treichville"), video)
	require.EqualValues(t, 1, tagCount(t, tag))

	send(t, events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: video.String(), UserID: author.String(), DeletedAt: time.Now()})
	send(t, events.TypeModerationContentRestored, events.ModerationContentRestoredV1{CaseID: uuid.NewString(),
		TargetType: events.ModerationTargetVideo, TargetID: video.String(), OwnerID: author.String(), RestoredAt: time.Now()})
	send(t, events.TypeVideoReady, ready(video, author, "Danse coupé-décalé au marché de Treichville", tag))
	require.NotContains(t, videoIDs(t, "treichville"), video, "a deleted video never comes back")
	require.EqualValues(t, 0, tagCount(t, tag))

	early := uuid.Must(uuid.NewV7())
	removed.TargetID = early.String()
	send(t, events.TypeModerationContentRemoved, removed)
	send(t, events.TypeVideoReady, ready(early, author, "Treichville la nuit"))
	require.NotContains(t, videoIDs(t, "treichville"), early, "removed before ready stays hidden")

	send(t, events.TypeModerationContentRemoved, events.ModerationContentRemovedV1{CaseID: uuid.NewString(),
		TargetType: events.ModerationTargetComment, TargetID: uuid.NewString(), OwnerID: author.String(),
		Reason: events.ModerationReasonSpam, DecidedBy: events.ModerationDecidedByAuto, RemovedAt: time.Now()})
}

func TestEngagementRanksAndCountersClamp(t *testing.T) {
	testdb.Available(t)
	author := uuid.Must(uuid.NewV7())
	quiet, popular := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	word := "zouglou" + quiet.String()[:6]
	send(t, events.TypeVideoReady, ready(quiet, author, "Concert "+word))
	send(t, events.TypeVideoReady, ready(popular, author, "Concert "+word))
	for range 20 {
		send(t, events.TypeSocialLikeCreated, events.SocialLikeCreatedV1{LikeID: uuid.NewString(), UserID: uuid.NewString(),
			VideoID: popular.String(), VideoOwnerID: author.String(), CreatedAt: time.Now()})
	}
	send(t, events.TypeSocialShareCreated, events.SocialShareCreatedV1{ShareID: uuid.NewString(), UserID: uuid.NewString(),
		VideoID: popular.String(), VideoOwnerID: author.String(), Channel: events.ShareChannelWhatsApp, CreatedAt: time.Now()})
	send(t, events.TypeSocialCommentCreated, events.SocialCommentCreatedV1{CommentID: uuid.NewString(), UserID: uuid.NewString(),
		VideoID: popular.String(), VideoOwnerID: author.String(), Excerpt: "x", CreatedAt: time.Now()})
	require.Equal(t, []uuid.UUID{popular, quiet}, videoIDs(t, word))

	for range 3 {
		send(t, events.TypeSocialCommentDeleted, events.SocialCommentDeletedV1{CommentID: uuid.NewString(),
			VideoID: quiet.String(), UserID: uuid.NewString(), DeletedAt: time.Now()})
		send(t, events.TypeSocialLikeDeleted, events.SocialLikeDeletedV1{UserID: uuid.NewString(), VideoID: quiet.String(),
			VideoOwnerID: author.String(), DeletedAt: time.Now()})
	}
	page, err := index.NewPostgres(testdb.Pool).Videos(context.Background(), word, 0, 50)
	require.NoError(t, err)
	require.Zero(t, page.Items[1].Likes, "counters never go below zero")
	require.Zero(t, page.Items[1].Comments)
	require.EqualValues(t, 20, page.Items[0].Likes)

	// A like on a video not indexed yet waits for its ready event.
	late := uuid.Must(uuid.NewV7())
	send(t, events.TypeSocialLikeCreated, events.SocialLikeCreatedV1{LikeID: uuid.NewString(), UserID: uuid.NewString(),
		VideoID: late.String(), VideoOwnerID: author.String(), CreatedAt: time.Now()})
	require.NotContains(t, videoIDs(t, word), late)
	send(t, events.TypeVideoReady, ready(late, author, "Concert "+word))
	page, err = index.NewPostgres(testdb.Pool).Videos(context.Background(), word, 0, 50)
	require.NoError(t, err)
	for _, h := range page.Items {
		if h.VideoID == late {
			require.EqualValues(t, 1, h.Likes)
		}
	}
}

func TestUsersAndFollowers(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	idx := index.NewPostgres(testdb.Pool)
	awa, other := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	handle := "awa_" + awa.String()[:6]
	send(t, events.TypeAuthUserCreated, events.AuthUserCreatedV1{UserID: awa.String(), SignupMethod: "phone", Language: "fr", CreatedAt: time.Now()})
	page, err := idx.Users(ctx, handle, 0, 10)
	require.NoError(t, err)
	require.Empty(t, page.Items, "an account without a username is not listed")

	display := "Awa Koné"
	send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: awa.String(), Username: &handle,
		DisplayName: &display, IsCreator: true, UpdatedAt: time.Now()})
	old := "ancien"
	send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: awa.String(), Username: &old,
		UpdatedAt: time.Now().Add(-time.Hour)})
	otherHandle := handle + "x"
	send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: other.String(), Username: &otherHandle,
		UpdatedAt: time.Now()})
	for range 5 {
		send(t, events.TypeSocialFollowCreated, events.SocialFollowCreatedV1{FollowerID: uuid.NewString(),
			FollowingID: other.String(), CreatedAt: time.Now()})
	}
	send(t, events.TypeSocialFollowDeleted, events.SocialFollowDeletedV1{FollowerID: uuid.NewString(),
		FollowingID: awa.String(), DeletedAt: time.Now()})

	page, err = idx.Users(ctx, "@"+handle, 0, 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.Equal(t, awa, page.Items[0].UserID, "an exact username wins over followers")
	require.Equal(t, handle, page.Items[0].Username, "an older snapshot never overwrites")
	require.Zero(t, page.Items[0].Followers)
	require.EqualValues(t, 5, page.Items[1].Followers)

	byName, err := idx.Users(ctx, "kone", 0, 10)
	require.NoError(t, err)
	require.Equal(t, awa, byName.Items[0].UserID, "display names match without accents")
}

func TestBadEventsArePermanent(t *testing.T) {
	h := consumer.NewIndexer(nil, zap.NewNop())
	mk := func(typ string, data any) events.Envelope {
		env, err := events.New(typ, 1, "test", uuid.NewString(), data, time.Now())
		require.NoError(t, err)
		return env
	}
	withData := func(env events.Envelope, raw string) events.Envelope {
		env.Data = json.RawMessage(raw)
		return env
	}
	v2 := mk(events.TypeVideoReady, ready(uuid.New(), uuid.New(), "t"))
	v2.Version = 2
	noTime := ready(uuid.New(), uuid.New(), "t")
	noTime.PublishedAt, noTime.ReadyAt = time.Time{}, time.Time{}
	cases := map[string]events.Envelope{
		"version 2":          v2,
		"no publish time":    mk(events.TypeVideoReady, noTime),
		"ready wrong types":  withData(mk(events.TypeVideoReady, struct{}{}), `{"video_id": 1}`),
		"bad deleted id":     mk(events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: "x", UserID: uuid.NewString()}),
		"deleted bad json":   withData(mk(events.TypeVideoDeleted, struct{}{}), `{"video_id": 1}`),
		"removed user":       mk(events.TypeModerationContentRemoved, events.ModerationContentRemovedV1{TargetType: "user", TargetID: uuid.NewString(), OwnerID: uuid.NewString()}),
		"removed bad id":     mk(events.TypeModerationContentRemoved, events.ModerationContentRemovedV1{TargetType: "video", TargetID: "x", OwnerID: uuid.NewString()}),
		"moderation json":    withData(mk(events.TypeModerationContentRestored, struct{}{}), `{"target_id": 1}`),
		"user created bad":   mk(events.TypeAuthUserCreated, events.AuthUserCreatedV1{UserID: uuid.Nil.String()}),
		"user created json":  withData(mk(events.TypeAuthUserCreated, struct{}{}), `{"user_id": 1}`),
		"profile no time":    mk(events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: uuid.NewString()}),
		"profile bad id":     mk(events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: "x", UpdatedAt: time.Now()}),
		"profile json":       withData(mk(events.TypeUserProfileUpdated, struct{}{}), `{"user_id": 1}`),
		"follow bad id":      mk(events.TypeSocialFollowCreated, events.SocialFollowCreatedV1{FollowingID: "x"}),
		"follow json":        withData(mk(events.TypeSocialFollowDeleted, struct{}{}), `{"following_id": 1}`),
		"like without video": mk(events.TypeSocialLikeCreated, events.SocialLikeCreatedV1{}),
		"like bad owner":     mk(events.TypeSocialLikeCreated, events.SocialLikeCreatedV1{VideoID: uuid.NewString(), VideoOwnerID: "x"}),
		"like json":          withData(mk(events.TypeSocialLikeCreated, struct{}{}), `{"video_id": 7}`),
		"unknown type":       mk(events.TypeVideoUploaded, events.VideoUploadedV1{}),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			err := h(context.Background(), env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "%v", err)
		})
	}
}
