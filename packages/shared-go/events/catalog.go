package events

import "time"

// Event types. The JSON Schema of each lives in packages/contracts/events.
const (
	TypeAuthUserCreated      = "poro.auth.user.created"
	TypeUserCreatorActivated = "poro.user.creator.activated"
	TypeUserProfileUpdated   = "poro.user.profile.updated"
	TypeVideoUploaded        = "poro.video.uploaded"
	TypeVideoReady           = "poro.video.ready"
	TypeVideoFailed          = "poro.video.failed"
	TypeVideoDeleted         = "poro.video.deleted"

	TypeSocialLikeCreated    = "poro.social.like.created"
	TypeSocialLikeDeleted    = "poro.social.like.deleted"
	TypeSocialCommentCreated = "poro.social.comment.created"
	TypeSocialCommentDeleted = "poro.social.comment.deleted"
	TypeSocialFollowCreated  = "poro.social.follow.created"
	TypeSocialFollowDeleted  = "poro.social.follow.deleted"
	TypeSocialShareCreated   = "poro.social.share.created"
)

// AuthUserCreatedV1 is published by auth when an account is created.
// It carries no phone number or email: consumers never need them.
type AuthUserCreatedV1 struct {
	UserID       string    `json:"user_id"`
	SignupMethod string    `json:"signup_method"` // phone, email
	CountryCode  *string   `json:"country_code"`
	Language     string    `json:"language"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserCreatorActivatedV1 is published by the user service when a profile becomes a creator.
type UserCreatorActivatedV1 struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	ActivatedAt time.Time `json:"activated_at"`
}

// UserProfileUpdatedV1 is published by the user service whenever a public
// profile field changes. It is a full snapshot: keep the latest UpdatedAt.
type UserProfileUpdatedV1 struct {
	UserID      string    `json:"user_id"`
	Username    *string   `json:"username"`
	DisplayName *string   `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url"`
	IsCreator   bool      `json:"is_creator"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VideoUploadedV1 is published by the video API after a multipart upload completes.
// The media-worker transcodes the object at SourceKey.
type VideoUploadedV1 struct {
	VideoID     string    `json:"video_id"`
	UserID      string    `json:"user_id"`
	SourceKey   string    `json:"source_key"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// VideoRenditionV1 is one HLS ladder rung published when a video is ready.
type VideoRenditionV1 struct {
	Name        string `json:"name"`
	Bandwidth   int    `json:"bandwidth"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	PlaylistKey string `json:"playlist_key"`
}

// VideoReadyV1 is published by the media-worker after HLS and a thumbnail exist.
// Title, Description, Hashtags and PublishedAt were added after the first
// release: consumers must accept events without them.
type VideoReadyV1 struct {
	VideoID      string             `json:"video_id"`
	UserID       string             `json:"user_id"`
	DurationMs   int                `json:"duration_ms"`
	Width        int                `json:"width"`
	Height       int                `json:"height"`
	HLSKey       string             `json:"hls_key"`
	ThumbnailKey string             `json:"thumbnail_key"`
	Renditions   []VideoRenditionV1 `json:"renditions"`
	ReadyAt      time.Time          `json:"ready_at"`
	Title        string             `json:"title"`
	Description  string             `json:"description"`
	Hashtags     []string           `json:"hashtags"`
	PublishedAt  time.Time          `json:"published_at"`
}

// Failure codes on VideoFailedV1. Never include FFmpeg stderr or file paths.
const (
	VideoFailTooLong   = "too_long"
	VideoFailInvalid   = "invalid_media"
	VideoFailTranscode = "transcode_failed"
)

// VideoFailedV1 is published when transcode cannot produce playable HLS.
type VideoFailedV1 struct {
	VideoID  string    `json:"video_id"`
	UserID   string    `json:"user_id"`
	Code     string    `json:"code"`
	FailedAt time.Time `json:"failed_at"`
}

// VideoDeletedV1 is published by the video API when the owner deletes a video.
// Projections must drop the video, whatever status they last saw.
type VideoDeletedV1 struct {
	VideoID   string    `json:"video_id"`
	UserID    string    `json:"user_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// Share channels accepted on SocialShareCreatedV1.
const (
	ShareChannelWhatsApp = "whatsapp"
	ShareChannelCopyLink = "copy_link"
	ShareChannelOther    = "other"
)

// SocialLikeCreatedV1 is published by social when a user starts liking a video.
type SocialLikeCreatedV1 struct {
	LikeID       string    `json:"like_id"`
	UserID       string    `json:"user_id"`
	VideoID      string    `json:"video_id"`
	VideoOwnerID string    `json:"video_owner_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// SocialLikeDeletedV1 is published by social when a user removes a like.
type SocialLikeDeletedV1 struct {
	UserID       string    `json:"user_id"`
	VideoID      string    `json:"video_id"`
	VideoOwnerID string    `json:"video_owner_id"`
	DeletedAt    time.Time `json:"deleted_at"`
}

// SocialCommentCreatedV1 is published by social for a comment or a reply.
// ParentID and ParentAuthorID are nil for a top-level comment.
type SocialCommentCreatedV1 struct {
	CommentID      string    `json:"comment_id"`
	UserID         string    `json:"user_id"`
	VideoID        string    `json:"video_id"`
	VideoOwnerID   string    `json:"video_owner_id"`
	ParentID       *string   `json:"parent_id"`
	ParentAuthorID *string   `json:"parent_author_id"`
	Excerpt        string    `json:"excerpt"`
	CreatedAt      time.Time `json:"created_at"`
}

// SocialCommentDeletedV1 is published by social when a comment is soft-deleted.
// UserID is the comment author, whoever deleted it.
type SocialCommentDeletedV1 struct {
	CommentID string    `json:"comment_id"`
	VideoID   string    `json:"video_id"`
	UserID    string    `json:"user_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// SocialFollowCreatedV1 is published by social when a user follows another.
type SocialFollowCreatedV1 struct {
	FollowerID  string    `json:"follower_id"`
	FollowingID string    `json:"following_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// SocialFollowDeletedV1 is published by social when a user unfollows another.
type SocialFollowDeletedV1 struct {
	FollowerID  string    `json:"follower_id"`
	FollowingID string    `json:"following_id"`
	DeletedAt   time.Time `json:"deleted_at"`
}

// SocialShareCreatedV1 is published by social each time a video is shared.
type SocialShareCreatedV1 struct {
	ShareID      string    `json:"share_id"`
	UserID       string    `json:"user_id"`
	VideoID      string    `json:"video_id"`
	VideoOwnerID string    `json:"video_owner_id"`
	Channel      string    `json:"channel"`
	CreatedAt    time.Time `json:"created_at"`
}
