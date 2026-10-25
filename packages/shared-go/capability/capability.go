// Package capability reads the Poro-Capabilities request header (ADR-0020):
// what a client can display, as comma-separated case-insensitive tokens.
// Unknown tokens are ignored. Without the header, as from every app released
// before photo posts, a client declares nothing and receives videos only.
package capability

import "strings"

// Header is the request header a client declares its capabilities in.
const Header = "Poro-Capabilities"

// PhotoPosts is declared by clients that display photo posts.
const PhotoPosts = "photo-posts"

// Has reports whether header lists token.
func Has(header, token string) bool {
	for _, t := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(t), token) {
			return true
		}
	}
	return false
}

// Media types of a post, as stored by the video service and its consumers.
const (
	MediaVideo = "video"
	MediaPhoto = "photo"
)

// MediaTypes returns the media types a client may receive: videos only,
// unless it declared photo posts.
func MediaTypes(photoPosts bool) []string {
	if photoPosts {
		return []string{MediaVideo, MediaPhoto}
	}
	return []string{MediaVideo}
}

// Scope names the set of media types, for cache keys and cursors: "v" for
// videos only, "vp" for videos and photos. A page or cursor built for one
// scope is never served to the other.
func Scope(photoPosts bool) string {
	if photoPosts {
		return "vp"
	}
	return "v"
}
