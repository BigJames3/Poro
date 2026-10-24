package erasure_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/video/internal/erasure"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/moderation"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

// flaky fails the first DeleteSource calls, like a storage outage.
type flaky struct {
	*storage.Memory
	failures int
}

func (f *flaky) DeleteSource(ctx context.Context, key string) error {
	if f.failures > 0 {
		f.failures--
		return errors.New("storage unavailable")
	}
	return f.Memory.DeleteSource(ctx, key)
}

type fixture struct {
	repo  *repository.Videos
	mem   *storage.Memory
	store *flaky
	h     *erasure.Handler
	user  uuid.UUID
	other uuid.UUID
	// ready, uploading, processing and gone (deleted by its owner before)
	// belong to user; kept belongs to other.
	ready, uploading, processing, gone, kept uuid.UUID
	uploadID                                 string
}

func setup(t *testing.T) fixture {
	t.Helper()
	testdb.Available(t)
	ctx := context.Background()
	mem := storage.NewMemory()
	store := &flaky{Memory: mem}
	repo := repository.New(testdb.Pool)
	f := fixture{
		repo: repo, mem: mem, store: store, h: erasure.New(testdb.Pool, repo, store, zap.NewNop()),
		user: uuid.Must(uuid.NewV7()), other: uuid.Must(uuid.NewV7()),
		ready: uuid.Must(uuid.NewV7()), uploading: uuid.Must(uuid.NewV7()), processing: uuid.Must(uuid.NewV7()),
		gone: uuid.Must(uuid.NewV7()), kept: uuid.Must(uuid.NewV7()),
	}
	var err error
	f.uploadID, err = mem.CreateMultipart(ctx, model.SourceKey(f.user.String(), f.uploading.String(), "mp4"), "video/mp4")
	require.NoError(t, err)
	f.create(t, f.user, f.ready, model.StatusReady, nil)
	f.create(t, f.user, f.uploading, model.StatusUploading, &f.uploadID)
	f.create(t, f.user, f.processing, model.StatusProcessing, nil)
	f.create(t, f.user, f.gone, model.StatusReady, nil)
	f.create(t, f.other, f.kept, model.StatusReady, nil)

	tx, err := testdb.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = repo.SoftDelete(ctx, tx, f.gone, f.user)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	return f
}

func (f fixture) create(t *testing.T, user, video uuid.UUID, status string, uploadID *string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	key := model.SourceKey(user.String(), video.String(), "mp4")
	require.NoError(t, f.repo.Create(ctx, &model.Video{
		ID: video.String(), UserID: user.String(), Title: "my title", Description: "my #description", Status: status,
		ContentType: "video/mp4", SizeBytes: 1, SourceKey: key, S3UploadID: uploadID, CreatedAt: now, UpdatedAt: now,
	}))
	if status == model.StatusUploading {
		return
	}
	f.mem.PutSource(key, []byte("source"))
	if status == model.StatusReady {
		require.NoError(t, f.mem.PutMedia(ctx, model.ThumbKey(user.String(), video.String()), "image/jpeg", bytes.NewReader([]byte("x"))))
	}
}

func deletedEnvelope(t *testing.T, user string) events.Envelope {
	t.Helper()
	env, err := events.New(events.TypeAuthUserDeleted, 1, "auth", user, events.AuthUserDeletedV1{
		UserID: user, DeletedAt: time.Now(),
	}, time.Now())
	require.NoError(t, err)
	return env
}

// deletions returns the poro.video.deleted events published for user.
func deletions(t *testing.T, user uuid.UUID) []events.Envelope {
	t.Helper()
	rows, err := testdb.Pool.Query(context.Background(), `SELECT payload FROM outbox_events
		WHERE topic = $1 AND payload->'data'->>'user_id' = $2 ORDER BY created_at, id`, events.TypeVideoDeleted, user.String())
	require.NoError(t, err)
	defer rows.Close()
	var out []events.Envelope
	for rows.Next() {
		var raw []byte
		require.NoError(t, rows.Scan(&raw))
		var env events.Envelope
		require.NoError(t, json.Unmarshal(raw, &env))
		out = append(out, env)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestErasesEveryVideoOfTheAccount(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	env := deletedEnvelope(t, f.user.String())
	require.NoError(t, f.h.Handle(ctx, env))

	for _, id := range []uuid.UUID{f.ready, f.uploading, f.processing, f.gone} {
		row, err := f.repo.GetAny(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, row.DeletedAt, "video %s is deleted", id)
		require.Empty(t, row.Title)
		require.Empty(t, row.Description)
		require.False(t, f.mem.HasSource(row.SourceKey), "source of %s is deleted", id)
	}
	require.False(t, f.mem.HasUpload(f.uploadID), "the open upload is aborted")

	published := deletions(t, f.user)
	require.Len(t, published, 3, "one poro.video.deleted per live video, none for the one already deleted")
	ids := map[string]bool{}
	for _, ev := range published {
		var d events.VideoDeletedV1
		require.NoError(t, ev.DecodeData(&d))
		ids[d.VideoID] = true
	}
	require.Equal(t, map[string]bool{f.ready.String(): true, f.uploading.String(): true, f.processing.String(): true}, ids)

	// The existing moderation consumer quarantines the media on those events.
	mod := moderation.New(testdb.Pool, f.repo, f.mem, zap.NewNop())
	for _, ev := range published {
		require.NoError(t, mod.Handle(ctx, ev))
	}
	require.False(t, f.mem.Has(model.ThumbKey(f.user.String(), f.ready.String())))
	require.True(t, f.mem.Quarantined(model.ThumbKey(f.user.String(), f.ready.String())))

	// Another account is untouched.
	kept, err := f.repo.GetAny(ctx, f.kept)
	require.NoError(t, err)
	require.Nil(t, kept.DeletedAt)
	require.Equal(t, "my title", kept.Title)
	require.Equal(t, "my #description", kept.Description)
	require.True(t, f.mem.HasSource(kept.SourceKey))
	require.True(t, f.mem.Has(model.ThumbKey(f.other.String(), f.kept.String())))
	require.Empty(t, deletions(t, f.other))

	// A replay and a second event for the same account publish nothing more.
	require.NoError(t, f.h.Handle(ctx, env))
	require.NoError(t, f.h.Handle(ctx, deletedEnvelope(t, f.user.String())))
	require.Len(t, deletions(t, f.user), 3)

	var claims int
	require.NoError(t, testdb.Pool.QueryRow(ctx,
		`SELECT count(*) FROM processed_events WHERE consumer = $1 AND event_id = $2`, erasure.Group, env.ID).Scan(&claims))
	require.Equal(t, 1, claims)
}

func TestStorageFailureIsRetried(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	f.store.failures = 1
	env := deletedEnvelope(t, f.user.String())
	err := f.h.Handle(ctx, env)
	require.Error(t, err)
	require.False(t, kafka.IsPermanent(err), "a storage outage is retried")

	var claims int
	require.NoError(t, testdb.Pool.QueryRow(ctx,
		`SELECT count(*) FROM processed_events WHERE consumer = $1 AND event_id = $2`, erasure.Group, env.ID).Scan(&claims))
	require.Zero(t, claims, "the inbox is claimed only once storage is done")
	require.Len(t, deletions(t, f.user), 3, "the database is committed before storage")

	require.NoError(t, f.h.Handle(ctx, env), "the redelivery finishes the deletions")
	for _, id := range []uuid.UUID{f.ready, f.uploading, f.processing, f.gone} {
		row, err := f.repo.GetAny(ctx, id)
		require.NoError(t, err)
		require.False(t, f.mem.HasSource(row.SourceKey))
	}
	require.False(t, f.mem.HasUpload(f.uploadID))
	require.Len(t, deletions(t, f.user), 3, "the redelivery publishes nothing more")
	require.NoError(t, testdb.Pool.QueryRow(ctx,
		`SELECT count(*) FROM processed_events WHERE consumer = $1 AND event_id = $2`, erasure.Group, env.ID).Scan(&claims))
	require.Equal(t, 1, claims)
}

func TestUnknownAccountIsANoop(t *testing.T) {
	f := setup(t)
	require.NoError(t, f.h.Handle(context.Background(), deletedEnvelope(t, uuid.NewString())))
	require.True(t, f.mem.HasSource(model.SourceKey(f.user.String(), f.ready.String(), "mp4")))
}

func TestBadEventsArePermanent(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	v2 := deletedEnvelope(t, f.user.String())
	v2.Version = 2
	wrongType := deletedEnvelope(t, f.user.String())
	wrongType.Type = events.TypeVideoDeleted
	notJSON := deletedEnvelope(t, f.user.String())
	notJSON.Data = json.RawMessage(`{"user_id":1}`)
	cases := map[string]events.Envelope{
		"version 2":   v2,
		"wrong type":  wrongType,
		"not json":    notJSON,
		"bad user id": deletedEnvelope(t, "nope"),
		"nil user id": deletedEnvelope(t, uuid.Nil.String()),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			err := f.h.Handle(ctx, env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "%v", err)
		})
	}
	row, err := f.repo.GetAny(ctx, f.ready)
	require.NoError(t, err)
	require.Nil(t, row.DeletedAt)
	require.True(t, f.mem.HasSource(row.SourceKey))
}

func TestErasureDeletesThePhotoSources(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	post := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	keys := []string{model.ImageSourceKey(f.user.String(), post.String(), 1), model.ImageSourceKey(f.user.String(), post.String(), 2)}
	require.NoError(t, f.repo.CreatePhoto(ctx, &model.Video{
		ID: post.String(), UserID: f.user.String(), Description: "lieu exact dans l'EXIF", Status: model.StatusUploading,
		ContentType: model.PhotoContentType, SizeBytes: 2, SourceKey: model.MediaPrefix(f.user.String(), post.String()),
		CreatedAt: now, UpdatedAt: now,
	}, []model.Image{
		{Position: 1, ContentType: "image/jpeg", SizeBytes: 1, SourceKey: keys[0]},
		{Position: 2, ContentType: "image/png", SizeBytes: 1, SourceKey: keys[1]},
	}))
	for _, key := range keys {
		f.mem.PutSource(key, []byte("x"))
	}
	require.NoError(t, f.h.Handle(ctx, deletedEnvelope(t, f.user.String())))

	row, err := f.repo.GetAny(ctx, post)
	require.NoError(t, err)
	require.NotNil(t, row.DeletedAt)
	require.Empty(t, row.Description, "the caption is erased")
	for _, key := range keys {
		require.False(t, f.mem.HasSource(key), "the original %s (GPS) is deleted", key)
	}
}
