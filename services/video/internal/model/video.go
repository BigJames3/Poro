package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

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

	// AbandonedUploadAge is how long an upload may stay in `uploading` before
	// the sweeper discards it. Part URLs expire after an hour, so a day leaves
	// a slow client ample time to call complete.
	AbandonedUploadAge = 24 * time.Hour
	// UploadSweepInterval is how often the API looks for abandoned uploads.
	UploadSweepInterval = time.Hour
	// UploadSweepBatch caps the uploads discarded in one pass.
	UploadSweepBatch = 100
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

// PartLength is the exact length of part (1-based) of an upload of size
// bytes: PartSize for every part but the last, which holds the rest.
func PartLength(size int64, part int) int64 {
	start := int64(part-1) * PartSize
	return min(PartSize, size-start)
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

// MediaRoot prefixes every object key of every video.
const MediaRoot = "videos/"

// SourceKey is the deterministic object key of the original upload. It lives
// in the private source bucket, never in the public media bucket.
func SourceKey(userID, videoID, ext string) string {
	return MediaRoot + userID + "/" + videoID + "/source." + ext
}

// IsSourceKey reports whether key has the shape of a SourceKey.
func IsSourceKey(key string) bool {
	parts := strings.Split(key, "/")
	if len(parts) != 4 || parts[0]+"/" != MediaRoot {
		return false
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return false
	}
	if _, err := uuid.Parse(parts[2]); err != nil {
		return false
	}
	ext, ok := strings.CutPrefix(parts[3], "source.")
	return ok && ext != "" && !strings.Contains(ext, ".")
}

// HLSDir is the prefix of transcoded HLS objects.
func HLSDir(userID, videoID string) string {
	return MediaRoot + userID + "/" + videoID + "/hls"
}

// MediaPrefix holds every object of a video: source, HLS and thumbnail.
func MediaPrefix(userID, videoID string) string {
	return MediaRoot + userID + "/" + videoID + "/"
}

// ThumbKey is the JPEG thumbnail object key.
func ThumbKey(userID, videoID string) string {
	return MediaRoot + userID + "/" + videoID + "/thumb.jpg"
}
