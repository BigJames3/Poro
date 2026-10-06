package storage

import (
	"context"
	"io"
)

// CompletedPart is one uploaded multipart part.
type CompletedPart struct {
	Number int32
	ETag   string
}

// Object holds HeadObject metadata.
type Object struct {
	Size int64
}

// Storage is the object-store operations the video pipeline needs.
type Storage interface {
	CreateMultipart(ctx context.Context, key, contentType string) (uploadID string, err error)
	PresignPart(ctx context.Context, key, uploadID string, part int32) (url string, err error)
	CompleteMultipart(ctx context.Context, key, uploadID string, parts []CompletedPart) error
	AbortMultipart(ctx context.Context, key, uploadID string) error
	Head(ctx context.Context, key string) (Object, error)
	GetRange(ctx context.Context, key string, start, end int64) ([]byte, error)
	Get(ctx context.Context, key string, dest io.Writer) error
	Put(ctx context.Context, key, contentType string, body io.Reader) error
	Delete(ctx context.Context, key string) error
}
