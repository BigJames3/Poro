package media

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLooksLikeVideo(t *testing.T) {
	mp4 := make([]byte, 12)
	copy(mp4[4:], []byte("ftypisom"))
	require.True(t, LooksLikeVideo(mp4))

	webm := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x00}
	require.True(t, LooksLikeVideo(webm))

	require.False(t, LooksLikeVideo([]byte("<html>")))
	require.False(t, LooksLikeVideo([]byte("%PDF-1.4")))
	require.False(t, LooksLikeVideo(nil))
	require.False(t, LooksLikeVideo([]byte("ftyp")))
}

func TestContainer(t *testing.T) {
	require.Equal(t, DemuxerMOV, Container([]byte("\x00\x00\x00\x18ftypisom")))
	require.Equal(t, DemuxerMatroska, Container([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x01}))
	require.Empty(t, Container([]byte("#EXTM3U\nhttp://127.0.0.1/x")), "a playlist never reaches ffmpeg")
	require.Empty(t, Container([]byte("ffconcat version 1.0")))
	require.Empty(t, Container(nil))
}

func TestDetectImageType(t *testing.T) {
	cases := map[string]struct {
		head []byte
		want string
	}{
		"jpeg":           {[]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F'}, "image/jpeg"},
		"jpeg with exif": {[]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x12, 0x34, 'E', 'x', 'i', 'f'}, "image/jpeg"},
		"png":            {[]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), "image/png"},
		"webp":           {[]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), "image/webp"},
		"heic":           {[]byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic"), ""},
		"heif":           {[]byte("\x00\x00\x00\x1cftypmif1\x00\x00\x00\x00mif1heic"), ""},
		"avi riff":       {[]byte("RIFF\x24\x00\x00\x00AVI LIST"), ""},
		"gif":            {[]byte("GIF89a\x01\x00\x01\x00"), ""},
		"svg":            {[]byte("<svg xmlns="), ""},
		"mp4":            {[]byte("\x00\x00\x00\x18ftypisom"), ""},
		"truncated jpeg": {[]byte{0xFF, 0xD8}, ""},
		"truncated png":  {[]byte("\x89PNG"), ""},
		"empty":          {nil, ""},
	}
	for name, c := range cases {
		if got := DetectImageType(c.head); got != c.want {
			t.Errorf("%s: DetectImageType = %q, want %q", name, got, c.want)
		}
	}
}
