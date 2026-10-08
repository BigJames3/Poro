package dto

import "time"

type InitRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type PartURL struct {
	PartNumber int32  `json:"part_number"`
	URL        string `json:"url"`
}

type InitResponse struct {
	VideoID  string    `json:"video_id"`
	UploadID string    `json:"upload_id"`
	Key      string    `json:"key"`
	PartSize int64     `json:"part_size"`
	Parts    []PartURL `json:"parts"`
}

type CompleteRequest struct {
	Parts []CompletePart `json:"parts"`
}

type CompletePart struct {
	PartNumber int32  `json:"part_number"`
	ETag       string `json:"etag"`
}

type VideoView struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	// ModerationStatus is approved or removed; only the owner sees a removed video.
	ModerationStatus string          `json:"moderation_status"`
	DurationMs       *int            `json:"duration_ms,omitempty"`
	Width            *int            `json:"width,omitempty"`
	Height           *int            `json:"height,omitempty"`
	HLSURL           *string         `json:"hls_url,omitempty"`
	ThumbnailURL     *string         `json:"thumbnail_url,omitempty"`
	Renditions       []RenditionView `json:"renditions,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

type RenditionView struct {
	Name      string `json:"name"`
	Bandwidth int    `json:"bandwidth"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type ListResponse struct {
	Items      []VideoView `json:"items"`
	NextCursor *string     `json:"next_cursor"`
}
