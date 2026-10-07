// Package dto defines the JSON bodies of the feed API. Items are kept small
// for slow mobile networks: the description is fetched from the video API.
package dto

import "time"

// Page is one page of a feed. NextCursor is null on the last page.
type Page struct {
	Items      []Item  `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// Item is one video of a feed. liked_by_me is not part of v1.
type Item struct {
	VideoID      string    `json:"video_id"`
	Title        string    `json:"title"`
	Hashtags     []string  `json:"hashtags"`
	DurationMs   int       `json:"duration_ms"`
	PublishedAt  time.Time `json:"published_at"`
	ThumbnailURL string    `json:"thumbnail_url"`
	HLSURL       string    `json:"hls_url"`
	Stats        Stats     `json:"stats"`
	Author       Author    `json:"author"`
}

// Stats are the counters of a video as last seen by the feed.
type Stats struct {
	Likes    int64 `json:"likes"`
	Comments int64 `json:"comments"`
	Shares   int64 `json:"shares"`
}

// Author is the public profile of the video author; fields are null until
// the feed receives the author's first poro.user.profile.updated.
type Author struct {
	ID          string  `json:"id"`
	Username    *string `json:"username"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}
