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

func removed(target, owner uuid.UUID, targetType string) events.ModerationContentRemovedV1 {
	return events.ModerationContentRemovedV1{
		CaseID: uuid.NewString(), TargetType: targetType, TargetID: target.String(), OwnerID: owner.String(),
		Reason: events.ModerationReasonNudity, DecidedBy: events.ModerationDecidedByModerator, RemovedAt: time.Now(),
	}
}

func restored(target, owner uuid.UUID) events.ModerationContentRestoredV1 {
	return events.ModerationContentRestoredV1{
		CaseID: uuid.NewString(), TargetType: events.ModerationTargetVideo, TargetID: target.String(),
		OwnerID: owner.String(), RestoredAt: time.Now(),
	}
}

func TestModerationRemovesAndRestoresVideos(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	h := consumer.NewProjections(testdb.Pool, zap.NewNop())
	video, owner := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	require.NoError(t, h(ctx, envelope(t, events.TypeVideoReady, ready(video, owner))))
	require.NoError(t, h(ctx, envelope(t, events.TypeModerationContentRemoved, removed(video, owner, events.ModerationTargetVideo))))
	require.Equal(t, "removed", status(t, video))
	require.NoError(t, h(ctx, envelope(t, events.TypeVideoReady, ready(video, owner))))
	require.Equal(t, "removed", status(t, video), "a late ready never republishes a removed video")
	require.NoError(t, h(ctx, envelope(t, events.TypeModerationContentRestored, restored(video, owner))))
	require.Equal(t, "ready", status(t, video))

	require.NoError(t, h(ctx, envelope(t, events.TypeVideoDeleted, events.VideoDeletedV1{
		VideoID: video.String(), UserID: owner.String(), DeletedAt: time.Now(),
	})))
	require.NoError(t, h(ctx, envelope(t, events.TypeModerationContentRemoved, removed(video, owner, events.ModerationTargetVideo))))
	require.NoError(t, h(ctx, envelope(t, events.TypeModerationContentRestored, restored(video, owner))))
	require.Equal(t, "deleted", status(t, video), "moderation never revives a deleted video")

	early := uuid.Must(uuid.NewV7())
	require.NoError(t, h(ctx, envelope(t, events.TypeModerationContentRemoved, removed(early, owner, events.ModerationTargetVideo))))
	require.NoError(t, h(ctx, envelope(t, events.TypeVideoReady, ready(early, owner))))
	require.Equal(t, "removed", status(t, early), "removed before ready stays removed")
}

func TestModerationRemovesComments(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	h := consumer.NewProjections(testdb.Pool, zap.NewNop())
	video, owner, author := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	require.NoError(t, h(ctx, envelope(t, events.TypeVideoReady, ready(video, owner))))
	comment := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	require.NoError(t, repository.InsertComment(ctx, testdb.Pool, repository.Comment{
		ID: comment, UserID: author, VideoID: video, Content: "insulte", CreatedAt: now, UpdatedAt: now,
	}))

	require.NoError(t, h(ctx, envelope(t, events.TypeModerationContentRemoved, removed(comment, author, events.ModerationTargetComment))))
	var deleted bool
	require.NoError(t, testdb.Pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM comments WHERE id = $1`, comment).Scan(&deleted))
	require.True(t, deleted)
	var published int
	require.NoError(t, testdb.Pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox_events WHERE topic = $1 AND payload->'data'->>'comment_id' = $2`,
		events.TypeSocialCommentDeleted, comment.String()).Scan(&published))
	require.Equal(t, 1, published, "consumers learn about the removal through comment.deleted")
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
	owner2 := uuid.Must(uuid.NewV7())
	badTarget := removed(uuid.Must(uuid.NewV7()), owner2, "user")
	badRemovedID := removed(uuid.Must(uuid.NewV7()), owner2, events.ModerationTargetVideo)
	badRemovedID.TargetID = "nope"
	badOwner := removed(uuid.Must(uuid.NewV7()), owner2, events.ModerationTargetVideo)
	badOwner.OwnerID = "nope"
	restoredComment := restored(uuid.Must(uuid.NewV7()), owner2)
	restoredComment.TargetType = events.ModerationTargetComment
	badRestoredID := restored(uuid.Must(uuid.NewV7()), owner2)
	badRestoredID.TargetID = "nope"
	notJSONRemoved := envelope(t, events.TypeModerationContentRemoved, removed(uuid.Must(uuid.NewV7()), owner2, events.ModerationTargetVideo))
	notJSONRemoved.Data = json.RawMessage(`{"target_id": 1}`)
	notJSONRestored := envelope(t, events.TypeModerationContentRestored, restored(uuid.Must(uuid.NewV7()), owner2))
	notJSONRestored.Data = json.RawMessage(`{"target_id": 1}`)

	for name, env := range map[string]events.Envelope{
		"version 2": v2, "bad video id": badVideo, "bad user id": badUser, "nil ids": nilIDs,
		"bad created user": badCreated, "wrong types": notJSON, "unknown type": unknown,
		"removed user target":    envelope(t, events.TypeModerationContentRemoved, badTarget),
		"removed bad target id":  envelope(t, events.TypeModerationContentRemoved, badRemovedID),
		"removed bad owner":      envelope(t, events.TypeModerationContentRemoved, badOwner),
		"restored comment":       envelope(t, events.TypeModerationContentRestored, restoredComment),
		"restored bad target id": envelope(t, events.TypeModerationContentRestored, badRestoredID),
		"removed wrong types":    notJSONRemoved,
		"restored wrong types":   notJSONRestored,
	} {
		t.Run(name, func(t *testing.T) {
			err := h(ctx, env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "bad data goes to the DLQ, it is not retried: %v", err)
		})
	}
}
