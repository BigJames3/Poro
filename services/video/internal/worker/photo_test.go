package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/testdb"
	"github.com/poro/video/internal/worker"
)

func imageFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "imaging", "testdata", name))
	require.NoError(t, err)
	return raw
}

type stored struct {
	contentType string
	body        []byte
}

// seedPhoto creates a processing photo post whose sources hold the given
// bytes, and returns its ids and the uploaded event. declared is what the
// client declared at creation; body is what sits in storage now.
func seedPhoto(t *testing.T, mem *storage.Memory, declared []stored, bodies [][]byte) (uuid.UUID, uuid.UUID, events.Envelope) {
	t.Helper()
	testdb.Available(t)
	ctx := context.Background()
	user, id := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	repo := repository.New(testdb.Pool)
	now := time.Now().UTC()
	images := make([]model.Image, len(declared))
	var total int64
	for i, d := range declared {
		images[i] = model.Image{Position: i + 1, ContentType: d.contentType, SizeBytes: int64(len(d.body)),
			SourceKey: model.ImageSourceKey(user.String(), id.String(), i+1)}
		total += int64(len(d.body))
		mem.PutObject(images[i].SourceKey, d.contentType, bodies[i])
	}
	require.NoError(t, repo.CreatePhoto(ctx, &model.Video{
		ID: id.String(), UserID: user.String(), Description: "Marché #Treichville #wax", Status: model.StatusUploading,
		ContentType: model.PhotoContentType, SizeBytes: total, SourceKey: model.MediaPrefix(user.String(), id.String()),
		CreatedAt: now, UpdatedAt: now,
	}, images))
	tx, err := testdb.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.MarkProcessing(ctx, tx, id, total))
	require.NoError(t, tx.Commit(ctx))
	env, err := events.New(events.TypeVideoUploaded, 1, "video", id.String(), events.VideoUploadedV1{
		VideoID: id.String(), UserID: user.String(), SourceKey: model.MediaPrefix(user.String(), id.String()),
		ContentType: model.PhotoContentType, SizeBytes: total, UploadedAt: now, MediaType: model.MediaTypePhoto,
	}, now)
	require.NoError(t, err)
	return user, id, env
}

func valid(t *testing.T) ([]stored, [][]byte) {
	jpg, png, webp := imageFixture(t, "jpeg_o6.jpg"), imageFixture(t, "png_alpha.png"), imageFixture(t, "webp_o6.webp")
	return []stored{{"image/jpeg", jpg}, {"image/png", png}, {"image/webp", webp}}, [][]byte{jpg, png, webp}
}

func outboxEvent(t *testing.T, topic, key string) *events.Envelope {
	t.Helper()
	var raw []byte
	err := testdb.Pool.QueryRow(context.Background(), `SELECT payload FROM outbox_events WHERE topic = $1 AND event_key = $2`,
		topic, key).Scan(&raw)
	if err != nil {
		return nil
	}
	var env events.Envelope
	require.NoError(t, json.Unmarshal(raw, &env))
	return &env
}

func TestPhotoPostIsProcessedAndReady(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	declared, bodies := valid(t)
	user, id, env := seedPhoto(t, mem, declared, bodies)
	repo := repository.New(testdb.Pool)

	require.NoError(t, worker.New(testdb.Pool, repo, mem, ffCalled{t}, zap.NewNop()).Handle(ctx, env))

	row, err := repo.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, model.StatusReady, row.Status)
	require.Equal(t, model.ImageThumbKey(user.String(), id.String(), 1), *row.ThumbnailKey)
	require.Nil(t, row.HLSKey, "no HLS for a photo post")
	images, err := repo.Images(ctx, id.String())
	require.NoError(t, err)
	require.Len(t, images, 3)
	require.Equal(t, 32, *images[0].Width, "the EXIF orientation 6 of image 1 is applied")
	require.Equal(t, 64, *images[0].Height)
	for _, img := range images {
		require.True(t, mem.Has(*img.ImageKey), "image %d is public", img.Position)
		require.True(t, mem.Has(*img.ThumbnailKey), "thumbnail %d is public", img.Position)
		out := mem.Bytes(*img.ImageKey)
		_, err := jpeg.Decode(bytes.NewReader(out))
		require.NoError(t, err, "re-encoded as JPEG")
		require.False(t, bytes.Contains(out, []byte("Exif")), "no EXIF, so no GPS, in image %d", img.Position)
		require.False(t, mem.HasSource(img.SourceKey), "the original %d is deleted", img.Position)
	}

	ready := outboxEvent(t, events.TypeVideoReady, id.String())
	require.NotNil(t, ready)
	var data events.VideoReadyV1
	require.NoError(t, ready.DecodeData(&data))
	require.Equal(t, model.MediaTypePhoto, data.MediaType)
	require.Len(t, data.Images, 3)
	require.Equal(t, *images[2].ImageKey, data.Images[2].ImageKey)
	require.Empty(t, data.HLSKey)
	require.Equal(t, []string{"treichville", "wax"}, data.Hashtags)
	require.Empty(t, data.Title)

	require.NoError(t, worker.New(testdb.Pool, repo, mem, ffCalled{t}, zap.NewNop()).Handle(ctx, env), "a redelivered event is a no-op")
}

func TestPhotoPostWithOneInvalidImageFailsWhole(t *testing.T) {
	ctx := context.Background()
	jpg := imageFixture(t, "jpeg_o1.jpg")
	png := imageFixture(t, "png_o6.png")
	big := append(append([]byte{}, jpg...), make([]byte, model.MaxImageBytes)...)
	// A PNG of exactly the declared JPEG size: the size check passes, the content does not.
	pngSized := append(append([]byte{}, png...), make([]byte, len(jpg)-len(png))...)
	cases := map[string]struct {
		declared []stored
		bodies   [][]byte
	}{
		// complete checked image 2, then it was replaced through the still valid URL.
		"replaced by a png after complete": {[]stored{{"image/jpeg", jpg}, {"image/jpeg", jpg}}, [][]byte{jpg, pngSized}},
		"replaced by a larger file":        {[]stored{{"image/jpeg", jpg}, {"image/jpeg", jpg}}, [][]byte{jpg, big}},
		"truncated":                        {[]stored{{"image/jpeg", jpg}, {"image/jpeg", jpg}}, [][]byte{jpg, append(jpg[:len(jpg)/2:len(jpg)/2], make([]byte, len(jpg)-len(jpg)/2)...)}},
	}
	for name, c := range cases {
		mem := storage.NewMemory()
		repo := repository.New(testdb.Pool)
		user, id, env := seedPhoto(t, mem, c.declared, c.bodies)
		counting := &countingPuts{Memory: mem}

		require.NoError(t, worker.New(testdb.Pool, repo, counting, ffCalled{t}, zap.NewNop()).Handle(ctx, env), name)
		require.Zero(t, counting.puts, "%s: nothing is written publicly, not even for a moment, before every image passed", name)

		row, err := repo.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, model.StatusFailed, row.Status, name)
		require.Equal(t, events.VideoFailInvalid, *row.FailureCode, name)
		for pos := 1; pos <= 2; pos++ {
			require.False(t, mem.Has(model.ImageKey(user.String(), id.String(), pos)), "%s: image %d never public", name, pos)
			require.False(t, mem.Has(model.ImageThumbKey(user.String(), id.String(), pos)), "%s: thumbnail %d never public", name, pos)
			require.False(t, mem.HasSource(model.ImageSourceKey(user.String(), id.String(), pos)), "%s: source %d deleted", name, pos)
		}
		require.Nil(t, outboxEvent(t, events.TypeVideoReady, id.String()), name)
		require.NotNil(t, outboxEvent(t, events.TypeVideoFailed, id.String()), name)
	}
}

func TestPhotoPostStorageOutageQuarantinesWhatWasWritten(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	declared, bodies := valid(t)
	user, id, env := seedPhoto(t, mem, declared, bodies)
	// The first image and thumbnail are written, then storage refuses every write.
	store := &failAfterPuts{Memory: mem, allowed: 2}
	h := worker.New(testdb.Pool, repository.New(testdb.Pool), store, ffCalled{t}, zap.NewNop())
	h.SetRetryBackoff(time.Millisecond)

	require.NoError(t, h.Handle(ctx, env))

	row, err := repository.New(testdb.Pool).Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, model.StatusFailed, row.Status)
	require.Equal(t, events.VideoFailTranscode, *row.FailureCode)
	first := model.ImageKey(user.String(), id.String(), 1)
	require.False(t, mem.Has(first), "the image written before the outage is not left public")
	require.True(t, mem.Quarantined(first), "it is quarantined")
}

func TestPhotoPostDeletedWhileProcessingStaysPrivate(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	declared, bodies := valid(t)
	user, id, env := seedPhoto(t, mem, declared, bodies)
	repo := repository.New(testdb.Pool)
	tx, err := testdb.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = repo.SoftDelete(ctx, tx, id, user)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	require.NoError(t, worker.New(testdb.Pool, repo, mem, ffCalled{t}, zap.NewNop()).Handle(ctx, env))
	require.Nil(t, outboxEvent(t, events.TypeVideoReady, id.String()), "never announced ready")
	require.False(t, mem.Has(model.ImageKey(user.String(), id.String(), 1)))
}

func TestPhotoPostMarkedDeletedMidProcessingIsHidden(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	declared, bodies := valid(t)
	user, id, env := seedPhoto(t, mem, declared, bodies)
	repo := repository.New(testdb.Pool)
	// The owner deletes the post while the worker writes its images.
	store := &deleteDuringPut{Memory: mem, user: user, id: id, repo: repo}
	require.NoError(t, worker.New(testdb.Pool, repo, store, ffCalled{t}, zap.NewNop()).Handle(ctx, env))

	require.Nil(t, outboxEvent(t, events.TypeVideoReady, id.String()))
	for pos := 1; pos <= 3; pos++ {
		require.False(t, mem.Has(model.ImageKey(user.String(), id.String(), pos)), "image %d not public", pos)
		require.False(t, mem.HasSource(model.ImageSourceKey(user.String(), id.String(), pos)), "source %d deleted", pos)
	}
}

func TestAVideoAnnouncedAsAPhotoIsRefused(t *testing.T) {
	ctx := context.Background()
	user, id := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	seedProcessing(t, user, id)
	env, err := events.New(events.TypeVideoUploaded, 1, "video", id.String(), events.VideoUploadedV1{
		VideoID: id.String(), UserID: user.String(), SourceKey: model.SourceKey(user.String(), id.String(), "mp4"),
		ContentType: "video/mp4", SizeBytes: 32, UploadedAt: time.Now(), MediaType: model.MediaTypePhoto,
	}, time.Now())
	require.NoError(t, err)
	err = worker.New(testdb.Pool, repository.New(testdb.Pool), storage.NewMemory(), ffCalled{t}, zap.NewNop()).Handle(ctx, env)
	require.True(t, kafka.IsPermanent(err), "the row decides: a video is never processed as photos")
	row, err := repository.New(testdb.Pool).Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, model.StatusProcessing, row.Status)
}

// failAfterPuts accepts allowed media writes, then fails every one.
type failAfterPuts struct {
	*storage.Memory
	allowed int
}

func (s *failAfterPuts) PutMedia(ctx context.Context, key, contentType string, body io.Reader) error {
	if s.allowed == 0 {
		return errStorageDown
	}
	s.allowed--
	return s.Memory.PutMedia(ctx, key, contentType, body)
}

// deleteDuringPut deletes the post at the first media write.
type deleteDuringPut struct {
	*storage.Memory
	user, id uuid.UUID
	repo     *repository.Videos
	done     bool
}

func (s *deleteDuringPut) PutMedia(ctx context.Context, key, contentType string, body io.Reader) error {
	if !s.done {
		s.done = true
		tx, err := testdb.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := s.repo.SoftDelete(ctx, tx, s.id, s.user); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return s.Memory.PutMedia(ctx, key, contentType, body)
}

var errStorageDown = errors.New("503 slow down")

// countingPuts counts the public writes.
type countingPuts struct {
	*storage.Memory
	puts int
}

func (s *countingPuts) PutMedia(ctx context.Context, key, contentType string, body io.Reader) error {
	s.puts++
	return s.Memory.PutMedia(ctx, key, contentType, body)
}
