package transcode

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Security tests run the real ffmpeg and ffprobe (installed in CI). They prove
// that an uploaded file cannot make the worker open URLs, and that the
// decoding limits hold on real files, not only on hand-written probe output.

func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
}

// generate writes a test file with ffmpeg from lavfi sources.
func generate(t *testing.T, name string, args ...string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), name)
	full := append([]string{"-nostdin", "-loglevel", "error", "-y"}, args...)
	raw, err := exec.Command("ffmpeg", append(full, out)...).CombinedOutput()
	require.NoError(t, err, string(raw))
	return out
}

func transcodeFile(t *testing.T, src string) (*Output, error) {
	t.Helper()
	work := filepath.Join(t.TempDir(), "work")
	require.NoError(t, os.MkdirAll(work, 0o750))
	return NewFFmpeg("ffmpeg", "ffprobe").Transcode(context.Background(), src, work, "user", "vid")
}

// countingServer records every connection, even one that never completes an
// HTTP request.
func countingServer(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			hits.Add(1)
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv.URL, &hits
}

func playlist(url string) string {
	return "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n#EXTINF:1.0,\n" + url + "/seg.ts\n#EXT-X-ENDLIST\n"
}

func TestUploadedPlaylistNeverReachesTheNetwork(t *testing.T) {
	requireFFmpeg(t)
	url, hits := countingServer(t)
	dir := t.TempDir()

	// Control: the same playlist opened with network protocols allowed does
	// connect, so a zero count below means the protections, not the test.
	// (ffmpeg 6 already refuses http from a local playlist by default; older
	// or differently built versions do not.) command drops proxy variables:
	// the connection must reach this server.
	plain := filepath.Join(dir, "plain.m3u8")
	require.NoError(t, os.WriteFile(plain, []byte(playlist(url)), 0o600))
	_ = command(context.Background(), "ffprobe", "-v", "error", "-protocol_whitelist", "file,http,tcp", plain).Run()
	require.Positive(t, hits.Load(), "control: ffprobe follows the playlist when http is allowed")
	hits.Store(0)

	t.Run("playlist upload", func(t *testing.T) {
		_, err := transcodeFile(t, plain)
		require.ErrorIs(t, err, ErrInvalid)
	})
	t.Run("playlist behind an mp4 signature", func(t *testing.T) {
		disguised := filepath.Join(dir, "disguised.mp4")
		require.NoError(t, os.WriteFile(disguised, []byte("\x00\x00\x00\x18ftypisom\n"+playlist(url)), 0o600))
		_, err := transcodeFile(t, disguised)
		require.ErrorIs(t, err, ErrInvalid)
	})
	t.Run("protocol whitelist alone", func(t *testing.T) {
		// Defence in depth: even if a wrong demuxer were chosen, only local
		// files may be opened.
		args := append([]string{"-v", "error"}, inputArgs("hls", plain)...)
		out, err := command(context.Background(), "ffprobe", args...).CombinedOutput()
		require.Error(t, err)
		require.Contains(t, string(out), "not on whitelist")
	})
	require.Zero(t, hits.Load(), "the worker must never connect anywhere on behalf of an upload")
}

func TestTranscodeAcceptsAllowedFormats(t *testing.T) {
	requireFFmpeg(t)
	mkv := generate(t, "src.mkv",
		"-f", "lavfi", "-i", "testsrc=s=320x240:d=1", "-f", "lavfi", "-i", "sine=d=1",
		"-c:v", "libx264", "-c:a", "aac", "-shortest")
	got, err := transcodeFile(t, mkv)
	require.NoError(t, err)
	require.Equal(t, 320, got.Width)
	require.Len(t, got.Renditions, 1)
}

func TestTranscodeRefusesFilesOutsideTheLimits(t *testing.T) {
	requireFFmpeg(t)
	cases := []struct {
		name string
		file func(t *testing.T) string
		want string
	}{
		{"codec outside the list", func(t *testing.T) string {
			return generate(t, "mjpeg.mov", "-f", "lavfi", "-i", "testsrc=s=64x64:d=1", "-c:v", "mjpeg")
		}, `video codec "mjpeg"`},
		{"resolution above the limit", func(t *testing.T) string {
			return generate(t, "wide.mp4", "-f", "lavfi", "-i", "color=s=4112x16:d=0.2", "-c:v", "libx264")
		}, "4112x16"},
		{"too many streams", func(t *testing.T) string {
			args := []string{"-f", "lavfi", "-i", "testsrc=s=64x64:d=1"}
			for range 4 {
				args = append(args, "-f", "lavfi", "-i", "sine=d=1")
			}
			args = append(args, "-map", "0", "-map", "1", "-map", "2", "-map", "3", "-map", "4", "-c:v", "libx264", "-c:a", "aac")
			return generate(t, "streams.mkv", args...)
		}, "5 audio and video streams"},
		{"audio codec outside the list", func(t *testing.T) string {
			return generate(t, "pcm.mov", "-f", "lavfi", "-i", "testsrc=s=64x64:d=1", "-f", "lavfi", "-i", "sine=d=1",
				"-c:v", "libx264", "-c:a", "pcm_s16le", "-shortest")
		}, `audio codec "pcm_s16le"`},
		{"truncated file", func(t *testing.T) string {
			src := generate(t, "full.mp4", "-f", "lavfi", "-i", "testsrc=s=64x64:d=1", "-c:v", "libx264")
			raw, err := os.ReadFile(src)
			require.NoError(t, err)
			cut := filepath.Join(t.TempDir(), "cut.mp4")
			require.NoError(t, os.WriteFile(cut, raw[:64], 0o600))
			return cut
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := transcodeFile(t, tc.file(t))
			require.ErrorIs(t, err, ErrInvalid)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// fakeBinary writes an executable script used in place of ffprobe.
func fakeBinary(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	path := filepath.Join(t.TempDir(), "fake-ffprobe")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700)) //nolint:gosec // G306: test executable
	return path
}

func mp4Header(t *testing.T) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "src.mp4")
	require.NoError(t, os.WriteFile(src, []byte("\x00\x00\x00\x18ftypisom"), 0o600))
	return src
}

func TestFFmpegSeesNoWorkerSecrets(t *testing.T) {
	t.Setenv("S3_SECRET_ACCESS_KEY", "worker-secret")
	t.Setenv("POSTGRES_PASSWORD", "worker-secret")
	dump := filepath.Join(t.TempDir(), "env")
	probe := fakeBinary(t, "env > "+dump+"\nexit 1")

	_, err := NewFFmpeg("ffmpeg", probe).Transcode(context.Background(), mp4Header(t), t.TempDir(), "u", "v")
	require.ErrorIs(t, err, ErrInvalid)
	env, err := os.ReadFile(dump) //nolint:gosec // G304: test file
	require.NoError(t, err)
	require.NotContains(t, string(env), "worker-secret")
	require.Contains(t, string(env), "PATH=")
}

func TestHungProbeIsKilled(t *testing.T) {
	probe := fakeBinary(t, "exec sleep 60")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := NewFFmpeg("ffmpeg", probe).Transcode(ctx, mp4Header(t), t.TempDir(), "u", "v")
	require.ErrorIs(t, err, ErrInvalid)
	require.Less(t, time.Since(start), 10*time.Second, "a cancelled job does not wait for ffprobe")
}

func TestArgumentsNeverReachAShell(t *testing.T) {
	dump := filepath.Join(t.TempDir(), "args")
	probe := fakeBinary(t, `for a in "$@"; do printf '%s\n' "$a"; done > `+dump+"\nexit 1")
	dir := filepath.Join(t.TempDir(), "$(touch pwned);-i")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	src := filepath.Join(dir, "source")
	require.NoError(t, os.WriteFile(src, []byte("\x00\x00\x00\x18ftypisom"), 0o600))

	_, err := NewFFmpeg("ffmpeg", probe).Transcode(context.Background(), src, t.TempDir(), "u", "v")
	require.ErrorIs(t, err, ErrInvalid)
	raw, err := os.ReadFile(dump) //nolint:gosec // G304: test file
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.Equal(t, "file:"+src, lines[len(lines)-1], "the path is one argument, prefixed with file:")
	require.NoFileExists(t, filepath.Join(filepath.Dir(dir), "pwned"))
}
