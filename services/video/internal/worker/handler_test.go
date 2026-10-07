package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"

	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/testdb"
	"github.com/poro/video/internal/transcode"
	"github.com/poro/video/internal/worker"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

type fakeFF struct {
	err error
	out *transcode.Output
}

func (f fakeFF) Transcode(_ context.Context, _, workDir, userID, videoID string) (*transcode.Output, error) {
	if f.err != nil {
		return nil, f.err
	}
	requireWrite := func(rel, body string) {
		p := filepath.Join(workDir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
	}
	requireWrite("hls/master.m3u8", "#EXTM3U\n")
	requireWrite("hls/360p/index.m3u8", "#EXTM3U\n")
	requireWrite("hls/360p/seg_000.ts", "ts")
	requireWrite("thumb.jpg", "jpg")
	out := f.out
	if out == nil {
		out = &transcode.Output{
			DurationMs: 1500, Width: 640, Height: 360,
			Renditions: []model.Rendition{{
				Name: "360p", Bandwidth: 896000, Width: 640, Height: 360,
				PlaylistKey: model.HLSDir(userID, videoID) + "/360p/index.m3u8",
			}},
			Files: []transcode.File{
				{Rel: "hls/master.m3u8", ContentType: "application/vnd.apple.mpegurl"},
				{Rel: "hls/360p/index.m3u8", ContentType: "application/vnd.apple.mpegurl"},
				{Rel: "hls/360p/seg_000.ts", ContentType: "video/mp2t"},
				{Rel: "thumb.jpg", ContentType: "image/jpeg"},
			},
		}
	}
	return out, nil
}

func seedProcessing(t *testing.T, user, id uuid.UUID) {
	t.Helper()
	testdb.Available(t)
	now := time.Now().UTC()
	v := &model.Video{
		ID: id.String(), UserID: user.String(), Title: "c", Description: "Danse #Abidjan #wax", Status: model.StatusProcessing,
		ContentType: "video/mp4", SizeBytes: 32, SourceKey: model.SourceKey(user.String(), id.String(), "mp4"),
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repository.New(testdb.Pool).Create(t.Context(), v))
}

func uploadedEnv(t *testing.T, user, id uuid.UUID, key string) events.Envelope {
	t.Helper()
	env, err := events.New(events.TypeVideoUploaded, 1, "video", id.String(), events.VideoUploadedV1{
		VideoID: id.String(), UserID: user.String(), SourceKey: key, ContentType: "video/mp4", SizeBytes: 32, UploadedAt: time.Now(),
	}, time.Now())
	require.NoError(t, err)
	return env
}

func TestHandlerMarksReady(t *testing.T) {
	ctx := context.Background()
	user, id := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	seedProcessing(t, user, id)
	mem := storage.NewMemory()
	key := model.SourceKey(user.String(), id.String(), "mp4")
	require.NoError(t, mem.Put(ctx, key, "video/mp4", bytes.NewReader([]byte("src"))))

	h := worker.New(testdb.Pool, repository.New(testdb.Pool), mem, fakeFF{}, zap.NewNop())
	env := uploadedEnv(t, user, id, key)
	require.NoError(t, h.Handle(ctx, env))

	got, err := repository.New(testdb.Pool).Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, model.StatusReady, got.Status)
	require.NotNil(t, got.HLSKey)
	require.NotEmpty(t, mem.Bytes(*got.HLSKey))
	require.NotEmpty(t, mem.Bytes(*got.ThumbnailKey))

	var n int
	require.NoError(t, testdb.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE topic = $1 AND event_key = $2`,
		events.TypeVideoReady, id.String()).Scan(&n))
	require.Equal(t, 1, n)

	var payload []byte
	require.NoError(t, testdb.Pool.QueryRow(ctx, `SELECT payload FROM outbox_events WHERE topic = $1 AND event_key = $2`,
		events.TypeVideoReady, id.String()).Scan(&payload))
	var ready events.Envelope
	require.NoError(t, json.Unmarshal(payload, &ready))
	var data events.VideoReadyV1
	require.NoError(t, ready.DecodeData(&data))
	require.Equal(t, "c", data.Title)
	require.Equal(t, "Danse #Abidjan #wax", data.Description)
	require.Equal(t, []string{"abidjan", "wax"}, data.Hashtags)
	require.False(t, data.PublishedAt.IsZero())
	require.Equal(t, data.ReadyAt, data.PublishedAt)

	require.NoError(t, h.Handle(ctx, env), "duplicate event is a no-op")
}

func TestHandlerMarksFailedTooLong(t *testing.T) {
	ctx := context.Background()
	user, id := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	seedProcessing(t, user, id)
	mem := storage.NewMemory()
	key := model.SourceKey(user.String(), id.String(), "mp4")
	require.NoError(t, mem.Put(ctx, key, "video/mp4", bytes.NewReader([]byte("src"))))

	h := worker.New(testdb.Pool, repository.New(testdb.Pool), mem, fakeFF{err: transcode.ErrTooLong}, zap.NewNop())
	require.NoError(t, h.Handle(ctx, uploadedEnv(t, user, id, key)))
	got, err := repository.New(testdb.Pool).Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, model.StatusFailed, got.Status)
	require.Equal(t, events.VideoFailTooLong, *got.FailureCode)
}
