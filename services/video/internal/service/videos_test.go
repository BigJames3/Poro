package service_test

import (
	"bytes"
	"context"
	"os"
	"testing"

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
