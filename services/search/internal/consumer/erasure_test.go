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

	"github.com/poro/search/internal/consumer"
	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/testdb"
)

func erasure(t *testing.T, user uuid.UUID) events.Envelope {
	t.Helper()
	env, err := events.New(events.TypeAuthUserDeleted, 1, "auth", user.String(),
		events.AuthUserDeletedV1{UserID: user.String(), DeletedAt: time.Now()}, time.Now())
	require.NoError(t, err)
	return env
}

func count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, testdb.Pool.QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

func TestErasureDeindexesTheAccountForGood(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	idx := index.NewPostgres(testdb.Pool)
	user, other := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	handle := "erase_" + user.String()[:6]
	display, avatar := "Awa Traoré", "https://cdn/a.jpg"
	otherHandle := handle + "x"
	send(t, events.TypeAuthUserCreated, events.AuthUserCreatedV1{UserID: user.String(), SignupMethod: "phone", Language: "fr", CreatedAt: time.Now()})
	send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: user.String(), Username: &handle,
		DisplayName: &display, AvatarURL: &avatar, IsCreator: true, UpdatedAt: time.Now()})
	send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: other.String(), Username: &otherHandle, UpdatedAt: time.Now()})
	for _, following := range []uuid.UUID{user, other} {
		send(t, events.TypeSocialFollowCreated, events.SocialFollowCreatedV1{FollowerID: uuid.NewString(),
			FollowingID: following.String(), CreatedAt: time.Now()})
	}
	video := uuid.Must(uuid.NewV7())
	word := "balafon" + video.String()[:6]
	send(t, events.TypeVideoReady, ready(video, user, "Solo de "+word, word))
	page, err := idx.Users(ctx, handle, 0, 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)

	h := consumer.NewEraser(testdb.Pool, zap.NewNop())
	env := erasure(t, user)
	require.NoError(t, h(ctx, env))

	require.Zero(t, count(t, `SELECT count(*) FROM users WHERE user_id = $1`, user), "the account row is deleted")
	require.Equal(t, 1, count(t, `SELECT count(*) FROM erased_users WHERE user_id = $1`, user), "only the opaque id is kept")
	page, err = idx.Users(ctx, handle, 0, 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, other, page.Items[0].UserID, "other accounts stay searchable")
	require.EqualValues(t, 1, page.Items[0].Followers)
	require.Equal(t, 1, count(t, `SELECT count(*) FROM videos WHERE video_id = $1 AND deleted_at IS NULL`, video),
		"videos leave through poro.video.deleted, not here")

	// Replay and a second erasure event are harmless.
	again := erasure(t, user)
	require.NoError(t, h(ctx, env))
	require.NoError(t, h(ctx, again))
	require.Equal(t, 2, count(t, `SELECT count(*) FROM processed_events WHERE consumer = $1 AND event_id = ANY($2)`,
		consumer.ErasureGroup, []uuid.UUID{env.ID, again.ID}))

	// Late events never index the account again.
	send(t, events.TypeAuthUserCreated, events.AuthUserCreatedV1{UserID: user.String(), SignupMethod: "phone", Language: "fr", CreatedAt: time.Now()})
	send(t, events.TypeUserProfileUpdated, events.UserProfileUpdatedV1{UserID: user.String(), Username: &handle,
		DisplayName: &display, UpdatedAt: time.Now().Add(time.Hour)})
	send(t, events.TypeSocialFollowCreated, events.SocialFollowCreatedV1{FollowerID: uuid.NewString(),
		FollowingID: user.String(), CreatedAt: time.Now()})
	send(t, events.TypeSocialFollowDeleted, events.SocialFollowDeletedV1{FollowerID: uuid.NewString(),
		FollowingID: user.String(), DeletedAt: time.Now()})
	require.Zero(t, count(t, `SELECT count(*) FROM users WHERE user_id = $1`, user), "never recreated")

	send(t, events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: video.String(), UserID: user.String(), DeletedAt: time.Now()})
	require.Empty(t, videoIDs(t, word), "the video leaves with poro.video.deleted")
}

func TestErasureOfAnUnknownAccountIsHarmless(t *testing.T) {
	testdb.Available(t)
	user := uuid.Must(uuid.NewV7())
	require.NoError(t, consumer.NewEraser(testdb.Pool, zap.NewNop())(context.Background(), erasure(t, user)))
	require.Equal(t, 1, count(t, `SELECT count(*) FROM erased_users WHERE user_id = $1`, user))
	require.Zero(t, count(t, `SELECT count(*) FROM users WHERE user_id = $1`, user))
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
