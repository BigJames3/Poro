// Package dto defines the JSON bodies of the social API.
package dto

import "time"

// Page is a keyset-paginated list. NextCursor is null on the last page.
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// VideoStats are the public counters of a video.
type VideoStats struct {
	VideoID   string `json:"video_id"`
	Likes     int64  `json:"likes"`
	Comments  int64  `json:"comments"`
	Shares    int64  `json:"shares"`
	LikedByMe bool   `json:"liked_by_me"`
}

// UserStats are the public counters of an account.
type UserStats struct {
	UserID       string `json:"user_id"`
	Followers    int64  `json:"followers"`
	Following    int64  `json:"following"`
	FollowedByMe bool   `json:"followed_by_me"`
}

// VideoLike is one entry of the likes of a video.
type VideoLike struct {
	UserID  string    `json:"user_id"`
	LikedAt time.Time `json:"liked_at"`
}

// LikedVideo is one entry of the videos a user likes.
type LikedVideo struct {
	VideoID string    `json:"video_id"`
	LikedAt time.Time `json:"liked_at"`
}

// FollowEntry is one entry of a followers or following list.
type FollowEntry struct {
	UserID     string    `json:"user_id"`
	FollowedAt time.Time `json:"followed_at"`
}

// CreateCommentRequest posts a comment, or a reply when ParentID is set.
type CreateCommentRequest struct {
	Content  string  `json:"content"`
	ParentID *string `json:"parent_id"`
}

// EditCommentRequest replaces the text of a comment.
type EditCommentRequest struct {
	Content string `json:"content"`
}

// Comment is a comment or a reply. A deleted comment keeps its place in the
// thread but exposes neither its author nor its text.
type Comment struct {
	ID           string    `json:"id"`
	VideoID      string    `json:"video_id"`
	UserID       *string   `json:"user_id"`
	ParentID     *string   `json:"parent_id"`
	Content      *string   `json:"content"`
	LikesCount   int       `json:"likes_count"`
	RepliesCount int       `json:"replies_count"`
	LikedByMe    bool      `json:"liked_by_me"`
	Edited       bool      `json:"edited"`
	Deleted      bool      `json:"deleted"`
	CreatedAt    time.Time `json:"created_at"`
}

// CommentLikeState is returned after liking or unliking a comment.
type CommentLikeState struct {
	CommentID  string `json:"comment_id"`
	LikesCount int    `json:"likes_count"`
	LikedByMe  bool   `json:"liked_by_me"`
}

// ShareRequest records a share on a channel: whatsapp, copy_link or other.
type ShareRequest struct {
	Channel string `json:"channel"`
}

// ShareResult is returned after a share.
type ShareResult struct {
	ShareID     string `json:"share_id"`
	VideoID     string `json:"video_id"`
	Channel     string `json:"channel"`
	SharesCount int64  `json:"shares_count"`
}
