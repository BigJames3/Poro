package moderation_test

import (
	"bytes"
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

	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/moderation"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

type fixture struct {
	h     *moderation.Handler
	mem   *storage.Memory
	repo  *repository.Videos
	user  uuid.UUID
	video uuid.UUID
	thumb string
}

func setup(t *testing.T) fixture {
	t.Helper()
	testdb.Available(t)
	ctx := context.Background()
	user, video := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	repo := repository.New(testdb.Pool)
	require.NoError(t, repo.Create(ctx, &model.Video{
		ID: video.String(), UserID: user.String(), Title: "t", Status: model.StatusReady, ContentType: "video/mp4",
		SizeBytes: 1, SourceKey: model.SourceKey(user.String(), video.String(), "mp4"), CreatedAt: now, UpdatedAt: now,
	}))
	mem := storage.NewMemory()
	thumb := model.ThumbKey(user.String(), video.String())
	for _, key := range []string{thumb, model.HLSDir(user.String(), video.String()) + "/master.m3u8"} {
		require.NoError(t, mem.Put(ctx, key, "x", bytes.NewReader([]byte("x"))))
	}
	return fixture{moderation.New(testdb.Pool, repo, mem, zap.NewNop()), mem, repo, user, video, thumb}
}

func envelope(t *testing.T, typ string, data any) events.Envelope {
	t.Helper()
	env, err := events.New(typ, 1, "moderation", uuid.NewString(), data, time.Now())
	require.NoError(t, err)
	return env
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

func status(t *testing.T, repo *repository.Videos, id uuid.UUID) string {
	t.Helper()
	row, err := repo.GetAny(context.Background(), id)
	require.NoError(t, err)
	return row.ModerationStatus
}

func TestRemoveThenRestore(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	env := envelope(t, events.TypeModerationContentRemoved, removed(f.video, f.user, events.ModerationTargetVideo))
	require.NoError(t, f.h.Handle(ctx, env))
	require.NoError(t, f.h.Handle(ctx, env), "a redelivery is harmless")
	require.Equal(t, model.ModerationRemoved, status(t, f.repo, f.video))
	require.False(t, f.mem.Has(f.thumb))
	require.True(t, f.mem.Quarantined(f.thumb))

	require.NoError(t, f.h.Handle(ctx, envelope(t, events.TypeModerationContentRestored, restored(f.video, f.user))))
	require.Equal(t, model.ModerationApproved, status(t, f.repo, f.video))
	require.True(t, f.mem.Has(f.thumb))
	require.False(t, f.mem.Quarantined(f.thumb))

	var claims int
	require.NoError(t, testdb.Pool.QueryRow(ctx,
		`SELECT count(*) FROM processed_events WHERE consumer = $1 AND event_id = $2`, moderation.Group, env.ID).Scan(&claims))
	require.Equal(t, 1, claims)
}

func TestDeletedMediaStayHidden(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	tx, err := testdb.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = f.repo.SoftDelete(ctx, tx, f.video, f.user)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	require.NoError(t, f.h.Handle(ctx, envelope(t, events.TypeVideoDeleted, events.VideoDeletedV1{
		VideoID: f.video.String(), UserID: f.user.String(), DeletedAt: time.Now(),
	})))
	require.True(t, f.mem.Quarantined(f.thumb), "the media of a deleted video leave the public bucket")

	require.NoError(t, f.h.Handle(ctx, envelope(t, events.TypeModerationContentRemoved, removed(f.video, f.user, events.ModerationTargetVideo))))
	require.NoError(t, f.h.Handle(ctx, envelope(t, events.TypeModerationContentRestored, restored(f.video, f.user))))
	require.True(t, f.mem.Quarantined(f.thumb), "a restore never republishes a deleted video")
}

func TestIgnoredEvents(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	unknown := uuid.Must(uuid.NewV7())
	for _, env := range []events.Envelope{
		envelope(t, events.TypeModerationContentRemoved, removed(uuid.Must(uuid.NewV7()), f.user, events.ModerationTargetComment)),
		envelope(t, events.TypeModerationContentRemoved, removed(unknown, f.user, events.ModerationTargetVideo)),
		envelope(t, events.TypeModerationContentRestored, restored(unknown, f.user)),
		envelope(t, events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: unknown.String(), UserID: f.user.String()}),
	} {
		require.NoError(t, f.h.Handle(ctx, env))
	}
	require.True(t, f.mem.Has(f.thumb))
}

func TestBadEventsArePermanent(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	v2 := envelope(t, events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: f.video.String()})
	v2.Version = 2
	badRemoved := envelope(t, events.TypeModerationContentRemoved, removed(f.video, f.user, events.ModerationTargetVideo))
	badRemoved.Data = json.RawMessage(`{"target_type":"video","target_id":"nope"}`)
	restoredComment := restored(f.video, f.user)
	restoredComment.TargetType = events.ModerationTargetComment
	badRestored := envelope(t, events.TypeModerationContentRestored, restored(f.video, f.user))
	badRestored.Data = json.RawMessage(`{"target_type":"video","target_id":"00000000-0000-0000-0000-000000000000"}`)
	cases := map[string]events.Envelope{
		"version 2":          v2,
		"bad removed id":     badRemoved,
		"removed not json":   withData(envelope(t, events.TypeModerationContentRemoved, struct{}{}), `{"target_id":1}`),
		"restored comment":   envelope(t, events.TypeModerationContentRestored, restoredComment),
		"nil restored id":    badRestored,
		"restored not json":  withData(envelope(t, events.TypeModerationContentRestored, struct{}{}), `{"target_id":1}`),
		"bad deleted id":     envelope(t, events.TypeVideoDeleted, events.VideoDeletedV1{VideoID: "x"}),
		"deleted not json":   withData(envelope(t, events.TypeVideoDeleted, struct{}{}), `{"video_id":1}`),
		"unsupported events": envelope(t, events.TypeVideoReady, events.VideoReadyV1{}),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			err := f.h.Handle(ctx, env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "%v", err)
		})
	}
	require.True(t, f.mem.Has(f.thumb))
}

func withData(env events.Envelope, raw string) events.Envelope {
	env.Data = json.RawMessage(raw)
	return env
}
