// Package index answers search queries. Index hides the engine: Postgres
// full-text search today, OpenSearch once ADR-0011's threshold is reached.
package index

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Page is a slice of ranked results; Offset is where it starts.
type Page[T any] struct {
	Items []T
	More  bool
}

// Author is the public profile attached to a video hit.
type Author struct {
	UserID      uuid.UUID
	Username    *string
	DisplayName *string
	AvatarURL   *string
}

// VideoHit is a searchable video.
type VideoHit struct {
	VideoID      uuid.UUID
	Title        string
	Hashtags     []string
	ThumbnailKey *string
	DurationMs   *int
	PublishedAt  time.Time
	Likes        int64
	Comments     int64
	Shares       int64
	Author       Author
}

// UserHit is an account with a username.
type UserHit struct {
	UserID      uuid.UUID
	Username    string
	DisplayName *string
	AvatarURL   *string
	IsCreator   bool
	Followers   int64
}

// HashtagHit is a tag carried by at least one searchable video.
type HashtagHit struct {
	Tag         string
	VideosCount int64
}

// Suggestions are prefix matches for the search box.
type Suggestions struct {
	Users    []UserHit
	Hashtags []HashtagHit
}

// Index ranks results for a query. offset and limit are already validated.
type Index interface {
	Videos(ctx context.Context, query string, offset, limit int) (Page[VideoHit], error)
	Users(ctx context.Context, query string, offset, limit int) (Page[UserHit], error)
	Hashtags(ctx context.Context, query string, offset, limit int) (Page[HashtagHit], error)
	Suggest(ctx context.Context, prefix string, perKind int) (Suggestions, error)
}
