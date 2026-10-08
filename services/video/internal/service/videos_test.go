package service_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"

	"github.com/poro/video/internal/config"
	"github.com/poro/video/internal/dto"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/service"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

func newSvc(t *testing.T) (*service.Videos, *storage.Memory) {
	t.Helper()
	testdb.Available(t)
	cfg := &config.Config{S3PublicEndpoint: "http://localhost:9000", S3Bucket: "poro-videos"}
	mem := storage.NewMemory()
	return service.NewVideos(cfg, repository.New(testdb.Pool), mem, zap.NewNop()), mem
}

func fakeMP4() []byte {
	b := bytes.Repeat([]byte{0}, 64)
	copy(b[4:], []byte("ftypisom"))
	return b
}

func TestInitCompleteAndGet(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user := uuid.Must(uuid.NewV7())

	init, err := svc.InitUpload(ctx, user, dto.InitRequest{
		Title: "clip", Description: "d", ContentType: "video/mp4", SizeBytes: int64(len(fakeMP4())),
	})
	require.NoError(t, err)
	require.Len(t, init.Parts, 1)
	require.Equal(t, int64(model.PartSize), init.PartSize)

	mem.PutPart(init.UploadID, 1, fakeMP4())
	vid := uuid.MustParse(init.VideoID)
	view, err := svc.Complete(ctx, user, vid, dto.CompleteRequest{Parts: []dto.CompletePart{{PartNumber: 1, ETag: "etag"}}})
	require.NoError(t, err)
	require.Equal(t, model.StatusProcessing, view.Status)

	var n int
	require.NoError(t, testdb.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_key = $1 AND topic = $2`,
		init.VideoID, events.TypeVideoUploaded).Scan(&n))
	require.Equal(t, 1, n)

	got, err := svc.Get(ctx, vid, &user)
	require.NoError(t, err)
	require.Equal(t, "clip", got.Title)

	other := uuid.Must(uuid.NewV7())
	_, err = svc.Get(ctx, vid, &other)
	require.Error(t, err)

	listed, err := svc.List(ctx, user, "", 20)
	require.NoError(t, err)
	require.Len(t, listed.Items, 1)

	require.NoError(t, svc.Delete(ctx, user, vid))
	_, err = svc.Get(ctx, vid, &user)
	require.Error(t, err)

	require.NoError(t, testdb.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_key = $1 AND topic = $2`,
		init.VideoID, events.TypeVideoDeleted).Scan(&n))
	require.Equal(t, 1, n, "delete publishes poro.video.deleted")

	require.Error(t, svc.Delete(ctx, user, vid), "second delete is not found")
	require.NoError(t, testdb.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_key = $1 AND topic = $2`,
		init.VideoID, events.TypeVideoDeleted).Scan(&n))
	require.Equal(t, 1, n, "no event when nothing changed")
}

func TestInitValidation(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	_, err := svc.InitUpload(ctx, user, dto.InitRequest{Title: "", ContentType: "video/mp4", SizeBytes: 10})
	require.Error(t, err)
	_, err = svc.InitUpload(ctx, user, dto.InitRequest{Title: "ok", ContentType: "video/avi", SizeBytes: 10})
	require.Error(t, err)
	_, err = svc.InitUpload(ctx, user, dto.InitRequest{Title: "ok", ContentType: "video/mp4", SizeBytes: model.MaxBytes + 1})
	require.Error(t, err)
}

func TestCompleteRejectsNonVideo(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	init, err := svc.InitUpload(ctx, user, dto.InitRequest{Title: "x", ContentType: "video/mp4", SizeBytes: 12})
	require.NoError(t, err)
	mem.PutPart(init.UploadID, 1, []byte("<html>nope!!"))
	_, err = svc.Complete(ctx, user, uuid.MustParse(init.VideoID), dto.CompleteRequest{Parts: []dto.CompletePart{{PartNumber: 1, ETag: "e"}}})
	require.Error(t, err)
}

func TestAbort(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	init, err := svc.InitUpload(ctx, user, dto.InitRequest{Title: "x", ContentType: "video/mp4", SizeBytes: 10})
	require.NoError(t, err)
	require.NoError(t, svc.Abort(ctx, user, uuid.MustParse(init.VideoID)))
	_, err = svc.Get(ctx, uuid.MustParse(init.VideoID), &user)
	require.Error(t, err)

	var n int
	require.NoError(t, testdb.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_key = $1 AND topic = $2`,
		init.VideoID, events.TypeVideoDeleted).Scan(&n))
	require.Zero(t, n, "an aborted upload was never visible: no poro.video.deleted")
}

func TestCompleteWrongPartCount(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	init, err := svc.InitUpload(ctx, user, dto.InitRequest{Title: "x", ContentType: "video/mp4", SizeBytes: 10})
	require.NoError(t, err)
	_, err = svc.Complete(ctx, user, uuid.MustParse(init.VideoID), dto.CompleteRequest{})
	require.Error(t, err)
}

func TestRemovedVideosAreHiddenFromEveryoneButTheOwner(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t)
	repo := repository.New(testdb.Pool)
	user, id := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	require.NoError(t, repo.Create(ctx, &model.Video{
		ID: id.String(), UserID: user.String(), Title: "t", Status: model.StatusProcessing, ContentType: "video/mp4",
		SizeBytes: 1, SourceKey: model.SourceKey(user.String(), id.String(), "mp4"), CreatedAt: now, UpdatedAt: now,
	}))
	tx, err := testdb.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.MarkReady(ctx, tx, id, 1000, 720, 1280, "videos/x/master.m3u8", "videos/x/thumb.jpg", nil))
	require.NoError(t, tx.Commit(ctx))

	stranger := uuid.Must(uuid.NewV7())
	visible, err := svc.Get(ctx, id, &stranger)
	require.NoError(t, err)
	require.Equal(t, model.ModerationApproved, visible.ModerationStatus)
	require.NotNil(t, visible.HLSURL)

	_, err = repo.SetModeration(ctx, id, model.ModerationRemoved, now)
	require.NoError(t, err)
	for _, viewer := range []*uuid.UUID{&stranger, nil} {
		_, err = svc.Get(ctx, id, viewer)
		require.Error(t, err)
	}
	own, err := svc.Get(ctx, id, &user)
	require.NoError(t, err)
	require.Equal(t, model.ModerationRemoved, own.ModerationStatus)
	require.Nil(t, own.HLSURL, "a removed video exposes no media URL")
	require.Nil(t, own.ThumbnailURL)
	listed, err := svc.List(ctx, user, "", 20)
	require.NoError(t, err)
	require.Equal(t, model.ModerationRemoved, listed.Items[0].ModerationStatus)
}
