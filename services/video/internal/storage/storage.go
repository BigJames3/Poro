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

// Storage is the object-store operations the video pipeline needs. It spans
// three buckets: the private source bucket holds the original uploads, the
// public media bucket holds HLS renditions and thumbnails, and the private
// quarantine bucket holds the media of removed and deleted videos.
type Storage interface {
	// Multipart operations upload a source object. AbortMultipart and
	// DeleteSource succeed when the upload or the object is already gone.
	CreateMultipart(ctx context.Context, key, contentType string) (uploadID string, err error)
	PresignPart(ctx context.Context, key, uploadID string, part int32) (url string, err error)
	CompleteMultipart(ctx context.Context, key, uploadID string, parts []CompletedPart) error
	AbortMultipart(ctx context.Context, key, uploadID string) error
	HeadSource(ctx context.Context, key string) (Object, error)
	GetSourceRange(ctx context.Context, key string, start, end int64) ([]byte, error)
	GetSource(ctx context.Context, key string, dest io.Writer) error
	DeleteSource(ctx context.Context, key string) error
	// PutMedia writes a public HLS or thumbnail object.
	PutMedia(ctx context.Context, key, contentType string, body io.Reader) error
	// Hide moves every media object under prefix from the public bucket to
	// the quarantine bucket; Reveal moves them back. Both are idempotent and
	// return how many objects moved.
	Hide(ctx context.Context, prefix string) (int, error)
	Reveal(ctx context.Context, prefix string) (int, error)
}
