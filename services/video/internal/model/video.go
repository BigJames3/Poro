package model

import "time"

const (
	StatusUploading  = "uploading"
	StatusProcessing = "processing"
	StatusReady      = "ready"
	StatusFailed     = "failed"

	ModerationApproved = "approved"
	ModerationRemoved  = "removed"

	MaxBytes            = 256 << 20
	MaxDurationSeconds  = 180
	PartSize            = 8 << 20
	MaxTitleRunes       = 100
	MaxDescriptionRunes = 500
	DefaultListLimit    = 20
	MaxListLimit        = 50
)

// AllowedContentTypes maps accepted MIME types to a file extension.
var AllowedContentTypes = map[string]string{
	"video/mp4":       "mp4",
	"video/quicktime": "mov",
	"video/webm":      "webm",
}

// Rendition is one HLS ladder rung stored on the video row.
type Rendition struct {
	Name        string `json:"name"`
	Bandwidth   int    `json:"bandwidth"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	PlaylistKey string `json:"playlist_key"`
}

// Video is the VIDEO aggregate. last_error is for operators only.
type Video struct {
	ID           string
	UserID       string
	Title        string
	Description  string
	Status       string
	ContentType  string
	SizeBytes    int64
	SourceKey    string
	S3UploadID   *string
	DurationMs   *int
	Width        *int
	Height       *int
	HLSKey       *string
	ThumbnailKey *string
	Renditions   []Rendition
	FailureCode  *string
	LastError    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
	// ModerationStatus is approved or removed; a removed video is shown to
	// its owner only, without media.
	ModerationStatus string
	ModeratedAt      *time.Time
}

// PartCount is how many multipart parts a client must upload for size bytes.
func PartCount(size int64) int {
	if size <= 0 {
		return 0
	}
	n := int((size + PartSize - 1) / PartSize)
	if n < 1 {
		return 1
	}
	return n
}

// SourceKey is the deterministic object key of the original upload.
func SourceKey(userID, videoID, ext string) string {
	return "videos/" + userID + "/" + videoID + "/source." + ext
}

// HLSDir is the prefix of transcoded HLS objects.
func HLSDir(userID, videoID string) string {
	return "videos/" + userID + "/" + videoID + "/hls"
}

// MediaPrefix holds every object of a video: source, HLS and thumbnail.
func MediaPrefix(userID, videoID string) string {
	return "videos/" + userID + "/" + videoID + "/"
}

// ThumbKey is the JPEG thumbnail object key.
func ThumbKey(userID, videoID string) string {
	return "videos/" + userID + "/" + videoID + "/thumb.jpg"
}
