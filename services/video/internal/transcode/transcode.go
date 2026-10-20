package transcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/poro/video/internal/media"
	"github.com/poro/video/internal/model"
)

// Limits on what the worker agrees to decode (ADR-0007). Uploads are
// untrusted: everything outside these lists is refused before ffmpeg decodes
// a single frame.
const (
	timeout      = 10 * time.Minute
	probeTimeout = 30 * time.Second
	// waitDelay bounds how long a killed ffmpeg may keep its pipes open.
	waitDelay = 5 * time.Second

	maxDimension = 4096
	// maxAVStreams counts the audio and video streams of the source; cover
	// art and data tracks (timecode, phone metadata) are never decoded and
	// only count toward maxStreams.
	maxAVStreams = 4
	maxStreams   = 16
	// threads caps the decoder and encoder threads of one ffmpeg process.
	threads = "2"
	// maxAlloc refuses any single allocation above 512 MiB inside ffmpeg.
	maxAlloc = "536870912"
)

var (
	videoCodecs = map[string]bool{"h264": true, "hevc": true, "mpeg4": true, "vp8": true, "vp9": true, "av1": true}
	audioCodecs = map[string]bool{"aac": true, "mp3": true, "opus": true, "vorbis": true}
)

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
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
	Streams []probeStream `json:"streams"`
}

type probeStream struct {
	Index       int    `json:"index"`
	CodecType   string `json:"codec_type"`
	CodecName   string `json:"codec_name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Channels    int    `json:"channels"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

// selection is the part of a source the worker decodes.
type selection struct {
	width, height int
	video         int // stream index
	audio         int // stream index, -1 without audio
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

	demuxer, err := sniff(srcPath)
	if err != nil {
		return nil, err
	}
	probe, err := f.probe(ctx, demuxer, srcPath)
	if err != nil {
		return nil, err
	}
	sel, err := selectStreams(demuxer, probe)
	if err != nil {
		return nil, err
	}
	w, h, hasAudio := sel.width, sel.height, sel.audio >= 0
	input := inputArgs(demuxer, srcPath)
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
		args := append(globalArgs(), input...)
		args = append(args, mapArgs(sel)...)
		args = append(args,
			"-threads", threads,
			"-vf", fmt.Sprintf("scale=-2:%d", height),
			"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-profile:v", "baseline", "-level", "3.0",
			"-b:v", strconv.Itoa(r.vBitrate), "-maxrate", strconv.Itoa(r.vBitrate),
			"-bufsize", strconv.Itoa(r.vBitrate*2), "-g", "48", "-keyint_min", "48", "-sc_threshold", "0",
			"-hls_time", "4", "-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(rungDir, "seg_%03d.ts"),
		)
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
	thumbArgs := append(globalArgs(), "-ss", ss)
	thumbArgs = append(thumbArgs, input...)
	thumbArgs = append(thumbArgs, "-map", "0:"+strconv.Itoa(sel.video), "-threads", threads,
		"-frames:v", "1", "-vf", "scale=640:-2", "-q:v", "4", thumb)
	if err := run(ctx, f.ffmpeg, thumbArgs...); err != nil {
		return nil, fmt.Errorf("%w: thumbnail: %w", ErrTranscode, err)
	}
	out.Files = append(out.Files, File{Rel: "thumb.jpg", ContentType: "image/jpeg"})
	return out, nil
}

// sniff picks the demuxer from the first bytes of the file, as the API did
// when it accepted the upload: the object may have changed since.
func sniff(srcPath string) (string, error) {
	f, err := os.Open(filepath.Clean(srcPath))
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 12)
	n, _ := io.ReadFull(f, head)
	demuxer := media.Container(head[:n])
	if demuxer == "" {
		return "", fmt.Errorf("%w: not an mp4, mov or webm file", ErrInvalid)
	}
	return demuxer, nil
}

// inputArgs opens the source with a forced demuxer and local file access
// only. Without them a crafted file could make ffmpeg read a playlist, a
// concat list or an external data reference, and fetch URLs or local files
// on the worker's behalf.
func inputArgs(demuxer, srcPath string) []string {
	args := []string{"-protocol_whitelist", "file", "-f", demuxer}
	if demuxer == media.DemuxerMOV {
		args = append(args, "-enable_drefs", "0", "-use_absolute_path", "0")
	}
	return append(args, "-i", "file:"+srcPath)
}

// globalArgs never read stdin, overwrite outputs this package created, and
// keep memory and threads bounded.
func globalArgs() []string {
	return []string{"-nostdin", "-hide_banner", "-nostats", "-loglevel", "error", "-y", "-max_alloc", maxAlloc, "-threads", threads}
}

// mapArgs decodes only the selected streams: data tracks, subtitles and
// cover art never reach a decoder.
func mapArgs(sel selection) []string {
	args := []string{"-map", "0:" + strconv.Itoa(sel.video)}
	if sel.audio >= 0 {
		args = append(args, "-map", "0:"+strconv.Itoa(sel.audio))
	}
	return args
}

func (f *FFmpeg) probe(ctx context.Context, demuxer, src string) (probeJSON, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	args := append([]string{"-v", "error", "-max_alloc", maxAlloc, "-print_format", "json", "-show_format", "-show_streams"}, inputArgs(demuxer, src)...)
	cmd := command(ctx, f.ffprobe, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return probeJSON{}, fmt.Errorf("%w: ffprobe: %s: %w", ErrInvalid, truncate(stderr.String()), err)
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
	cmd := command(ctx, bin, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", truncate(stderr.String()), err)
	}
	return nil
}

// command starts ffmpeg or ffprobe without a shell and with an environment
// reduced to PATH: the worker's storage and database credentials are never
// visible to the process that parses untrusted files. A cancelled context
// kills it, and waitDelay bounds the wait for its pipes.
func command(ctx context.Context, bin string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // G204: configured binary, arguments built by this package
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	cmd.Dir = os.TempDir()
	cmd.WaitDelay = waitDelay
	return cmd
}

func truncate(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

// selectStreams checks the probe against the decoding limits and picks the
// first video stream and the first audio stream.
func selectStreams(demuxer string, p probeJSON) (selection, error) {
	if !strings.HasPrefix(p.Format.FormatName, demuxer) {
		return selection{}, fmt.Errorf("%w: format %q", ErrInvalid, p.Format.FormatName)
	}
	if len(p.Streams) > maxStreams {
		return selection{}, fmt.Errorf("%w: %d streams", ErrInvalid, len(p.Streams))
	}
	sel := selection{video: -1, audio: -1}
	av := 0
	for _, s := range p.Streams {
		switch {
		case s.CodecType == "video" && s.Disposition.AttachedPic == 0:
			if !videoCodecs[s.CodecName] {
				return selection{}, fmt.Errorf("%w: video codec %q", ErrInvalid, s.CodecName)
			}
			if s.Width < 2 || s.Height < 2 || s.Width > maxDimension || s.Height > maxDimension {
				return selection{}, fmt.Errorf("%w: %dx%d", ErrInvalid, s.Width, s.Height)
			}
			if sel.video < 0 {
				sel.video, sel.width, sel.height = s.Index, s.Width, s.Height
			}
			av++
		case s.CodecType == "audio":
			if !audioCodecs[s.CodecName] {
				return selection{}, fmt.Errorf("%w: audio codec %q", ErrInvalid, s.CodecName)
			}
			if sel.audio < 0 {
				sel.audio = s.Index
			}
			av++
		}
	}
	if av > maxAVStreams {
		return selection{}, fmt.Errorf("%w: %d audio and video streams", ErrInvalid, av)
	}
	if sel.video < 0 {
		return selection{}, fmt.Errorf("%w: no video stream", ErrInvalid)
	}
	return sel, nil
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
