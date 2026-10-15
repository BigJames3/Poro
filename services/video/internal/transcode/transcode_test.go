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

func TestSelectStreams(t *testing.T) {
	video := func(i int, codec string, w, h int) probeStream {
		return probeStream{Index: i, CodecType: "video", CodecName: codec, Width: w, Height: h}
	}
	audio := func(i int, codec string) probeStream {
		return probeStream{Index: i, CodecType: "audio", CodecName: codec, Channels: 2}
	}
	probe := func(format string, streams ...probeStream) probeJSON {
		p := probeJSON{Streams: streams}
		p.Format.FormatName = format
		return p
	}
	const mov = "mov,mp4,m4a,3gp,3g2,mj2"

	cover := video(0, "mjpeg", 600, 600)
	cover.Disposition.AttachedPic = 1
	data := probeStream{Index: 3, CodecType: "data", CodecName: "bin_data"}
	sel, err := selectStreams("mov", probe(mov, cover, audio(1, "aac"), video(2, "hevc", 1080, 1920), data))
	require.NoError(t, err, "cover art and data tracks are ignored, not decoded")
	require.Equal(t, selection{width: 1080, height: 1920, video: 2, audio: 1}, sel)
	require.Equal(t, []string{"-map", "0:2", "-map", "0:1"}, mapArgs(sel))

	sel, err = selectStreams("matroska", probe("matroska,webm", video(0, "vp9", 640, 360)))
	require.NoError(t, err)
	require.Equal(t, -1, sel.audio)
	require.Equal(t, []string{"-map", "0:0"}, mapArgs(sel))

	many := make([]probeStream, 0, maxStreams+1)
	many = append(many, video(0, "h264", 640, 360))
	for i := 1; i <= maxStreams; i++ {
		many = append(many, probeStream{Index: i, CodecType: "data"})
	}
	cases := []struct {
		name  string
		demux string
		probe probeJSON
		want  string
	}{
		{"demuxer mismatch", "mov", probe("hls", video(0, "h264", 640, 360)), "format"},
		{"video codec", "mov", probe(mov, video(0, "mjpeg", 640, 360)), "video codec"},
		{"raw video", "mov", probe(mov, video(0, "rawvideo", 640, 360)), "video codec"},
		{"second video codec", "mov", probe(mov, video(0, "h264", 640, 360), video(1, "prores", 640, 360)), "video codec"},
		{"audio codec", "mov", probe(mov, video(0, "h264", 640, 360), audio(1, "pcm_s16le")), "audio codec"},
		{"too wide", "mov", probe(mov, video(0, "h264", maxDimension+2, 360)), "x360"},
		{"too tall", "mov", probe(mov, video(0, "h264", 360, maxDimension+2)), "360x"},
		{"too small", "mov", probe(mov, video(0, "h264", 1, 360)), "1x360"},
		{"too many av streams", "mov", probe(mov, video(0, "h264", 640, 360), audio(1, "aac"), audio(2, "aac"), audio(3, "aac"), audio(4, "aac")), "5 audio and video"},
		{"too many streams", "mov", probe(mov, many...), "17 streams"},
		{"no video", "mov", probe(mov, audio(0, "aac")), "no video"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := selectStreams(tc.demux, tc.probe)
			require.ErrorIs(t, err, ErrInvalid)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestInputArgs(t *testing.T) {
	require.Equal(t, []string{
		"-protocol_whitelist", "file", "-f", "mov", "-enable_drefs", "0", "-use_absolute_path", "0", "-i", "file:/w/source",
	}, inputArgs("mov", "/w/source"))
	require.Equal(t, []string{"-protocol_whitelist", "file", "-f", "matroska", "-i", "file:-x"}, inputArgs("matroska", "-x"),
		"the file: prefix keeps a path from being read as an option or a URL")
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
