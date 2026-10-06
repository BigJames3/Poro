package media

import "bytes"

var (
	webmMagic = []byte{0x1A, 0x45, 0xDF, 0xA3}
	ftyp      = []byte("ftyp")
)

// LooksLikeVideo reports whether the first bytes of an object are MP4/MOV (ftyp)
// or WebM (EBML). Anything else — HTML, PDF, random bytes — is rejected.
func LooksLikeVideo(head []byte) bool {
	if len(head) >= 4 && bytes.Equal(head[:4], webmMagic) {
		return true
	}
	if len(head) >= 8 && bytes.Equal(head[4:8], ftyp) {
		return true
	}
	return false
}
