// Package model holds the feed rules shared by the layers.
package model

import "time"

// Page sizes of the feed endpoints.
const (
	DefaultPageSize = 10
	MaxPageSize     = 30
)

// Trending window and recompute period.
const (
	TrendingWindow   = 72 * time.Hour
	TrendingInterval = 5 * time.Minute
)

// For You session.
const (
	SessionSize = 200
	SessionTTL  = 30 * time.Minute
)

// Discovery: recent videos of small authors.
const (
	DiscoveryMaxAge       = 48 * time.Hour
	DiscoveryMaxFollowers = 1000
)

// Candidate pool sizes read to build a For You session.
const (
	FollowingCandidates = 200
	TrendingCandidates  = 200
	DiscoveryCandidates = 100
)

// FirstPageTTL is how long page 1 of following and trending stays cached.
const FirstPageTTL = 60 * time.Second

// ModerationApproved is the only status shown in feeds. Until the moderation
// service exists every projected video is approved.
const ModerationApproved = "approved"

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
