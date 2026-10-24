package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
)

// Memory is an in-process object store for tests. Its three maps stand for
// the source, media and quarantine buckets.
type Memory struct {
	mu         sync.Mutex
	sources    map[string][]byte
	types      map[string]string
	objects    map[string][]byte
	quarantine map[string][]byte
	uploads    map[string]*memUpload
	seq        atomic.Int64
}

type memUpload struct {
	key   string
	parts map[int32][]byte
}

// NewMemory builds an empty store.
func NewMemory() *Memory {
	return &Memory{sources: map[string][]byte{}, types: map[string]string{}, objects: map[string][]byte{}, quarantine: map[string][]byte{}, uploads: map[string]*memUpload{}}
}

func (m *Memory) CreateMultipart(_ context.Context, key, _ string) (string, error) {
	id := fmt.Sprintf("upload-%d", m.seq.Add(1))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploads[id] = &memUpload{key: key, parts: map[int32][]byte{}}
	return id, nil
}

func (m *Memory) PresignPart(_ context.Context, _, uploadID string, part int32, size int64) (string, error) {
	return fmt.Sprintf("memory://%s/%d?size=%d", uploadID, part, size), nil
}

func (m *Memory) PresignPut(_ context.Context, key, contentType string, size int64) (string, map[string]string, error) {
	return fmt.Sprintf("memory://put/%s?size=%d", key, size), map[string]string{
		"Content-Type": contentType, "Content-Length": fmt.Sprint(size),
	}, nil
}

// PutObject stores a source as a presigned PUT would, with its Content-Type
// (tests simulate the client PUT; unlike real storage, nothing is checked).
func (m *Memory) PutObject(key, contentType string, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(body))
	copy(cp, body)
	m.sources[key] = cp
	m.types[key] = contentType
}

// PutPart stores bytes for a presigned part (tests simulate the client PUT).
func (m *Memory) PutPart(uploadID string, part int32, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.uploads[uploadID]; ok {
		cp := make([]byte, len(body))
		copy(cp, body)
		u.parts[part] = cp
	}
}

func (m *Memory) CompleteMultipart(_ context.Context, key, uploadID string, parts []CompletedPart) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.uploads[uploadID]
	if !ok {
		return fmt.Errorf("unknown upload %s", uploadID)
	}
	var buf bytes.Buffer
	for _, p := range parts {
		chunk, ok := u.parts[p.Number]
		if !ok {
			return fmt.Errorf("missing part %d", p.Number)
		}
		buf.Write(chunk)
	}
	m.sources[key] = buf.Bytes()
	delete(m.uploads, uploadID)
	return nil
}

func (m *Memory) AbortMultipart(_ context.Context, _, uploadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.uploads, uploadID)
	return nil
}

// HasUpload reports whether a multipart upload is still open.
func (m *Memory) HasUpload(uploadID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.uploads[uploadID]
	return ok
}

func (m *Memory) HeadSource(_ context.Context, key string) (Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.sources[key]
	if !ok {
		return Object{}, fmt.Errorf("head %s: %w", key, ErrMissing)
	}
	return Object{Size: int64(len(b)), ContentType: m.types[key]}, nil
}

func (m *Memory) GetSourceRange(_ context.Context, key string, start, end int64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.sources[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	if start < 0 {
		start = 0
	}
	if end >= int64(len(b)) {
		end = int64(len(b)) - 1
	}
	if start > end {
		return nil, nil
	}
	out := make([]byte, end-start+1)
	copy(out, b[start:end+1])
	return out, nil
}

func (m *Memory) GetSource(_ context.Context, key string, dest io.Writer) error {
	m.mu.Lock()
	b, ok := m.sources[key]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("not found: %s", key)
	}
	_, err := dest.Write(b)
	return err
}

func (m *Memory) PutMedia(_ context.Context, key, _ string, body io.Reader) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = raw
	return nil
}

func (m *Memory) DeleteSource(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sources, key)
	delete(m.types, key)
	return nil
}

// PutSource stores a source object directly (tests simulate a finished upload).
func (m *Memory) PutSource(key string, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(body))
	copy(cp, body)
	m.sources[key] = cp
}

// HasSource reports whether key sits in the source bucket.
func (m *Memory) HasSource(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.sources[key]
	return ok
}

// Bytes returns a copy of a media object for assertions.
func (m *Memory) Bytes(key string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	b := m.objects[key]
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func (m *Memory) Hide(_ context.Context, prefix string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return moveObjects(m.objects, m.quarantine, prefix), nil
}

func (m *Memory) Reveal(_ context.Context, prefix string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return moveObjects(m.quarantine, m.objects, prefix), nil
}

// Quarantined reports whether key sits in the quarantine bucket.
func (m *Memory) Quarantined(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.quarantine[key]
	return ok
}

// Has reports whether key sits in the public media bucket.
func (m *Memory) Has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[key]
	return ok
}

func moveObjects(from, to map[string][]byte, prefix string) int {
	moved := 0
	for key, body := range from {
		if strings.HasPrefix(key, prefix) {
			to[key] = body
			delete(from, key)
			moved++
		}
	}
	return moved
}
