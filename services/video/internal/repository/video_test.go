package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

func TestVideosCRUD(t *testing.T) {
	testdb.Available(t)
	ctx := context.Background()
	repo := repository.New(testdb.Pool)
	id := uuid.Must(uuid.NewV7())
	user := uuid.Must(uuid.NewV7())
	now := time.Now().UTC().Truncate(time.Microsecond)
	upload := "up-1"
	v := &model.Video{
		ID: id.String(), UserID: user.String(), Title: "clip", Description: "d",
		Status: model.StatusUploading, ContentType: "video/mp4", SizeBytes: 100,
		SourceKey: "videos/a/b/source.mp4", S3UploadID: &upload, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.Create(ctx, v))

	got, err := repo.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "clip", got.Title)
	require.Equal(t, model.StatusUploading, got.Status)

	tx, err := testdb.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.MarkProcessing(ctx, tx, id, 200))
	require.NoError(t, tx.Commit(ctx))

	got, err = repo.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, model.StatusProcessing, got.Status)
	require.Equal(t, int64(200), got.SizeBytes)
	require.Nil(t, got.S3UploadID)

	tx, err = testdb.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.MarkReady(ctx, tx, id, 1500, 640, 360, "hls", "thumb", []model.Rendition{{Name: "360p", Bandwidth: 800000, Width: 640, Height: 360, PlaylistKey: "p"}}))
	require.NoError(t, tx.Commit(ctx))
	got, err = repo.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, model.StatusReady, got.Status)
	require.Equal(t, 1500, *got.DurationMs)
	require.Equal(t, "360p", got.Renditions[0].Name)

	listed, err := repo.ListByUser(ctx, user, nil, nil, 10)
	require.NoError(t, err)
	require.Len(t, listed, 1)

	tx, err = testdb.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = repo.SoftDelete(ctx, tx, id, user)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	_, err = repo.Get(ctx, id)
	require.ErrorIs(t, err, repository.ErrNotFound)
}
