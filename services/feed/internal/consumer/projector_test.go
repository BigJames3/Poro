package consumer_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/feed/internal/consumer"
	"github.com/poro/feed/internal/testdb"
	"github.com/poro/feed/internal/trending"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

func send(t *testing.T, typ string, data any) error {
	t.Helper()
	env, err := events.New(typ, 1, "test", uuid.Must(uuid.NewV7()).String(), data, time.Now())
	require.NoError(t, err)
	return consumer.NewProjector(testdb.Pool, zap.NewNop())(context.Background(), env)
}

func ready(video, author uuid.UUID, published time.Time) events.VideoReadyV1 {
	return events.VideoReadyV1{VideoID: video.String(), UserID: author.String(), DurationMs: 15000, Width: 720, Height: 1280,
		HLSKey: "videos/a/v/hls/master.m3u8", ThumbnailKey: "videos/a/v/thumb.jpg", ReadyAt: published,
		Title: "Danse", Hashtags: []string{"abidjan"}, PublishedAt: published}
}

func row[T any](t *testing.T, sql string, args ...any) T {
	t.Helper()
	var v T
	require.NoError(t, testdb.Pool.QueryRow(context.Background(), sql, args...).Scan(&v))
	return v
}

func TestVideoLifecycleIsIdempotentAndNeverRevives(t *testing.T) {
	testdb.Available(t)
	video, author := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	env, err := events.New(events.TypeVideoReady, 1, "media-worker", video.String(), ready(video, author, time.Now()), time.Now())
	require.NoError(t, err)
	h := consumer.NewProjector(testdb.Pool, zap.NewNop())
	require.NoError(t, h(context.Background(), env))
	require.NoError(t, h(context.Background(), env), "redelivery is a no-op")
	require.Equal(t, "Danse", row[string](t, `SELECT title FROM videos WHERE video_id = $1`, video))
	require.Equal(t, []string{"abidjan"}, row[[]string](t, `SELECT hashtags FROM videos WHERE video_id = $1`, video))

	require.NoError(t, send(t, events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: video.String(), UserID: author.String(), DeletedAt: time.Now()}))
	require.NoError(t, send(t, events.TypeVideoReady, ready(video, author, time.Now())))
	require.True(t, row[bool](t, `SELECT deleted_at IS NOT NULL FROM videos WHERE video_id = $1`, video), "a late ready never revives")

	early := uuid.Must(uuid.NewV7())
	require.NoError(t, send(t, events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: early.String(), UserID: author.String(), DeletedAt: time.Now()}))
	require.NoError(t, send(t, events.TypeVideoReady, ready(early, author, time.Now())))
	require.True(t, row[bool](t, `SELECT deleted_at IS NOT NULL FROM videos WHERE video_id = $1`, early), "tombstone wins")
}

func TestModerationHidesAndRestoresVideos(t *testing.T) {
	testdb.Available(t)
	video, author := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	status := func(id uuid.UUID) string {
		return row[string](t, `SELECT moderation_status FROM videos WHERE video_id = $1`, id)
	}
	removed := func(id uuid.UUID, target string) events.ModerationContentRemovedV1 {
		return events.ModerationContentRemovedV1{CaseID: uuid.NewString(), TargetType: target, TargetID: id.String(),
			OwnerID: author.String(), Reason: events.ModerationReasonSpam, DecidedBy: events.ModerationDecidedByAuto, RemovedAt: time.Now()}
	}
	restored := func(id uuid.UUID) events.ModerationContentRestoredV1 {
		return events.ModerationContentRestoredV1{CaseID: uuid.NewString(), TargetType: events.ModerationTargetVideo,
			TargetID: id.String(), OwnerID: author.String(), RestoredAt: time.Now()}
	}

	require.NoError(t, send(t, events.TypeVideoReady, ready(video, author, time.Now())))
	require.NoError(t, send(t, events.TypeSocialLikeCreated, events.SocialLikeCreatedV1{
		LikeID: uuid.NewString(), UserID: uuid.NewString(), VideoID: video.String(), VideoOwnerID: author.String(), CreatedAt: time.Now(),
	}))
	require.Positive(t, row[float64](t, `SELECT trending_score FROM video_stats WHERE video_id = $1`, video))

	require.NoError(t, send(t, events.TypeModerationContentRemoved, removed(video, events.ModerationTargetVideo)))
	require.Equal(t, "rejected", status(video))
	require.Zero(t, row[float64](t, `SELECT trending_score FROM video_stats WHERE video_id = $1`, video))
	require.NoError(t, send(t, events.TypeVideoReady, ready(video, author, time.Now())))
	require.Equal(t, "rejected", status(video), "a late ready keeps the video hidden")

	require.NoError(t, send(t, events.TypeModerationContentRestored, restored(video)))
	require.Equal(t, "approved", status(video))
	require.Positive(t, row[float64](t, `SELECT trending_score FROM video_stats WHERE video_id = $1`, video))
	require.NoError(t, send(t, events.TypeModerationContentRestored, restored(video)), "restoring twice is a no-op")

	early := uuid.Must(uuid.NewV7())
	require.NoError(t, send(t, events.TypeModerationContentRemoved, removed(early, events.ModerationTargetVideo)))
	require.NoError(t, send(t, events.TypeVideoReady, ready(early, author, time.Now())))
	require.Equal(t, "rejected", status(early), "removed before ready stays hidden")
	require.Equal(t, "Danse", row[string](t, `SELECT title FROM videos WHERE video_id = $1`, early))

	comment := uuid.Must(uuid.NewV7())
	require.NoError(t, send(t, events.TypeModerationContentRemoved, removed(comment, events.ModerationTargetComment)))
	require.False(t, row[bool](t, `SELECT EXISTS (SELECT 1 FROM videos WHERE video_id = $1)`, comment),
		"comments are handled through social.comment.deleted")
}

func TestLegacyReadyWithoutPublishedAt(t *testing.T) {
	testdb.Available(t)
	video, author := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	at := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	legacy := ready(video, author, at)
	legacy.PublishedAt, legacy.Title, legacy.Hashtags = time.Time{}, "", nil
	require.NoError(t, send(t, events.TypeVideoReady, legacy))
	require.True(t, at.Equal(row[time.Time](t, `SELECT published_at FROM videos WHERE video_id = $1`, video)))
	require.Empty(t, row[[]string](t, `SELECT hashtags FROM videos WHERE video_id = $1`, video))
}

func TestFollowsAndFollowerCounts(t *testing.T) {
	testdb.Available(t)
	a, b, star := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	for _, f := range []uuid.UUID{a, a, b} {
		require.NoError(t, send(t, events.TypeSocialFollowCreated, events.SocialFollowCreatedV1{
			FollowerID: f.String(), FollowingID: star.String(), CreatedAt: time.Now()}))
	}
	require.Equal(t, int64(2), row[int64](t, `SELECT followers_count FROM author_followers WHERE author_id = $1`, star),
		"the same follow twice counts once")
	for range 2 {
		require.NoError(t, send(t, events.TypeSocialFollowDeleted, events.SocialFollowDeletedV1{
			FollowerID: a.String(), FollowingID: star.String(), DeletedAt: time.Now()}))
	}
	require.Equal(t, int64(1), row[int64](t, `SELECT followers_count FROM author_followers WHERE author_id = $1`, star))
	require.Equal(t, 1, row[int](t, `SELECT count(*) FROM follows WHERE following_id = $1`, star))
}

func TestStatsScoreAndClampAtZero(t *testing.T) {
	testdb.Available(t)
	video, author, user := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	vid, owner := video.String(), author.String()

	// Engagement can arrive before the video: counted, scored once it is ready.
	require.NoError(t, send(t, events.TypeSocialLikeCreated, events.SocialLikeCreatedV1{
		LikeID: uuid.NewString(), UserID: user.String(), VideoID: vid, VideoOwnerID: owner, CreatedAt: time.Now()}))
	require.Zero(t, row[float64](t, `SELECT trending_score FROM video_stats WHERE video_id = $1`, video))

	published := time.Now().Add(-2 * time.Hour)
	require.NoError(t, send(t, events.TypeVideoReady, ready(video, author, published)))
	require.NoError(t, send(t, events.TypeSocialCommentCreated, events.SocialCommentCreatedV1{
		CommentID: uuid.NewString(), UserID: user.String(), VideoID: vid, VideoOwnerID: owner, Excerpt: "x", CreatedAt: time.Now()}))
	require.NoError(t, send(t, events.TypeSocialShareCreated, events.SocialShareCreatedV1{
		ShareID: uuid.NewString(), UserID: user.String(), VideoID: vid, VideoOwnerID: owner, Channel: "whatsapp", CreatedAt: time.Now()}))

	score := row[float64](t, `SELECT trending_score FROM video_stats WHERE video_id = $1`, video)
	want := trending.Score(1, 1, 1, 2*time.Hour)
	require.InDelta(t, want, score, want*0.001, "SQL and Go compute the same score")

	require.NoError(t, send(t, events.TypeSocialLikeDeleted, events.SocialLikeDeletedV1{
		UserID: user.String(), VideoID: vid, VideoOwnerID: owner, DeletedAt: time.Now()}))
	require.NoError(t, send(t, events.TypeSocialLikeDeleted, events.SocialLikeDeletedV1{
		UserID: user.String(), VideoID: vid, VideoOwnerID: owner, DeletedAt: time.Now()}))
	require.NoError(t, send(t, events.TypeSocialCommentDeleted, events.SocialCommentDeletedV1{
		CommentID: uuid.NewString(), VideoID: vid, UserID: user.String(), DeletedAt: time.Now()}))
	require.Equal(t, int64(0), row[int64](t, `SELECT likes_count FROM video_stats WHERE video_id = $1`, video),
		"a missed like.created never drives the counter negative or blocks the consumer")
	require.Equal(t, int64(1), row[int64](t, `SELECT shares_count FROM video_stats WHERE video_id = $1`, video))
	require.False(t, math.IsNaN(row[float64](t, `SELECT trending_score FROM video_stats WHERE video_id = $1`, video)))
}

func TestProfileKeepsLatestSnapshot(t *testing.T) {
	testdb.Available(t)
	author := uuid.Must(uuid.NewV7())
	name := func(s string) *string { return &s }
	now := time.Now()
	require.NoError(t, send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{
		UserID: author.String(), Username: name("awa.new"), UpdatedAt: now}))
	require.NoError(t, send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{
		UserID: author.String(), Username: name("awa.old"), UpdatedAt: now.Add(-time.Hour)}))
	require.Equal(t, "awa.new", row[string](t, `SELECT username FROM authors WHERE author_id = $1`, author))
}

func withData(env events.Envelope, raw string) events.Envelope {
	env.Data = json.RawMessage(raw)
	return env
}

func TestBadEventsArePermanent(t *testing.T) {
	h := consumer.NewProjector(nil, zap.NewNop())
	mk := func(typ string, data any) events.Envelope {
		env, err := events.New(typ, 1, "test", uuid.NewString(), data, time.Now())
		require.NoError(t, err)
		return env
	}
	v2 := mk(events.TypeVideoReady, ready(uuid.New(), uuid.New(), time.Now()))
	v2.Version = 2
	noTime := ready(uuid.New(), uuid.New(), time.Time{})
	badJSON := mk(events.TypeSocialLikeCreated, events.SocialLikeCreatedV1{})
	badJSON.Data = json.RawMessage(`{"video_id": 7}`)
	cases := map[string]events.Envelope{
		"version 2":        v2,
		"no publish time":  mk(events.TypeVideoReady, noTime),
		"bad video id":     mk(events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: "x", UserID: uuid.NewString()}),
		"nil follower":     mk(events.TypeSocialFollowCreated, events.SocialFollowCreatedV1{FollowerID: uuid.Nil.String(), FollowingID: uuid.NewString()}),
		"bad unfollow":     mk(events.TypeSocialFollowDeleted, events.SocialFollowDeletedV1{FollowerID: "x", FollowingID: uuid.NewString()}),
		"like without id":  mk(events.TypeSocialLikeCreated, events.SocialLikeCreatedV1{}),
		"like wrong types": badJSON,
		"profile no time":  mk(events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: uuid.NewString()}),
		"profile bad id":   mk(events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: "x", UpdatedAt: time.Now()}),
		"unknown type":     mk(events.TypeAuthUserCreated, events.AuthUserCreatedV1{UserID: uuid.NewString()}),
		"removed user": mk(events.TypeModerationContentRemoved, events.ModerationContentRemovedV1{
			TargetType: "user", TargetID: uuid.NewString(), OwnerID: uuid.NewString()}),
		"removed bad id": mk(events.TypeModerationContentRemoved, events.ModerationContentRemovedV1{
			TargetType: events.ModerationTargetVideo, TargetID: "x", OwnerID: uuid.NewString()}),
		"restored comment": mk(events.TypeModerationContentRestored, events.ModerationContentRestoredV1{
			TargetType: events.ModerationTargetComment, TargetID: uuid.NewString(), OwnerID: uuid.NewString()}),
		"restored bad id": mk(events.TypeModerationContentRestored, events.ModerationContentRestoredV1{
			TargetType: events.ModerationTargetVideo, TargetID: "x", OwnerID: uuid.NewString()}),
		"removed wrong types":  withData(mk(events.TypeModerationContentRemoved, events.ModerationContentRemovedV1{}), `{"target_id": 1}`),
		"restored wrong types": withData(mk(events.TypeModerationContentRestored, events.ModerationContentRestoredV1{}), `{"target_id": 1}`),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			err := h(context.Background(), env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "%v", err)
		})
	}
}
