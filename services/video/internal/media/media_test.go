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
