package storage

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryMultipart(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	id, err := m.CreateMultipart(ctx, "k", "video/mp4")
	require.NoError(t, err)
	url, err := m.PresignPart(ctx, "k", id, 1, 5)
	require.NoError(t, err)
	require.Contains(t, url, id)
	require.Contains(t, url, "size=5")
	m.PutPart(id, 1, []byte("hello"))
	require.NoError(t, m.CompleteMultipart(ctx, "k", id, []CompletedPart{{Number: 1, ETag: "e"}}))
	require.True(t, m.HasSource("k"))
	require.False(t, m.Has("k"), "a source never lands in the public bucket")
	obj, err := m.HeadSource(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, int64(5), obj.Size)
	got, err := m.GetSourceRange(ctx, "k", 0, 4)
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))
	var buf bytes.Buffer
	require.NoError(t, m.GetSource(ctx, "k", &buf))
	require.Equal(t, "hello", buf.String())
	require.NoError(t, m.DeleteSource(ctx, "k"))
	_, err = m.HeadSource(ctx, "k")
	require.Error(t, err)
}

func TestMemoryAbort(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	id, err := m.CreateMultipart(ctx, "k", "video/mp4")
	require.NoError(t, err)
	require.NoError(t, m.AbortMultipart(ctx, "k", id))
	require.Error(t, m.CompleteMultipart(ctx, "k", id, nil))
}

func TestMemoryPutMedia(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	require.NoError(t, m.PutMedia(ctx, "a", "text/plain", bytes.NewReader([]byte("z"))))
	require.Equal(t, []byte("z"), m.Bytes("a"))
	require.False(t, m.HasSource("a"))
	m.PutSource("s", []byte("src"))
	require.True(t, m.HasSource("s"))
	require.False(t, m.Has("s"))
}

func TestMemoryListParts(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	id, err := m.CreateMultipart(ctx, "k", "video/mp4")
	require.NoError(t, err)
	m.PutPart(id, 3, []byte("ccc"))
	m.PutPart(id, 1, []byte("a"))
	parts, err := m.ListParts(ctx, "k", id)
	require.NoError(t, err)
	require.Equal(t, []UploadedPart{{Number: 1, ETag: "etag-1", Size: 1}, {Number: 3, ETag: "etag-3", Size: 3}}, parts)
	require.NoError(t, m.AbortMultipart(ctx, "k", id))
	_, err = m.ListParts(ctx, "k", id)
	require.ErrorIs(t, err, ErrMissing)
}
