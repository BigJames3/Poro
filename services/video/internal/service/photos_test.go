package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/httpx"

	"github.com/poro/video/internal/config"
	"github.com/poro/video/internal/dto"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/service"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/testdb"
)

var (
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}, bytes.Repeat([]byte{1}, 40)...)
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), bytes.Repeat([]byte{2}, 40)...)
	webpBytes = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{3}, 40)...)
	heicBytes = append([]byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic"), bytes.Repeat([]byte{4}, 40)...)
)

// apiCode is the error code an API error answers with.
func apiCode(t *testing.T, err error) string {
	t.Helper()
	var apiErr *httpx.APIError
	require.True(t, errors.As(err, &apiErr), "an API error, got %v", err)
	return apiErr.Code
}

type upload struct {
	contentType string
	body        []byte
}

// startPhoto creates a photo post of the given images and returns it.
func startPhoto(t *testing.T, svc *service.Videos, user uuid.UUID, caption string, images ...upload) *dto.InitPhotoResponse {
	t.Helper()
	specs := make([]dto.PhotoImageSpec, len(images))
	for i, img := range images {
		specs[i] = dto.PhotoImageSpec{ContentType: img.contentType, SizeBytes: int64(len(img.body))}
	}
	init, err := svc.InitPhotoUpload(context.Background(), user, dto.InitPhotoRequest{Description: caption, Images: specs})
	require.NoError(t, err)
	return init
}

// send stores the images as the presigned PUTs would.
func send(mem *storage.Memory, user uuid.UUID, init *dto.InitPhotoResponse, images ...upload) {
	for i, img := range images {
		mem.PutObject(model.ImageSourceKey(user.String(), init.VideoID, i+1), img.contentType, img.body)
	}
}

func outboxCount(t *testing.T, videoID string) int {
	t.Helper()
	var n int
	require.NoError(t, testdb.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox_events WHERE event_key = $1 AND topic = $2`, videoID, events.TypeVideoUploaded).Scan(&n))
	return n
}

func TestPhotoPostInitValidatesEveryImage(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	spec := func(ct string, size int64) dto.PhotoImageSpec {
		return dto.PhotoImageSpec{ContentType: ct, SizeBytes: size}
	}
	many := func(n int) []dto.PhotoImageSpec {
		out := make([]dto.PhotoImageSpec, n)
		for i := range out {
			out[i] = spec("image/jpeg", 100)
		}
		return out
	}
	cases := map[string]struct {
		req  dto.InitPhotoRequest
		code string
	}{
		"no image":            {dto.InitPhotoRequest{}, "image_count_invalid"},
		"11 images":           {dto.InitPhotoRequest{Images: many(11)}, "image_count_invalid"},
		"heic":                {dto.InitPhotoRequest{Images: []dto.PhotoImageSpec{spec("image/heic", 100)}}, "content_type_invalid"},
		"heif":                {dto.InitPhotoRequest{Images: []dto.PhotoImageSpec{spec("image/heif", 100)}}, "content_type_invalid"},
		"gif":                 {dto.InitPhotoRequest{Images: []dto.PhotoImageSpec{spec("image/gif", 100)}}, "content_type_invalid"},
		"video":               {dto.InitPhotoRequest{Images: []dto.PhotoImageSpec{spec("video/mp4", 100)}}, "content_type_invalid"},
		"empty size":          {dto.InitPhotoRequest{Images: []dto.PhotoImageSpec{spec("image/png", 0)}}, "size_invalid"},
		"10 MiB + 1":          {dto.InitPhotoRequest{Images: []dto.PhotoImageSpec{spec("image/png", model.MaxImageBytes+1)}}, "payload_too_large"},
		"one bad of three":    {dto.InitPhotoRequest{Images: []dto.PhotoImageSpec{spec("image/jpeg", 1), spec("image/webp", 1), spec("image/bmp", 1)}}, "content_type_invalid"},
		"caption 501 runes":   {dto.InitPhotoRequest{Description: strings.Repeat("é", 501), Images: many(1)}, "description_invalid"},
		"caption with U+200B": {dto.InitPhotoRequest{Description: "a\u200bb", Images: many(1)}, "description_invalid"},
	}
	for name, c := range cases {
		_, err := svc.InitPhotoUpload(ctx, user, c.req)
		require.Error(t, err, name)
		require.Equal(t, c.code, apiCode(t, err), name)
	}
	listed, err := svc.List(ctx, user, "", 50, true)
	require.NoError(t, err)
	require.Empty(t, listed.Items, "a refused request creates nothing")

	init, err := svc.InitPhotoUpload(ctx, user, dto.InitPhotoRequest{Images: append(many(9), spec(" IMAGE/PNG ", model.MaxImageBytes))})
	require.NoError(t, err, "10 images, the last one of exactly 10 MiB, no caption")
	require.Len(t, init.Images, 10)
	require.Equal(t, 3600, init.ExpiresIn)
	for i, img := range init.Images {
		require.Equal(t, i+1, img.Position)
		require.Contains(t, img.URL, model.ImageSourceKey(user.String(), init.VideoID, i+1))
	}
	require.Equal(t, "image/png", init.Images[9].Headers["Content-Type"], "the type is normalised and signed")
	require.Equal(t, "10485760", init.Images[9].Headers["Content-Length"], "the exact size is signed")
}

func TestPhotoPostCompletesOnlyWithValidImages(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	images := []upload{{"image/jpeg", jpegBytes}, {"image/png", pngBytes}, {"image/webp", webpBytes}}
	init := startPhoto(t, svc, user, "Marché de Treichville", images...)
	post := uuid.MustParse(init.VideoID)

	// Two images sent, one missing: nothing is deleted, the client can finish.
	send(mem, user, init, images[:2]...)
	_, err := svc.CompletePhotoUpload(ctx, user, post)
	require.Equal(t, "upload_incomplete", apiCode(t, err))
	require.True(t, mem.HasSource(model.ImageSourceKey(user.String(), init.VideoID, 1)), "a missing image deletes nothing")

	send(mem, user, init, images...)
	view, err := svc.CompletePhotoUpload(ctx, user, post)
	require.NoError(t, err)
	require.Equal(t, model.StatusProcessing, view.Status, "processing, never ready before the worker")
	require.Equal(t, model.MediaTypePhoto, view.MediaType)
	require.Empty(t, view.Title)
	require.Equal(t, "Marché de Treichville", view.Description)
	require.Empty(t, view.Images, "no picture before processing")
	require.Equal(t, 1, outboxCount(t, init.VideoID))

	var raw []byte
	require.NoError(t, testdb.Pool.QueryRow(ctx, `SELECT payload FROM outbox_events WHERE event_key = $1`, init.VideoID).Scan(&raw))
	var env events.Envelope
	require.NoError(t, json.Unmarshal(raw, &env))
	var data events.VideoUploadedV1
	require.NoError(t, env.DecodeData(&data))
	require.Equal(t, model.MediaTypePhoto, data.MediaType)
	require.Equal(t, int64(len(jpegBytes)+len(pngBytes)+len(webpBytes)), data.SizeBytes)

	// Completing again (lost answer, double tap) changes nothing.
	_, err = svc.CompletePhotoUpload(ctx, user, post)
	require.Equal(t, "conflict", apiCode(t, err))
	require.Equal(t, 1, outboxCount(t, init.VideoID), "no second event")
	err = svc.Abort(ctx, user, post)
	require.Equal(t, "conflict", apiCode(t, err), "a completed post cannot be aborted")
	got, err := svc.Get(ctx, post, &user, true)
	require.NoError(t, err)
	require.Equal(t, model.StatusProcessing, got.Status)
	require.True(t, mem.HasSource(model.ImageSourceKey(user.String(), init.VideoID, 3)), "its sources stay for the worker")
}

func TestPhotoPostWithAnInvalidImageIsDeletedWhole(t *testing.T) {
	ctx := context.Background()
	user := uuid.Must(uuid.NewV7())
	cases := map[string]struct {
		declared string
		body     []byte
		code     string
	}{
		"heic renamed jpeg": {"image/jpeg", heicBytes, "invalid_media"},
		"png declared jpeg": {"image/jpeg", pngBytes, "invalid_media"},
		"html declared png": {"image/png", []byte("<html><script>alert(1)</script></html>"), "invalid_media"},
		"mp4 declared webp": {"image/webp", fakeMP4(), "invalid_media"},
	}
	for name, c := range cases {
		svc, mem := newSvc(t)
		good := upload{"image/jpeg", jpegBytes}
		bad := upload{c.declared, c.body}
		init := startPhoto(t, svc, user, "", good, bad, good)
		send(mem, user, init, good, bad, good)

		_, err := svc.CompletePhotoUpload(ctx, user, uuid.MustParse(init.VideoID))
		require.Equal(t, c.code, apiCode(t, err), name)
		for pos := 1; pos <= 3; pos++ {
			require.False(t, mem.HasSource(model.ImageSourceKey(user.String(), init.VideoID, pos)), "%s: source %d deleted", name, pos)
		}
		_, err = svc.Get(ctx, uuid.MustParse(init.VideoID), &user, true)
		require.Equal(t, "not_found", apiCode(t, err), "%s: no partial post remains", name)
		require.Zero(t, outboxCount(t, init.VideoID), name)
	}
}

func TestPhotoPostStoredSizeIsChecked(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	init := startPhoto(t, svc, user, "", upload{"image/jpeg", jpegBytes})
	// Real storage refuses another length (signed); a store that did not is caught here.
	mem.PutObject(model.ImageSourceKey(user.String(), init.VideoID, 1), "image/jpeg", append(jpegBytes, 0))
	_, err := svc.CompletePhotoUpload(ctx, user, uuid.MustParse(init.VideoID))
	require.Equal(t, "invalid_media", apiCode(t, err))

	big := startPhoto(t, svc, user, "", upload{"image/jpeg", jpegBytes})
	mem.PutObject(model.ImageSourceKey(user.String(), big.VideoID, 1), "image/jpeg",
		append(append([]byte{}, jpegBytes...), make([]byte, model.MaxImageBytes)...))
	_, err = svc.CompletePhotoUpload(ctx, user, uuid.MustParse(big.VideoID))
	require.Equal(t, "payload_too_large", apiCode(t, err))
	require.False(t, mem.HasSource(model.ImageSourceKey(user.String(), big.VideoID, 1)))
}

// failingStore fails HeadSource or DeleteSource on demand.
type failingStore struct {
	*storage.Memory
	failHead, failDelete bool
}

func (f *failingStore) HeadSource(ctx context.Context, key string) (storage.Object, error) {
	if f.failHead {
		return storage.Object{}, errors.New("storage down")
	}
	return f.Memory.HeadSource(ctx, key)
}

func (f *failingStore) DeleteSource(ctx context.Context, key string) error {
	if f.failDelete {
		return errors.New("storage down")
	}
	return f.Memory.DeleteSource(ctx, key)
}

func TestPhotoPostStorageFailuresKeepEverythingForARetry(t *testing.T) {
	ctx := context.Background()
	testdb.Available(t)
	store := &failingStore{Memory: storage.NewMemory()}
	cfg := &config.Config{S3PublicEndpoint: "http://localhost:9000", S3Bucket: "poro-videos"}
	svc := service.NewVideos(cfg, repository.New(testdb.Pool), store, zap.NewNop())
	user := uuid.Must(uuid.NewV7())
	init := startPhoto(t, svc, user, "", upload{"image/jpeg", jpegBytes})
	post := uuid.MustParse(init.VideoID)
	send(store.Memory, user, init, upload{"image/jpeg", jpegBytes})

	store.failHead = true
	_, err := svc.CompletePhotoUpload(ctx, user, post)
	require.Equal(t, "internal_error", apiCode(t, err), "a storage failure is not reported as a missing image")
	got, err := svc.Get(ctx, post, &user, true)
	require.NoError(t, err)
	require.Equal(t, model.StatusUploading, got.Status, "the post waits for a retry")

	store.failDelete = true
	err = svc.Abort(ctx, user, post)
	require.Equal(t, "internal_error", apiCode(t, err))
	_, err = svc.Get(ctx, post, &user, true)
	require.NoError(t, err, "a post whose sources could not be deleted is kept, so the abort can be repeated")

	store.failHead, store.failDelete = false, false
	view, err := svc.CompletePhotoUpload(ctx, user, post)
	require.NoError(t, err, "the retry succeeds once storage is back")
	require.Equal(t, model.StatusProcessing, view.Status)
}

func TestPhotoPostAbortSweepAndDelete(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	img := upload{"image/png", pngBytes}

	aborted := startPhoto(t, svc, user, "", img, img)
	send(mem, user, aborted, img, img)
	require.NoError(t, svc.Abort(ctx, user, uuid.MustParse(aborted.VideoID)))
	for pos := 1; pos <= 2; pos++ {
		require.False(t, mem.HasSource(model.ImageSourceKey(user.String(), aborted.VideoID, pos)), "abort deletes the sources")
	}
	_, err := svc.Get(ctx, uuid.MustParse(aborted.VideoID), &user, true)
	require.Equal(t, "not_found", apiCode(t, err))

	left := startPhoto(t, svc, user, "", img)
	send(mem, user, left, img)
	n, err := svc.SweepAbandonedUploads(ctx, time.Now().Add(time.Minute), 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 1)
	require.False(t, mem.HasSource(model.ImageSourceKey(user.String(), left.VideoID, 1)), "the sweeper deletes abandoned sources")

	deleted := startPhoto(t, svc, user, "", img)
	send(mem, user, deleted, img)
	require.NoError(t, svc.Delete(ctx, user, uuid.MustParse(deleted.VideoID)))
	require.False(t, mem.HasSource(model.ImageSourceKey(user.String(), deleted.VideoID, 1)), "delete removes the originals (GPS)")
}

func TestPhotoPostsAreOwnedAndKeptApartFromVideos(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	owner, stranger := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	img := upload{"image/jpeg", jpegBytes}
	init := startPhoto(t, svc, owner, "", img)
	send(mem, owner, init, img)
	post := uuid.MustParse(init.VideoID)

	_, err := svc.CompletePhotoUpload(ctx, stranger, post)
	require.Equal(t, "not_found", apiCode(t, err), "another account's post is not found")
	require.Equal(t, "not_found", apiCode(t, svc.Abort(ctx, stranger, post)))
	_, err = svc.Complete(ctx, owner, post, dto.CompleteRequest{Parts: []dto.CompletePart{{PartNumber: 1, ETag: "e"}}})
	require.Equal(t, "not_found", apiCode(t, err), "the video route never completes a photo post")

	video, err := svc.InitUpload(ctx, owner, dto.InitRequest{Title: "clip", ContentType: "video/mp4", SizeBytes: int64(len(fakeMP4()))})
	require.NoError(t, err)
	_, err = svc.CompletePhotoUpload(ctx, owner, uuid.MustParse(video.VideoID))
	require.Equal(t, "not_found", apiCode(t, err), "the photo route never completes a video")
}

func TestClientsWithoutThePhotoCapabilityNeverReceivePhotoPosts(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user := uuid.Must(uuid.NewV7())
	img := upload{"image/webp", webpBytes}
	var photos []string
	for i := 0; i < 3; i++ {
		init := startPhoto(t, svc, user, "", img)
		send(mem, user, init, img)
		_, err := svc.CompletePhotoUpload(ctx, user, uuid.MustParse(init.VideoID))
		require.NoError(t, err)
		photos = append(photos, init.VideoID)
	}
	video, err := svc.InitUpload(ctx, user, dto.InitRequest{Title: "clip", ContentType: "video/mp4", SizeBytes: int64(len(fakeMP4()))})
	require.NoError(t, err)

	// Without the capability: one page of one item still holds the video,
	// not an empty page of photos (the filter is in the query).
	page, err := svc.List(ctx, user, "", 1, false)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, video.VideoID, page.Items[0].ID)
	require.Equal(t, model.MediaTypeVideo, page.Items[0].MediaType)
	require.Nil(t, page.NextCursor, "no further page: the photos are not there")
	for _, id := range photos {
		_, err := svc.Get(ctx, uuid.MustParse(id), &user, false)
		require.Equal(t, "not_found", apiCode(t, err), "even its owner gets 404 without the capability")
	}

	all, err := svc.List(ctx, user, "", 50, true)
	require.NoError(t, err)
	require.Len(t, all.Items, 4)
	got, err := svc.Get(ctx, uuid.MustParse(photos[0]), &user, true)
	require.NoError(t, err)
	require.Equal(t, model.MediaTypePhoto, got.MediaType)
}

func TestPhotoPostImagesAreShownOnlyOnceReady(t *testing.T) {
	ctx := context.Background()
	svc, mem := newSvc(t)
	user, viewer := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	img := upload{"image/jpeg", jpegBytes}
	init := startPhoto(t, svc, user, "", img, img)
	send(mem, user, init, img, img)
	post := uuid.MustParse(init.VideoID)
	_, err := svc.CompletePhotoUpload(ctx, user, post)
	require.NoError(t, err)
	_, err = svc.Get(ctx, post, &viewer, true)
	require.Equal(t, "not_found", apiCode(t, err), "a processing post is the owner's only")

	// What the worker of step 2c will record.
	_, err = testdb.Pool.Exec(ctx, `UPDATE video_images SET image_key = 'videos/u/p/image-' || position || '.jpg',
		thumbnail_key = 'videos/u/p/thumb-' || position || '.jpg', width = 2048, height = 1536 WHERE video_id = $1`, post)
	require.NoError(t, err)
	_, err = testdb.Pool.Exec(ctx, `UPDATE videos SET status = 'ready', thumbnail_key = 'videos/u/p/thumb-1.jpg' WHERE id = $1`, post)
	require.NoError(t, err)

	got, err := svc.Get(ctx, post, &viewer, true)
	require.NoError(t, err)
	require.Len(t, got.Images, 2)
	require.Equal(t, 1, got.Images[0].Position)
	require.Equal(t, "http://localhost:9000/poro-videos/videos/u/p/image-1.jpg", got.Images[0].URL)
	require.Equal(t, 2048, got.Images[1].Width)
	require.Nil(t, got.HLSURL, "a photo post has no HLS")
	_, err = svc.Get(ctx, post, &viewer, false)
	require.Equal(t, "not_found", apiCode(t, err), "a ready photo post stays hidden from a client without the capability")
}
