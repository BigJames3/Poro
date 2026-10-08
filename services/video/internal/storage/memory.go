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

// Memory is an in-process object store for tests.
type Memory struct {
	mu         sync.Mutex
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
	return &Memory{objects: map[string][]byte{}, quarantine: map[string][]byte{}, uploads: map[string]*memUpload{}}
}

func (m *Memory) CreateMultipart(_ context.Context, key, _ string) (string, error) {
	id := fmt.Sprintf("upload-%d", m.seq.Add(1))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploads[id] = &memUpload{key: key, parts: map[int32][]byte{}}
	return id, nil
}

func (m *Memory) PresignPart(_ context.Context, _, uploadID string, part int32) (string, error) {
	return fmt.Sprintf("memory://%s/%d", uploadID, part), nil
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
	m.objects[key] = buf.Bytes()
	delete(m.uploads, uploadID)
	return nil
}

func (m *Memory) AbortMultipart(_ context.Context, _, uploadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.uploads, uploadID)
	return nil
}

func (m *Memory) Head(_ context.Context, key string) (Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return Object{}, fmt.Errorf("not found: %s", key)
	}
	return Object{Size: int64(len(b))}, nil
}

func (m *Memory) GetRange(_ context.Context, key string, start, end int64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
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

func (m *Memory) Get(_ context.Context, key string, dest io.Writer) error {
	m.mu.Lock()
	b, ok := m.objects[key]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("not found: %s", key)
	}
	_, err := dest.Write(b)
	return err
}

func (m *Memory) Put(_ context.Context, key, _ string, body io.Reader) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = raw
	return nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// Bytes returns a copy of an object for assertions.
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

// Has reports whether key sits in the public bucket.
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
