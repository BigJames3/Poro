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
	url, err := m.PresignPart(ctx, "k", id, 1)
	require.NoError(t, err)
	require.Contains(t, url, id)
	m.PutPart(id, 1, []byte("hello"))
	require.NoError(t, m.CompleteMultipart(ctx, "k", id, []CompletedPart{{Number: 1, ETag: "e"}}))
	obj, err := m.Head(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, int64(5), obj.Size)
	got, err := m.GetRange(ctx, "k", 0, 4)
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))
	var buf bytes.Buffer
	require.NoError(t, m.Get(ctx, "k", &buf))
	require.Equal(t, "hello", buf.String())
	require.NoError(t, m.Delete(ctx, "k"))
	_, err = m.Head(ctx, "k")
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

func TestMemoryPut(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	require.NoError(t, m.Put(ctx, "a", "text/plain", bytes.NewReader([]byte("z"))))
	require.Equal(t, []byte("z"), m.Bytes("a"))
}
