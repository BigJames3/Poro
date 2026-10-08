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
		require.NoError(t, m.Put(ctx, key, "x", bytes.NewReader([]byte(key))))
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

// TestS3HideAndReveal runs the real S3 client against an in-process S3 server.
func TestS3HideAndReveal(t *testing.T) {
	ctx := context.Background()
	backend := s3mem.New()
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	defer srv.Close()
	cfg := &config.Config{
		S3Endpoint: srv.URL, S3PublicEndpoint: srv.URL, S3Region: "us-east-1",
		S3Bucket: "poro-videos", S3QuarantineBucket: "poro-quarantine",
		S3AccessKey: "k", S3SecretKey: "s", S3ForcePathStyle: true,
	}
	store, err := NewS3(ctx, cfg)
	require.NoError(t, err)
	for _, bucket := range []string{"poro-videos", "poro-quarantine"} {
		_, err := store.internal.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
		require.NoError(t, err)
	}

	keys := []string{"videos/u/v/thumb.jpg", "videos/u/v/hls/master.m3u8", "videos/u/v/hls/360p/seg 000.ts"}
	// More objects than one list page, to exercise pagination.
	for i := range 1005 {
		keys = append(keys, "videos/u/v/hls/720p/"+string(rune('a'+i%26))+"-"+strconv.Itoa(i)+".ts")
	}
	for _, key := range keys {
		require.NoError(t, store.Put(ctx, key, "application/octet-stream", bytes.NewReader([]byte(key))))
	}
	require.NoError(t, store.Put(ctx, "videos/u/w/thumb.jpg", "image/jpeg", bytes.NewReader([]byte("other"))))

	n, err := store.Hide(ctx, "videos/u/v/")
	require.NoError(t, err)
	require.Equal(t, len(keys), n)
	_, err = store.Head(ctx, "videos/u/v/thumb.jpg")
	require.Error(t, err, "nothing left in the public bucket")
	obj, err := store.internal.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("poro-quarantine"), Key: aws.String("videos/u/v/hls/360p/seg 000.ts"),
	})
	require.NoError(t, err)
	body, err := io.ReadAll(obj.Body)
	require.NoError(t, err)
	require.Equal(t, "videos/u/v/hls/360p/seg 000.ts", string(body))
	_, err = store.Head(ctx, "videos/u/w/thumb.jpg")
	require.NoError(t, err, "another video is untouched")

	n, err = store.Reveal(ctx, "videos/u/v/")
	require.NoError(t, err)
	require.Equal(t, len(keys), n)
	_, err = store.Head(ctx, "videos/u/v/thumb.jpg")
	require.NoError(t, err)

	_, err = (&S3{internal: store.internal, bucket: "missing", quarantine: "poro-quarantine"}).Hide(ctx, "videos/")
	require.Error(t, err)
}
