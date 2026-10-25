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
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	// MediaType is "video" or "photo"; a photo post has images, no HLS.
	MediaType   string `json:"media_type"`
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
	Images           []ImageView     `json:"images,omitempty"`
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

// InitPhotoRequest creates a photo post upload (ADR-0020).
type InitPhotoRequest struct {
	Description string           `json:"description"`
	Images      []PhotoImageSpec `json:"images"`
}

// PhotoImageSpec is one image the client is about to upload.
type PhotoImageSpec struct {
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

// InitPhotoResponse holds one presigned PUT per image, in display order.
type InitPhotoResponse struct {
	VideoID   string        `json:"video_id"`
	ExpiresIn int           `json:"expires_in"`
	Images    []ImageUpload `json:"images"`
}

// ImageUpload is where and how to PUT one image: the headers are signed and
// must be sent unchanged.
type ImageUpload struct {
	Position int               `json:"position"`
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers"`
}

// ImageView is one processed picture of a ready photo post.
type ImageView struct {
	Position     int    `json:"position"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}
