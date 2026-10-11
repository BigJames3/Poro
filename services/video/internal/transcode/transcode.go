package transcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/poro/video/internal/model"
)

const timeout = 10 * time.Minute

// Output is the HLS ladder and thumbnail produced from a source file.
type Output struct {
	DurationMs int
	Width      int
	Height     int
	Renditions []model.Rendition
	Files      []File
}

// File is one object to upload, relative to the transcode working directory.
type File struct {
	Rel         string
	ContentType string
}

// Transcoder turns a source file into HLS + a JPEG thumbnail.
type Transcoder interface {
	Transcode(ctx context.Context, srcPath, workDir, userID, videoID string) (*Output, error)
}

type probeJSON struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Channels  int    `json:"channels"`
	} `json:"streams"`
}

type rung struct {
	name     string
	height   int
	vBitrate int
	aBitrate int
}

var ladder = []rung{
	{name: "360p", height: 360, vBitrate: 800_000, aBitrate: 96_000},
	{name: "720p", height: 720, vBitrate: 2_800_000, aBitrate: 128_000},
}

// FFmpeg shells out to ffmpeg and ffprobe.
type FFmpeg struct {
	ffmpeg  string
	ffprobe string
}

// NewFFmpeg uses binaries on PATH when the names are empty.
func NewFFmpeg(ffmpeg, ffprobe string) *FFmpeg {
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	if ffprobe == "" {
		ffprobe = "ffprobe"
	}
	return &FFmpeg{ffmpeg: ffmpeg, ffprobe: ffprobe}
}

// Transcode probes the source, rejects clips longer than 3 minutes, writes
// HLS rungs the source can support, and a JPEG thumbnail.
func (f *FFmpeg) Transcode(ctx context.Context, srcPath, workDir, userID, videoID string) (*Output, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	probe, err := f.probe(ctx, srcPath)
	if err != nil {
		return nil, err
	}
	w, h, hasAudio, err := videoStream(probe)
	if err != nil {
		return nil, err
	}
	dur, err := parseDuration(probe.Format.Duration)
	if err != nil {
		return nil, err
	}
	if dur > time.Duration(model.MaxDurationSeconds)*time.Second {
		return nil, ErrTooLong
	}
	if dur < time.Millisecond {
		return nil, ErrInvalid
	}

	hlsDir := filepath.Join(workDir, "hls")
	if err := os.MkdirAll(hlsDir, 0o750); err != nil {
		return nil, err
	}

	out := &Output{DurationMs: int(dur / time.Millisecond), Width: w, Height: h}
	var master bytes.Buffer
	master.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")

	for _, r := range selectRungs(h) {
		height := r.height
		if h < height {
			height = h
			if height%2 != 0 {
				height--
			}
		}
		if height < 2 {
			return nil, ErrInvalid
		}
		width := even(w * height / h)
		if width < 2 {
			width = 2
		}
		rungDir := filepath.Join(hlsDir, r.name)
		if err := os.MkdirAll(rungDir, 0o750); err != nil {
			return nil, err
		}
		playlist := filepath.Join(rungDir, "index.m3u8")
		args := []string{
			"-y", "-i", srcPath,
			"-vf", fmt.Sprintf("scale=-2:%d", height),
			"-c:v", "libx264", "-preset", "veryfast", "-profile:v", "baseline", "-level", "3.0",
			"-b:v", strconv.Itoa(r.vBitrate), "-maxrate", strconv.Itoa(r.vBitrate),
			"-bufsize", strconv.Itoa(r.vBitrate * 2), "-g", "48", "-keyint_min", "48", "-sc_threshold", "0",
			"-hls_time", "4", "-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(rungDir, "seg_%03d.ts"),
		}
		if hasAudio {
			args = append(args, "-c:a", "aac", "-b:a", strconv.Itoa(r.aBitrate), "-ac", "2")
		} else {
			args = append(args, "-an")
		}
		args = append(args, playlist)
		if err := run(ctx, f.ffmpeg, args...); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrTranscode, err)
		}
		bw := r.vBitrate
		if hasAudio {
			bw += r.aBitrate
		}
		relPlaylist := "hls/" + r.name + "/index.m3u8"
		key := model.HLSDir(userID, videoID) + "/" + r.name + "/index.m3u8"
		out.Renditions = append(out.Renditions, model.Rendition{
			Name: r.name, Bandwidth: bw, Width: width, Height: height, PlaylistKey: key,
		})
		fmt.Fprintf(&master, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s\n", bw, width, height, r.name+"/index.m3u8")
		out.Files = append(out.Files, File{Rel: relPlaylist, ContentType: "application/vnd.apple.mpegurl"})
		segs, err := filepath.Glob(filepath.Join(rungDir, "seg_*.ts"))
		if err != nil {
			return nil, err
		}
		for _, seg := range segs {
			rel, err := filepath.Rel(workDir, seg)
			if err != nil {
				return nil, err
			}
			out.Files = append(out.Files, File{Rel: filepath.ToSlash(rel), ContentType: "video/mp2t"})
		}
	}
	if len(out.Renditions) == 0 {
		return nil, ErrInvalid
	}
	masterPath := filepath.Join(hlsDir, "master.m3u8")
	if err := os.WriteFile(masterPath, master.Bytes(), 0o600); err != nil {
		return nil, err
	}
	out.Files = append(out.Files, File{Rel: "hls/master.m3u8", ContentType: "application/vnd.apple.mpegurl"})

	ss := "1"
	if dur < 2*time.Second {
		ss = "0"
	}
	thumb := filepath.Join(workDir, "thumb.jpg")
	if err := run(ctx, f.ffmpeg, "-y", "-ss", ss, "-i", srcPath, "-frames:v", "1", "-vf", "scale=640:-2", "-q:v", "4", thumb); err != nil {
		return nil, fmt.Errorf("%w: thumbnail: %w", ErrTranscode, err)
	}
	out.Files = append(out.Files, File{Rel: "thumb.jpg", ContentType: "image/jpeg"})
	return out, nil
}

func (f *FFmpeg) probe(ctx context.Context, src string) (probeJSON, error) {
	var stdout, stderr bytes.Buffer
	// The binary comes from configuration and src is a file this worker wrote:
	// arguments are passed without a shell.
	cmd := exec.CommandContext(ctx, f.ffprobe, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", src) //nolint:gosec // G204: configured binary, local file
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return probeJSON{}, fmt.Errorf("%w: ffprobe: %w", ErrInvalid, err)
	}
	var p probeJSON
	if err := json.Unmarshal(stdout.Bytes(), &p); err != nil {
		return probeJSON{}, fmt.Errorf("%w: ffprobe json", ErrInvalid)
	}
	return p, nil
}

// run executes the configured ffmpeg binary with arguments built by this
// package from paths under the worker's own directory, never through a shell.
func run(ctx context.Context, bin string, args ...string) error {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // G204: configured binary, arguments built here
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return fmt.Errorf("%s: %w", msg, err)
	}
	return nil
}

func videoStream(p probeJSON) (w, h int, audio bool, err error) {
	for _, s := range p.Streams {
		if s.CodecType == "audio" {
			audio = true
		}
		if s.CodecType == "video" && w == 0 {
			w, h = s.Width, s.Height
		}
	}
	if w < 2 || h < 2 {
		return 0, 0, false, ErrInvalid
	}
	return w, h, audio, nil
}

func parseDuration(s string) (time.Duration, error) {
	sec, err := strconv.ParseFloat(s, 64)
	if err != nil || sec <= 0 {
		return 0, ErrInvalid
	}
	return time.Duration(sec * float64(time.Second)), nil
}

func selectRungs(sourceHeight int) []rung {
	var out []rung
	for _, r := range ladder {
		if r.height > 360 && sourceHeight < r.height {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		out = append(out, ladder[0])
	}
	return out
}

func even(n int) int {
	if n%2 != 0 {
		return n - 1
	}
	return n
}

// Sentinel errors the worker maps to public failure codes.
var (
	ErrTooLong   = fmt.Errorf("too_long")
	ErrInvalid   = fmt.Errorf("invalid_media")
	ErrTranscode = fmt.Errorf("transcode_failed")
)
