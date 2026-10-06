package transcode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectRungs(t *testing.T) {
	r := selectRungs(1080)
	require.Len(t, r, 2)
	require.Equal(t, "360p", r[0].name)
	require.Equal(t, "720p", r[1].name)

	r = selectRungs(480)
	require.Len(t, r, 1)
	require.Equal(t, "360p", r[0].name)

	r = selectRungs(240)
	require.Len(t, r, 1)
}

func TestParseDurationAndEven(t *testing.T) {
	d, err := parseDuration("1.5")
	require.NoError(t, err)
	require.Equal(t, int64(1500), d.Milliseconds())
	_, err = parseDuration("0")
	require.ErrorIs(t, err, ErrInvalid)
	require.Equal(t, 640, even(641))
	require.Equal(t, 640, even(640))
}

func TestVideoStream(t *testing.T) {
	_, _, _, err := videoStream(probeJSON{})
	require.ErrorIs(t, err, ErrInvalid)
	w, h, audio, err := videoStream(probeJSON{Streams: []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Channels  int    `json:"channels"`
	}{
		{CodecType: "video", Width: 1280, Height: 720},
		{CodecType: "audio", Channels: 2},
	}})
	require.NoError(t, err)
	require.Equal(t, 1280, w)
	require.Equal(t, 720, h)
	require.True(t, audio)
}

func TestFFmpegTranscodeIfInstalled(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "color=c=red:s=640x360:d=1", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-shortest", "-c:v", "libx264", "-t", "1", src)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	work := filepath.Join(dir, "work")
	require.NoError(t, os.MkdirAll(work, 0o755))
	got, err := NewFFmpeg("ffmpeg", "ffprobe").Transcode(context.Background(), src, work, "user", "vid")
	require.NoError(t, err)
	require.Greater(t, got.DurationMs, 0)
	require.NotEmpty(t, got.Renditions)
	require.FileExists(t, filepath.Join(work, "hls", "master.m3u8"))
	require.FileExists(t, filepath.Join(work, "thumb.jpg"))
}
