// Package model holds the search domain rules shared by the layers.
package model

// Result kinds of GET /api/v1/search.
const (
	TypeVideos   = "videos"
	TypeUsers    = "users"
	TypeHashtags = "hashtags"
)

// Query and paging bounds.
const (
	MaxQueryRunes   = 100
	DefaultPageSize = 20
	MaxPageSize     = 50
	// MaxResults caps how deep a client can page: relevance past it is noise.
	MaxResults     = 500
	SuggestPerKind = 5
)

// Moderation statuses of a video; only approved videos are searchable.
const (
	ModerationApproved = "approved"
	ModerationRejected = "rejected"
)

// PageSize clamps a requested page size to [1, MaxPageSize]; 0 means the default.
func PageSize(requested int) int {
	switch {
	case requested <= 0:
		return DefaultPageSize
	case requested > MaxPageSize:
		return MaxPageSize
	default:
		return requested
	}
}
