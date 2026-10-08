// Package dto holds the JSON shapes of the search API.
package dto

import "time"

// Results is one page of GET /api/v1/search.
type Results struct {
	Type       string  `json:"type"`
	Items      any     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

type Author struct {
	UserID      string  `json:"user_id"`
	Username    *string `json:"username"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

type Stats struct {
	Likes    int64 `json:"likes"`
	Comments int64 `json:"comments"`
	Shares   int64 `json:"shares"`
}

type Video struct {
	VideoID      string    `json:"video_id"`
	Title        string    `json:"title"`
	Hashtags     []string  `json:"hashtags"`
	ThumbnailURL *string   `json:"thumbnail_url"`
	DurationMs   *int      `json:"duration_ms"`
	PublishedAt  time.Time `json:"published_at"`
	Stats        Stats     `json:"stats"`
	Author       Author    `json:"author"`
}

type User struct {
	UserID      string  `json:"user_id"`
	Username    string  `json:"username"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	IsCreator   bool    `json:"is_creator"`
	Followers   int64   `json:"followers"`
}

type Hashtag struct {
	Tag         string `json:"tag"`
	VideosCount int64  `json:"videos_count"`
}

// Suggestions answers GET /api/v1/search/suggest.
type Suggestions struct {
	Users    []User    `json:"users"`
	Hashtags []Hashtag `json:"hashtags"`
}
