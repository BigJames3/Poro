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

	"github.com/poro/feed/internal/consumer"
	"github.com/poro/feed/internal/testdb"
)

func erasure(t *testing.T, user uuid.UUID) events.Envelope {
	t.Helper()
	env, err := events.New(events.TypeAuthUserDeleted, 1, "auth", user.String(),
		events.AuthUserDeletedV1{UserID: user.String(), DeletedAt: time.Now()}, time.Now())
	require.NoError(t, err)
	return env
}

func follow(t *testing.T, follower, following uuid.UUID) {
	t.Helper()
	require.NoError(t, send(t, events.TypeSocialFollowCreated, events.SocialFollowCreatedV1{
		FollowerID: follower.String(), FollowingID: following.String(), CreatedAt: time.Now()}))
}

func followers(t *testing.T, author uuid.UUID) int64 {
	t.Helper()
	return row[int64](t, `SELECT COALESCE((SELECT followers_count FROM author_followers WHERE author_id = $1), -1)`, author)
}

func TestErasureForgetsTheAccountOnce(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	user, other := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	fan, star, small := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	name := func(s string) *string { return &s }
	for id, username := range map[uuid.UUID]string{user: "awa", other: "koffi"} {
		require.NoError(t, send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{
			UserID: id.String(), Username: name(username), DisplayName: name(username), AvatarURL: name("https://cdn/a.jpg"), UpdatedAt: time.Now()}))
	}
	video := uuid.Must(uuid.NewV7())
	require.NoError(t, send(t, events.TypeVideoReady, ready(video, user, time.Now())))
	follow(t, fan, user)
	follow(t, other, user)
	follow(t, user, star)
	follow(t, other, star)
	follow(t, user, small)
	follow(t, other, fan)
	require.Equal(t, int64(2), followers(t, user))
	require.Equal(t, int64(2), followers(t, star))
	require.Equal(t, int64(1), followers(t, small))

	h := consumer.NewEraser(testdb.Pool, zap.NewNop())
	env := erasure(t, user)
	require.NoError(t, h(ctx, env))

	require.Zero(t, row[int](t, `SELECT count(*) FROM authors WHERE author_id = $1`, user), "author profile deleted")
	require.Zero(t, row[int](t, `SELECT count(*) FROM follows WHERE follower_id = $1 OR following_id = $1`, user), "follows deleted both ways")
	require.Equal(t, int64(-1), followers(t, user), "follower count deleted")
	require.Equal(t, int64(1), followers(t, star), "followed accounts lose one follower")
	require.Equal(t, int64(0), followers(t, small))
	require.Equal(t, 1, row[int](t, `SELECT count(*) FROM erased_users WHERE user_id = $1`, user), "only the opaque id is kept")
	require.Equal(t, 1, row[int](t, `SELECT count(*) FROM videos WHERE video_id = $1 AND deleted_at IS NULL`, video),
		"videos leave through poro.video.deleted, not here")

	// Other accounts are untouched.
	require.Equal(t, "koffi", row[string](t, `SELECT username FROM authors WHERE author_id = $1`, other))
	require.Equal(t, 2, row[int](t, `SELECT count(*) FROM follows WHERE follower_id = $1`, other))
	require.Equal(t, int64(1), followers(t, fan))

	// Replay and a second erasure event change nothing more.
	again := erasure(t, user)
	require.NoError(t, h(ctx, env))
	require.NoError(t, h(ctx, again))
	require.Equal(t, 2, row[int](t, `SELECT count(*) FROM processed_events WHERE consumer = $1 AND event_id = ANY($2)`,
		consumer.ErasureGroup, []uuid.UUID{env.ID, again.ID}))
	require.Equal(t, int64(1), followers(t, star), "never decremented twice")
	require.Equal(t, int64(1), followers(t, fan))

	// The follow.deleted events social publishes afterwards are no-ops.
	require.NoError(t, send(t, events.TypeSocialFollowDeleted, events.SocialFollowDeletedV1{
		FollowerID: user.String(), FollowingID: star.String(), DeletedAt: time.Now()}))
	require.Equal(t, int64(1), followers(t, star))

	// Late events never project the account again.
	require.NoError(t, send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{
		UserID: user.String(), Username: name("awa"), UpdatedAt: time.Now().Add(time.Hour)}))
	require.Zero(t, row[int](t, `SELECT count(*) FROM authors WHERE author_id = $1`, user), "a late profile never recreates the author")
	follow(t, user, star)
	follow(t, fan, user)
	require.Zero(t, row[int](t, `SELECT count(*) FROM follows WHERE follower_id = $1 OR following_id = $1`, user), "a late follow is not projected")
	require.Equal(t, int64(1), followers(t, star))
	require.Equal(t, int64(-1), followers(t, user))
}

func TestErasureOfAnUnknownAccountIsHarmless(t *testing.T) {
	testdb.Available(t)
	user := uuid.Must(uuid.NewV7())
	require.NoError(t, consumer.NewEraser(testdb.Pool, zap.NewNop())(context.Background(), erasure(t, user)))
	require.Equal(t, 1, row[int](t, `SELECT count(*) FROM erased_users WHERE user_id = $1`, user))
}

func TestBadErasureEventsArePermanent(t *testing.T) {
	h := consumer.NewEraser(nil, zap.NewNop())
	v2 := erasure(t, uuid.New())
	v2.Version = 2
	wrongType := erasure(t, uuid.New())
	wrongType.Type = events.TypeAuthUserCreated
	mk := func(raw string) events.Envelope {
		env := erasure(t, uuid.New())
		env.Data = json.RawMessage(raw)
		return env
	}
	cases := map[string]events.Envelope{
		"version 2":   v2,
		"wrong type":  wrongType,
		"bad user id": mk(`{"user_id": "x", "deleted_at": "2026-10-15T00:00:00Z"}`),
		"nil user id": mk(`{"user_id": "` + uuid.Nil.String() + `"}`),
		"no user id":  mk(`{}`),
		"wrong types": mk(`{"user_id": 7}`),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			err := h(context.Background(), env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "%v", err)
		})
	}
}
