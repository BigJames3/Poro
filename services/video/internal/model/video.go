package model

import (
	"strconv"
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

	// MediaTypeVideo and MediaTypePhoto tell a video from a photo post
	// (ADR-0020). Rows written before photo posts are videos.
	MediaTypeVideo = "video"
	MediaTypePhoto = "photo"

	// MaxImages and MaxImageBytes bound a photo post: 1 to 10 images of at
	// most 10 MiB each before processing.
	MaxImages     = 10
	MaxImageBytes = 10 << 20
	// PhotoContentType is stored in videos.content_type for a photo post;
	// each image keeps its own type in video_images.
	PhotoContentType = "image/*"

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

// AllowedImageTypes are the image types a photo post accepts, checked again
// on the stored bytes (DetectImageType). HEIC/HEIF is refused.
var AllowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// Image is one picture of a photo post. ImageKey, ThumbnailKey, Width and
// Height are set by the media-worker once the post is processed.
type Image struct {
	Position     int
	ContentType  string
	SizeBytes    int64
	SourceKey    string
	ImageKey     *string
	ThumbnailKey *string
	Width        *int
	Height       *int
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
	// MediaType is MediaTypeVideo or MediaTypePhoto.
	MediaType string
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

// ImageSourceKey is the private source object of image position (1-based) of
// a photo post. It sits under MediaPrefix like every object of the post, but
// never matches IsSourceKey, which only names video sources.
func ImageSourceKey(userID, videoID string, position int) string {
	return MediaRoot + userID + "/" + videoID + "/source-" + strconv.Itoa(position)
}

// MediaPrefix holds every object of a video: source, HLS and thumbnail.
func MediaPrefix(userID, videoID string) string {
	return MediaRoot + userID + "/" + videoID + "/"
}

// ThumbKey is the JPEG thumbnail object key.
func ThumbKey(userID, videoID string) string {
	return MediaRoot + userID + "/" + videoID + "/thumb.jpg"
}
