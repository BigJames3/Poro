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
