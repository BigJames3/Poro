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

	"github.com/poro/social/internal/consumer"
	"github.com/poro/social/internal/repository"
	"github.com/poro/social/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

func envelope(t *testing.T, typ string, data any) events.Envelope {
	t.Helper()
	env, err := events.New(typ, 1, "test", uuid.Must(uuid.NewV7()).String(), data, time.Now())
	require.NoError(t, err)
	return env
}

func status(t *testing.T, video uuid.UUID) string {
	t.Helper()
	var s string
	err := testdb.Pool.QueryRow(context.Background(), `SELECT status FROM videos_projection WHERE video_id = $1`, video).Scan(&s)
	if err != nil {
		return ""
	}
	return s
}

func ready(video, owner uuid.UUID) events.VideoReadyV1 {
	now := time.Now().UTC()
	return events.VideoReadyV1{VideoID: video.String(), UserID: owner.String(), DurationMs: 1, Width: 1, Height: 1,
		HLSKey: "h", ThumbnailKey: "t", ReadyAt: now, Title: "t", PublishedAt: now}
}

func TestReadyThenDeleted(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	h := consumer.NewProjections(testdb.Pool, zap.NewNop())
	video, owner := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	readyEnv := envelope(t, events.TypeVideoReady, ready(video, owner))
	require.NoError(t, h(ctx, readyEnv))
	require.NoError(t, h(ctx, readyEnv), "a redelivered event is a no-op")
	require.Equal(t, "ready", status(t, video))
	known, err := repository.UserExists(ctx, testdb.Pool, owner)
	require.NoError(t, err)
	require.True(t, known, "a video owner is a known account")

	require.NoError(t, h(ctx, envelope(t, events.TypeVideoDeleted, events.VideoDeletedV1{
		VideoID: video.String(), UserID: owner.String(), DeletedAt: time.Now(),
	})))
	require.Equal(t, "deleted", status(t, video))

	require.NoError(t, h(ctx, envelope(t, events.TypeVideoReady, ready(video, owner))))
	require.Equal(t, "deleted", status(t, video), "a late ready never revives a deleted video")
}

func TestDeletedBeforeReady(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	h := consumer.NewProjections(testdb.Pool, zap.NewNop())
	video, owner := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	require.NoError(t, h(ctx, envelope(t, events.TypeVideoDeleted, events.VideoDeletedV1{
		VideoID: video.String(), UserID: owner.String(), DeletedAt: time.Now(),
	})))
	require.NoError(t, h(ctx, envelope(t, events.TypeVideoReady, ready(video, owner))))
	require.Equal(t, "deleted", status(t, video))
}

func TestReadyWithoutPublishedAtUsesReadyAt(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	h := consumer.NewProjections(testdb.Pool, zap.NewNop())
	video, owner := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	legacy := ready(video, owner)
	legacy.PublishedAt, legacy.Title = time.Time{}, ""
	require.NoError(t, h(ctx, envelope(t, events.TypeVideoReady, legacy)))
	require.Equal(t, "ready", status(t, video))
}

func TestUserCreated(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	h := consumer.NewProjections(testdb.Pool, zap.NewNop())
	user := uuid.Must(uuid.NewV7())
	require.NoError(t, h(ctx, envelope(t, events.TypeAuthUserCreated, events.AuthUserCreatedV1{
		UserID: user.String(), SignupMethod: "phone", Language: "fr", CreatedAt: time.Now(),
	})))
	known, err := repository.UserExists(ctx, testdb.Pool, user)
	require.NoError(t, err)
	require.True(t, known)
}

func TestBadEventsArePermanent(t *testing.T) {
	ctx := context.Background()
	h := consumer.NewProjections(nil, zap.NewNop())
	owner := uuid.Must(uuid.NewV7())

	v2 := envelope(t, events.TypeVideoReady, ready(uuid.Must(uuid.NewV7()), owner))
	v2.Version = 2
	badVideo := envelope(t, events.TypeVideoReady, ready(uuid.Must(uuid.NewV7()), owner))
	badVideo.Data = json.RawMessage(`{"video_id":"nope","user_id":"` + owner.String() + `"}`)
	badUser := envelope(t, events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: uuid.NewString(), UserID: "nope"})
	nilIDs := envelope(t, events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: uuid.Nil.String(), UserID: owner.String()})
	badCreated := envelope(t, events.TypeAuthUserCreated, events.AuthUserCreatedV1{UserID: "nope"})
	notJSON := envelope(t, events.TypeAuthUserCreated, events.AuthUserCreatedV1{UserID: owner.String()})
	notJSON.Data = json.RawMessage(`{"user_id": 42}`)
	unknown := envelope(t, events.TypeSocialLikeCreated, events.SocialLikeCreatedV1{})

	for name, env := range map[string]events.Envelope{
		"version 2": v2, "bad video id": badVideo, "bad user id": badUser, "nil ids": nilIDs,
		"bad created user": badCreated, "wrong types": notJSON, "unknown type": unknown,
	} {
		t.Run(name, func(t *testing.T) {
			err := h(ctx, env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "bad data goes to the DLQ, it is not retried: %v", err)
		})
	}
}
