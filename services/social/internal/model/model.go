// Package model holds the social domain rules shared by the layers.
package model

import "time"

// Comment rules.
const (
	MaxCommentRunes = 1000
	ExcerptRunes    = 140
	EditWindow      = 15 * time.Minute
)

// Pagination bounds of every list endpoint.
const (
	DefaultPageSize = 20
	MaxPageSize     = 50
)

// Video projection statuses. Only ready videos accept actions.
const (
	VideoReady   = "ready"
	VideoDeleted = "deleted"
)

// EventSource identifies this service in event envelopes.
const EventSource = "social"

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
