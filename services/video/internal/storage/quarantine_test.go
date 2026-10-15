package storage

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/require"

	"github.com/poro/video/internal/config"
)

func TestMemoryHideAndReveal(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	for _, key := range []string{"videos/u/v/hls/master.m3u8", "videos/u/v/thumb.jpg", "videos/u/w/thumb.jpg"} {
		require.NoError(t, m.PutMedia(ctx, key, "x", bytes.NewReader([]byte(key))))
	}
	n, err := m.Hide(ctx, "videos/u/v/")
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.False(t, m.Has("videos/u/v/thumb.jpg"))
	require.True(t, m.Quarantined("videos/u/v/thumb.jpg"))
	require.True(t, m.Has("videos/u/w/thumb.jpg"), "another video is untouched")

	n, err = m.Hide(ctx, "videos/u/v/")
	require.NoError(t, err)
	require.Zero(t, n, "hiding twice moves nothing")

	n, err = m.Reveal(ctx, "videos/u/v/")
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.Equal(t, []byte("videos/u/v/thumb.jpg"), m.Bytes("videos/u/v/thumb.jpg"))
}

// newFakeS3 runs the real S3 client against an in-process S3 server with the
// three video buckets.
func newFakeS3(t *testing.T) *S3 {
	t.Helper()
	ctx := context.Background()
	srv := httptest.NewServer(gofakes3.New(s3mem.New()).Server())
	t.Cleanup(srv.Close)
	cfg := &config.Config{
		S3Endpoint: srv.URL, S3PublicEndpoint: srv.URL, S3Region: "us-east-1",
		S3Bucket: "poro-videos", S3SourceBucket: "poro-video-sources", S3QuarantineBucket: "poro-quarantine",
		S3AccessKey: "k", S3SecretKey: "s", S3ForcePathStyle: true,
	}
	store, err := NewS3(ctx, cfg)
	require.NoError(t, err)
	for _, bucket := range []string{"poro-videos", "poro-video-sources", "poro-quarantine"} {
		_, err := store.internal.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
		require.NoError(t, err)
	}
	return store
}

func objectBody(t *testing.T, store *S3, bucket, key string) (string, bool) {
	t.Helper()
	obj, err := store.internal.GetObject(context.Background(), &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return "", false
	}
	defer func() { _ = obj.Body.Close() }()
	body, err := io.ReadAll(obj.Body)
	require.NoError(t, err)
	return string(body), true
}

// TestS3HideAndReveal moves media between the public and quarantine buckets.
func TestS3HideAndReveal(t *testing.T) {
	ctx := context.Background()
	store := newFakeS3(t)

	keys := []string{"videos/u/v/thumb.jpg", "videos/u/v/hls/master.m3u8", "videos/u/v/hls/360p/seg 000.ts"}
	// More objects than one list page, to exercise pagination.
	for i := range 1005 {
		keys = append(keys, "videos/u/v/hls/720p/"+string(rune('a'+i%26))+"-"+strconv.Itoa(i)+".ts")
	}
	for _, key := range keys {
		require.NoError(t, store.PutMedia(ctx, key, "application/octet-stream", bytes.NewReader([]byte(key))))
	}
	require.NoError(t, store.PutMedia(ctx, "videos/u/w/thumb.jpg", "image/jpeg", bytes.NewReader([]byte("other"))))

	n, err := store.Hide(ctx, "videos/u/v/")
	require.NoError(t, err)
	require.Equal(t, len(keys), n)
	_, ok := objectBody(t, store, "poro-videos", "videos/u/v/thumb.jpg")
	require.False(t, ok, "nothing left in the public bucket")
	body, ok := objectBody(t, store, "poro-quarantine", "videos/u/v/hls/360p/seg 000.ts")
	require.True(t, ok)
	require.Equal(t, "videos/u/v/hls/360p/seg 000.ts", body)
	_, ok = objectBody(t, store, "poro-videos", "videos/u/w/thumb.jpg")
	require.True(t, ok, "another video is untouched")

	n, err = store.Reveal(ctx, "videos/u/v/")
	require.NoError(t, err)
	require.Equal(t, len(keys), n)
	_, ok = objectBody(t, store, "poro-videos", "videos/u/v/thumb.jpg")
	require.True(t, ok)

	_, err = (&S3{internal: store.internal, media: "missing", quarantine: "poro-quarantine"}).Hide(ctx, "videos/")
	require.Error(t, err)
}

// TestS3SourcesStayPrivate checks that every source operation targets the
// private source bucket and media writes target the public one.
func TestS3SourcesStayPrivate(t *testing.T) {
	ctx := context.Background()
	store := newFakeS3(t)
	key := "videos/u/v/source.mp4"

	uploadID, err := store.CreateMultipart(ctx, key, "video/mp4")
	require.NoError(t, err)
	url, err := store.PresignPart(ctx, key, uploadID, 1)
	require.NoError(t, err)
	require.Contains(t, url, "/poro-video-sources/"+key)
	part, err := store.internal.UploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String("poro-video-sources"), Key: aws.String(key), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(1), Body: bytes.NewReader([]byte("ftypsource")),
	})
	require.NoError(t, err)
	require.NoError(t, store.CompleteMultipart(ctx, key, uploadID, []CompletedPart{{Number: 1, ETag: aws.ToString(part.ETag)}}))

	_, ok := objectBody(t, store, "poro-videos", key)
	require.False(t, ok, "the source never reaches the public bucket")
	obj, err := store.HeadSource(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(10), obj.Size)
	head, err := store.GetSourceRange(ctx, key, 0, 3)
	require.NoError(t, err)
	require.Equal(t, "ftyp", string(head))
	var buf bytes.Buffer
	require.NoError(t, store.GetSource(ctx, key, &buf))
	require.Equal(t, "ftypsource", buf.String())

	require.NoError(t, store.PutMedia(ctx, "videos/u/v/thumb.jpg", "image/jpeg", bytes.NewReader([]byte("jpg"))))
	_, ok = objectBody(t, store, "poro-videos", "videos/u/v/thumb.jpg")
	require.True(t, ok)
	_, ok = objectBody(t, store, "poro-video-sources", "videos/u/v/thumb.jpg")
	require.False(t, ok)

	require.NoError(t, store.DeleteSource(ctx, key))
	_, err = store.HeadSource(ctx, key)
	require.Error(t, err)

	other, err := store.CreateMultipart(ctx, "videos/u/w/source.webm", "video/webm")
	require.NoError(t, err)
	require.NoError(t, store.AbortMultipart(ctx, "videos/u/w/source.webm", other))
}

// TestS3MoveLegacySources moves the sources stored before the source bucket
// existed, from the public and quarantine buckets, and nothing else.
func TestS3MoveLegacySources(t *testing.T) {
	ctx := context.Background()
	store := newFakeS3(t)
	const (
		user  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b"
		ready = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6c"
		gone  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6d"
	)
	put := func(bucket, key string) {
		_, err := store.internal.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: bytes.NewReader([]byte(key))})
		require.NoError(t, err)
	}
	readySource := "videos/" + user + "/" + ready + "/source.mp4"
	goneSource := "videos/" + user + "/" + gone + "/source.mov"
	thumb := "videos/" + user + "/" + ready + "/thumb.jpg"
	lookalike := "videos/" + user + "/" + ready + "/hls/source.mp4"
	put("poro-videos", readySource)
	put("poro-videos", thumb)
	put("poro-videos", lookalike)
	put("poro-quarantine", goneSource)

	n, err := store.MoveLegacySources(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	for _, key := range []string{readySource, goneSource} {
		body, ok := objectBody(t, store, "poro-video-sources", key)
		require.True(t, ok, key)
		require.Equal(t, key, body)
	}
	_, ok := objectBody(t, store, "poro-videos", readySource)
	require.False(t, ok)
	_, ok = objectBody(t, store, "poro-quarantine", goneSource)
	require.False(t, ok)
	for _, key := range []string{thumb, lookalike} {
		_, ok := objectBody(t, store, "poro-videos", key)
		require.True(t, ok, "media stay public: "+key)
	}

	n, err = store.MoveLegacySources(ctx)
	require.NoError(t, err)
	require.Zero(t, n, "a second run moves nothing")

	_, err = (&S3{internal: store.internal, media: "missing", sources: "poro-video-sources", quarantine: "poro-quarantine"}).MoveLegacySources(ctx)
	require.Error(t, err)
}
