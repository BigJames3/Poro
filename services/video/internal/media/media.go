package media

import "bytes"

var (
	webmMagic = []byte{0x1A, 0x45, 0xDF, 0xA3}
	ftyp      = []byte("ftyp")
)

// Demuxers named by Container. They are passed to ffmpeg with -f so that the
// content of a file can never pick another demuxer (HLS, concat, image2...).
const (
	DemuxerMOV      = "mov"
	DemuxerMatroska = "matroska"
)

// Container returns the ffmpeg demuxer matching the first bytes of an object:
// MP4/MOV (ftyp) or WebM/Matroska (EBML). Anything else — HTML, PDF, playlists,
// random bytes — returns "".
func Container(head []byte) string {
	if len(head) >= 4 && bytes.Equal(head[:4], webmMagic) {
		return DemuxerMatroska
	}
	if len(head) >= 8 && bytes.Equal(head[4:8], ftyp) {
		return DemuxerMOV
	}
	return ""
}

// LooksLikeVideo reports whether the first bytes of an object are MP4/MOV (ftyp)
// or WebM (EBML).
func LooksLikeVideo(head []byte) bool {
	return Container(head) != ""
}

var (
	jpegMagic = []byte{0xFF, 0xD8, 0xFF}
	pngMagic  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}
	riffMagic = []byte("RIFF")
	webpMagic = []byte("WEBP")
)

// DetectImageType returns the image type of the first bytes of an object:
// image/jpeg, image/png or image/webp. Anything else — HEIC/HEIF (an ISO
// ftyp box), GIF, SVG, HTML, videos — returns "". The declared type and the
// file name are never trusted.
func DetectImageType(head []byte) string {
	switch {
	case len(head) >= 3 && bytes.Equal(head[:3], jpegMagic):
		return "image/jpeg"
	case len(head) >= 8 && bytes.Equal(head[:8], pngMagic):
		return "image/png"
	case len(head) >= 12 && bytes.Equal(head[:4], riffMagic) && bytes.Equal(head[8:12], webpMagic):
		return "image/webp"
	}
	return ""
}
