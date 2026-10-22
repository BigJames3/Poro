package storage

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/video/internal/config"
	"github.com/poro/video/internal/model"
)

// TestS3AgainstSeaweedFS runs the S3 adapter against a SeaweedFS configured by
// infra/storage/init.sh, which checks signatures: presigned part URLs must be
// accepted as the app sends them, and only media must be publicly readable.
// infra/storage/tests/run.sh sets S3_INTEGRATION_ENDPOINT and the video
// credentials; without them the test is skipped.
func TestS3AgainstSeaweedFS(t *testing.T) {
	endpoint := os.Getenv("S3_INTEGRATION_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_INTEGRATION_ENDPOINT not set: run infra/storage/tests/run.sh")
	}
	ctx := context.Background()
	store, err := NewS3(ctx, &config.Config{
		S3Endpoint: endpoint, S3PublicEndpoint: endpoint, S3Region: "us-east-1",
		S3Bucket: "poro-videos", S3SourceBucket: "poro-video-sources", S3QuarantineBucket: "poro-quarantine",
		S3AccessKey: os.Getenv("S3_VIDEO_ACCESS_KEY_ID"), S3SecretKey: os.Getenv("S3_VIDEO_SECRET_ACCESS_KEY"),
		S3ForcePathStyle: true,
	})
	require.NoError(t, err)
	user, video := uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String()
	source := model.SourceKey(user, video, "mp4")
	client := &http.Client{Timeout: 10 * time.Second}
	anonymous := func(bucket, key string) int {
		resp, err := client.Get(endpoint + "/" + bucket + "/" + key)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	uploadID, err := store.CreateMultipart(ctx, source, "video/mp4")
	require.NoError(t, err)
	body := append([]byte("\x00\x00\x00\x18ftypisom"), bytes.Repeat([]byte{1}, 64)...)
	url, err := store.PresignPart(ctx, source, uploadID, 1, int64(len(body)))
	require.NoError(t, err)
	for _, wrong := range [][]byte{append(body, 0), body[:len(body)-1]} {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(wrong))
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode, "a part of another length than signed is refused")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "the presigned part URL is accepted")
	require.NoError(t, store.CompleteMultipart(ctx, source, uploadID, []CompletedPart{{Number: 1, ETag: resp.Header.Get("ETag")}}))

	obj, err := store.HeadSource(ctx, source)
	require.NoError(t, err)
	require.Equal(t, int64(len(body)), obj.Size)
	head, err := store.GetSourceRange(ctx, source, 4, 7)
	require.NoError(t, err)
	require.Equal(t, "ftyp", string(head))
	var got bytes.Buffer
	require.NoError(t, store.GetSource(ctx, source, &got))
	require.Equal(t, body, got.Bytes())
	require.Equal(t, http.StatusForbidden, anonymous("poro-video-sources", source), "sources are private")

	thumb := model.ThumbKey(user, video)
	require.NoError(t, store.PutMedia(ctx, thumb, "image/jpeg", bytes.NewReader([]byte("jpg"))))
	require.Equal(t, http.StatusOK, anonymous("poro-videos", thumb), "media are public")

	n, err := store.Hide(ctx, model.MediaPrefix(user, video))
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, http.StatusNotFound, anonymous("poro-videos", thumb))
	require.Equal(t, http.StatusForbidden, anonymous("poro-quarantine", thumb), "quarantine is private")
	n, err = store.Reveal(ctx, model.MediaPrefix(user, video))
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, http.StatusOK, anonymous("poro-videos", thumb))

	require.NoError(t, store.DeleteSource(ctx, source))
	_, err = store.HeadSource(ctx, source)
	require.Error(t, err)
	require.NoError(t, store.DeleteSource(ctx, source), "deleting a missing source succeeds")

	pending := model.SourceKey(user, uuid.Must(uuid.NewV7()).String(), "mp4")
	abandoned, err := store.CreateMultipart(ctx, pending, "video/mp4")
	require.NoError(t, err)
	require.NoError(t, store.AbortMultipart(ctx, pending, abandoned))
	require.NoError(t, store.AbortMultipart(ctx, pending, abandoned), "aborting an aborted upload succeeds")
}
