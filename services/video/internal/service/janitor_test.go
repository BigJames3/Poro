package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/video/internal/dto"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/service"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/testdb"
)

// failingAbort fails the first aborts, as a storage outage would.
type failingAbort struct {
	*storage.Memory
	failures int
}

func (s *failingAbort) AbortMultipart(ctx context.Context, key, uploadID string) error {
	if s.failures > 0 {
		s.failures--
		return errors.New("503 slow down")
	}
	return s.Memory.AbortMultipart(ctx, key, uploadID)
}

func startUpload(t *testing.T, svc *service.Videos, user uuid.UUID) *dto.InitResponse {
	t.Helper()
	init, err := svc.InitUpload(context.Background(), user, dto.InitRequest{
		Title: "clip", Description: "d", ContentType: "video/mp4", SizeBytes: int64(len(fakeMP4())),
	})
	require.NoError(t, err)
	return init
}

// age backdates an upload: its row and its multipart upload.
func age(t *testing.T, mem *storage.Memory, init *dto.InitResponse, by time.Duration) {
	t.Helper()
	_, err := testdb.Pool.Exec(context.Background(), `UPDATE videos SET created_at = now() - $2::interval WHERE id = $1`,
		init.VideoID, by.String())
	require.NoError(t, err)
	mem.SetUploadInitiated(init.UploadID, time.Now().Add(-by))
}

func status(t *testing.T, id string) (string, bool) {
	t.Helper()
	row, err := repository.New(testdb.Pool).GetAny(context.Background(), uuid.MustParse(id))
	require.NoError(t, err)
	return row.Status, row.DeletedAt != nil
}

func TestJanitorAbandonsStaleUploadsOnly(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user := uuid.Must(uuid.NewV7())

	stale := startUpload(t, svc, user)
	age(t, mem, stale, 25*time.Hour)
	fresh := startUpload(t, svc, user)
	age(t, mem, fresh, 23*time.Hour)
	done := startUpload(t, svc, user)
	mem.PutPart(done.UploadID, 1, fakeMP4())
	_, err := svc.Complete(ctx, user, uuid.MustParse(done.VideoID), dto.CompleteRequest{Parts: []dto.CompletePart{{PartNumber: 1, ETag: "e"}}})
	require.NoError(t, err)
	_, err = testdb.Pool.Exec(ctx, `UPDATE videos SET created_at = now() - interval '48 hours' WHERE id = $1`, done.VideoID)
	require.NoError(t, err)
	orphan, err := mem.CreateMultipart(ctx, model.SourceKey(user.String(), uuid.NewString(), "mp4"), "video/mp4")
	require.NoError(t, err)
	mem.SetUploadInitiated(orphan, time.Now().Add(-30*time.Hour))

	janitor := service.NewUploadJanitor(repository.New(testdb.Pool), mem, 24*time.Hour, time.Hour, zap.NewNop())
	abandoned, orphans, err := janitor.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, abandoned)
	require.Equal(t, 1, orphans, "the multipart upload no row knows about is aborted too")

	st, deleted := status(t, stale.VideoID)
	require.Equal(t, model.StatusAbandoned, st)
	require.True(t, deleted, "an abandoned upload leaves the owner's list")
	require.False(t, mem.HasUpload(stale.UploadID), "its multipart upload is aborted")
	require.False(t, mem.HasUpload(orphan))

	st, deleted = status(t, fresh.VideoID)
	require.Equal(t, model.StatusUploading, st, "a slow upload under the limit is left alone")
	require.False(t, deleted)
	require.True(t, mem.HasUpload(fresh.UploadID))
	st, _ = status(t, done.VideoID)
	require.Equal(t, model.StatusProcessing, st, "a completed upload is never abandoned, however old")

	listed, err := svc.List(ctx, user, "", 20)
	require.NoError(t, err)
	require.Len(t, listed.Items, 2)

	abandoned, orphans, err = janitor.RunOnce(ctx)
	require.NoError(t, err)
	require.Zero(t, abandoned, "a second pass finds nothing")
	require.Zero(t, orphans)
}

func TestJanitorKeepsAnUploadWhoseAbortFailedForTheNextPass(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	stale := startUpload(t, svc, user)
	age(t, mem, stale, 25*time.Hour)

	store := &failingAbort{Memory: mem, failures: 1}
	// The orphan sweep uses the wrapped store's own abort, so only the row
	// pass sees the failure.
	janitor := service.NewUploadJanitor(repository.New(testdb.Pool), store, 24*time.Hour, time.Hour, zap.NewNop())
	abandoned, _, err := janitor.RunOnce(ctx)
	require.NoError(t, err)
	require.Zero(t, abandoned)
	st, deleted := status(t, stale.VideoID)
	require.Equal(t, model.StatusUploading, st, "a row whose abort failed is not marked abandoned")
	require.False(t, deleted)

	abandoned, _, err = janitor.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, abandoned)
	st, _ = status(t, stale.VideoID)
	require.Equal(t, model.StatusAbandoned, st)
}

func TestJanitorRunStopsWithItsContext(t *testing.T) {
	_, mem := newSvc(t)
	janitor := service.NewUploadJanitor(repository.New(testdb.Pool), mem, 24*time.Hour, time.Millisecond, zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		janitor.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("janitor did not stop")
	}
}
