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
