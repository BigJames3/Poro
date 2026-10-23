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

type ShopRef struct {
	ShopID      string  `json:"shop_id"`
	Name        string  `json:"name"`
	Handle      string  `json:"handle"`
	LogoURL     *string `json:"logo_url"`
	CountryCode string  `json:"country_code"`
}

// Product is an active product of an active shop. MinPrice is the lowest
// variant price in minor units of Currency.
type Product struct {
	ProductID string  `json:"product_id"`
	Title     string  `json:"title"`
	ImageURL  *string `json:"image_url"`
	MinPrice  *int64  `json:"min_price"`
	Currency  string  `json:"currency"`
	InStock   bool    `json:"in_stock"`
	Shop      ShopRef `json:"shop"`
}

type Shop struct {
	ShopRef
	Currency string `json:"currency"`
}

// Suggestions answers GET /api/v1/search/suggest.
type Suggestions struct {
	Users    []User    `json:"users"`
	Hashtags []Hashtag `json:"hashtags"`
}
