package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/httpx"

	"github.com/poro/video/internal/dto"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/testdb"
)

func TestResumeUploadSignsOnlyMissingParts(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	size := 2*int64(model.PartSize) + 10
	init, err := svc.InitUpload(ctx, user, dto.InitRequest{Title: "x", ContentType: "video/mp4", SizeBytes: size})
	require.NoError(t, err)
	require.Len(t, init.Parts, 3)
	vid := uuid.MustParse(init.VideoID)

	// Part 1 arrived whole, part 3 was cut short: only part 1 is kept.
	first := make([]byte, model.PartSize)
	copy(first[4:], "ftypisom")
	mem.PutPart(init.UploadID, 1, first)
	mem.PutPart(init.UploadID, 3, []byte("short"))

	resumed, err := svc.ResumeUpload(ctx, user, vid)
	require.NoError(t, err)
	require.Equal(t, init.UploadID, resumed.UploadID)
	require.Equal(t, int64(model.PartSize), resumed.PartSize)
	require.Equal(t, []dto.CompletePart{{PartNumber: 1, ETag: "etag-1"}}, resumed.UploadedParts)
	require.Len(t, resumed.Parts, 2)
	require.Equal(t, int32(2), resumed.Parts[0].PartNumber)
	require.Contains(t, resumed.Parts[0].URL, "size=8388608")
	require.Equal(t, int32(3), resumed.Parts[1].PartNumber)
	require.Contains(t, resumed.Parts[1].URL, "size=10", "the last part is signed at its own length")

	mem.PutPart(init.UploadID, 2, make([]byte, model.PartSize))
	mem.PutPart(init.UploadID, 3, make([]byte, 10))
	parts := append(resumed.UploadedParts, dto.CompletePart{PartNumber: 2, ETag: "etag-2"}, dto.CompletePart{PartNumber: 3, ETag: "etag-3"})
	view, err := svc.Complete(ctx, user, vid, dto.CompleteRequest{Parts: parts})
	require.NoError(t, err)
	require.Equal(t, model.StatusProcessing, view.Status)

	_, err = svc.ResumeUpload(ctx, user, vid)
	requireCode(t, err, "conflict", "a completed upload cannot be resumed")
}

func TestResumeUploadIsTheOwnersOnly(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user, other := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	init, err := svc.InitUpload(ctx, user, dto.InitRequest{Title: "x", ContentType: "video/mp4", SizeBytes: 10})
	require.NoError(t, err)
	vid := uuid.MustParse(init.VideoID)

	_, err = svc.ResumeUpload(ctx, other, vid)
	requireCode(t, err, "not_found", "another account learns nothing")
	_, err = svc.ResumeUpload(ctx, user, uuid.Must(uuid.NewV7()))
	requireCode(t, err, "not_found", "unknown video")

	// Storage lost the upload (swept or aborted elsewhere): start over.
	require.NoError(t, mem.AbortMultipart(ctx, "", init.UploadID))
	_, err = svc.ResumeUpload(ctx, user, vid)
	requireCode(t, err, "conflict", "the upload is gone from storage")

	require.NoError(t, svc.Abort(ctx, user, vid))
	_, err = svc.ResumeUpload(ctx, user, vid)
	requireCode(t, err, "not_found", "an aborted upload is gone")
}

func TestListPublicShowsOnlyWhatAnyoneMaySee(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	newVideo := func(title, status, moderation string, at time.Time) string {
		init, err := svc.InitUpload(ctx, user, dto.InitRequest{Title: title, ContentType: "video/mp4", SizeBytes: 10})
		require.NoError(t, err)
		_, err = testdb.Pool.Exec(ctx, `UPDATE videos SET status = $2, moderation_status = $3, created_at = $4,
			hls_key = 'hls/master.m3u8', thumbnail_key = 'thumb.jpg' WHERE id = $1`,
			init.VideoID, status, moderation, at)
		require.NoError(t, err)
		return init.VideoID
	}
	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	older := newVideo("older", model.StatusReady, "approved", base)
	newer := newVideo("newer", model.StatusReady, "approved", base.Add(time.Minute))
	newVideo("processing", model.StatusProcessing, "approved", base.Add(2*time.Minute))
	newVideo("removed", model.StatusReady, model.ModerationRemoved, base.Add(3*time.Minute))
	deleted := newVideo("deleted", model.StatusReady, "approved", base.Add(4*time.Minute))
	require.NoError(t, svc.Delete(ctx, user, uuid.MustParse(deleted)))
	photo := newVideo("photo", model.StatusReady, "approved", base.Add(5*time.Minute))
	_, err := testdb.Pool.Exec(ctx, `UPDATE videos SET media_type = 'photo' WHERE id = $1`, photo)
	require.NoError(t, err)

	page, err := svc.ListPublic(ctx, user, "", 1, false)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, newer, page.Items[0].ID)
	require.NotNil(t, page.Items[0].HLSURL)
	require.NotNil(t, page.NextCursor)
	page, err = svc.ListPublic(ctx, user, *page.NextCursor, 1, false)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, older, page.Items[0].ID)
	require.Nil(t, page.NextCursor)

	all, err := svc.ListPublic(ctx, user, "", 20, true)
	require.NoError(t, err)
	ids := make([]string, 0, len(all.Items))
	for _, item := range all.Items {
		ids = append(ids, item.ID)
	}
	require.Equal(t, []string{photo, newer, older}, ids, "photo posts only with the capability")

	_, err = svc.ListPublic(ctx, user, "nope", 20, false)
	requireCode(t, err, "invalid_request", "bad cursor")
}

func requireCode(t *testing.T, err error, code, msg string) {
	t.Helper()
	var apiErr *httpx.APIError
	require.ErrorAs(t, err, &apiErr, msg)
	require.Equal(t, code, apiErr.Code, msg)
}
