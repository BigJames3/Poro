// Package imaging turns the stored original of a photo post image into its
// public JPEG and thumbnail (ADR-0020). It trusts nothing that was checked
// before: the presigned PUT stays valid for an hour, so the object may have
// been replaced after complete. Every check runs again here, on the bytes
// the worker read, before any decoding.
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"

	"github.com/poro/video/internal/media"
	"github.com/poro/video/internal/model"
)

const (
	// MaxPixels refuses decompression bombs: the dimensions are read from
	// the header before any pixel is decoded.
	MaxPixels = 40_000_000
	// MaxSide is the long side of the public image; smaller images are
	// never enlarged.
	MaxSide = 2048
	// ThumbSide is the long side of the thumbnail.
	ThumbSide = 540
	// Quality and ThumbQuality are the JPEG qualities of both outputs.
	Quality      = 85
	ThumbQuality = 80
)

// ErrInvalid marks an image that is not what was declared or cannot be
// decoded. The whole photo post fails on it.
var ErrInvalid = errors.New("invalid image")

// Result holds the encoded outputs and the dimensions of Image, after
// orientation and reduction.
type Result struct {
	Image     []byte
	Thumbnail []byte
	Width     int
	Height    int
}

// Process revalidates raw against the declared type and size, then decodes
// it, applies its EXIF orientation, flattens transparency on white, reduces
// it to MaxSide and re-encodes it as JPEG. The output is built from pixels
// only: no EXIF, GPS, XMP or ICC data of the original survives.
func Process(raw []byte, declaredType string, declaredSize int64) (res *Result, err error) {
	if int64(len(raw)) != declaredSize || int64(len(raw)) > model.MaxImageBytes || len(raw) == 0 {
		return nil, fmt.Errorf("%w: %d bytes stored, %d declared", ErrInvalid, len(raw), declaredSize)
	}
	kind := media.DetectImageType(raw)
	if kind == "" || kind != declaredType {
		return nil, fmt.Errorf("%w: content is %q, declared %q", ErrInvalid, kind, declaredType)
	}
	decodeConfig, decode := codec(kind)
	cfg, err := decodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: header: %w", ErrInvalid, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, fmt.Errorf("%w: %dx%d pixels", ErrInvalid, cfg.Width, cfg.Height)
	}
	// A decoder bug on hostile input must fail this image, not the worker.
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("%w: decoder panic: %v", ErrInvalid, r)
		}
	}()
	src, err := decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: decode: %w", ErrInvalid, err)
	}
	b := src.Bounds()
	if b.Dx() != cfg.Width || b.Dy() != cfg.Height {
		return nil, fmt.Errorf("%w: decoded %dx%d, header %dx%d", ErrInvalid, b.Dx(), b.Dy(), cfg.Width, cfg.Height)
	}

	// Reduce first, then orient: the long side bound holds either way and
	// the pixel loop of the rotation runs on 4 Mpx at most.
	w, h := fit(b.Dx(), b.Dy(), MaxSide)
	full := orient(flatten(src, w, h), Orientation(raw, kind))
	image1, err := encode(full, Quality)
	if err != nil {
		return nil, err
	}
	tw, th := fit(full.Rect.Dx(), full.Rect.Dy(), ThumbSide)
	thumb, err := encode(flatten(full, tw, th), ThumbQuality)
	if err != nil {
		return nil, err
	}
	return &Result{Image: image1, Thumbnail: thumb, Width: full.Rect.Dx(), Height: full.Rect.Dy()}, nil
}

type (
	configFunc func(r *bytes.Reader) (image.Config, error)
	decodeFunc func(r *bytes.Reader) (image.Image, error)
)

// codec picks the decoder of the detected type only: the content can never
// select another format.
func codec(kind string) (configFunc, decodeFunc) {
	switch kind {
	case "image/png":
		return func(r *bytes.Reader) (image.Config, error) { return png.DecodeConfig(r) },
			func(r *bytes.Reader) (image.Image, error) { return png.Decode(r) }
	case "image/webp":
		return func(r *bytes.Reader) (image.Config, error) { return webp.DecodeConfig(r) },
			func(r *bytes.Reader) (image.Image, error) { return webp.Decode(r) }
	default:
		return func(r *bytes.Reader) (image.Config, error) { return jpeg.DecodeConfig(r) },
			func(r *bytes.Reader) (image.Image, error) { return jpeg.Decode(r) }
	}
}

// fit returns w×h reduced so that its long side is at most side, keeping the
// aspect ratio; it never enlarges.
func fit(w, h, side int) (int, int) {
	if w <= side && h <= side {
		return w, h
	}
	// Rounded to the nearest pixel, so the ratio drifts by half a pixel at most.
	if w >= h {
		return side, max(1, int((int64(h)*int64(side)+int64(w)/2)/int64(w)))
	}
	return max(1, int((int64(w)*int64(side)+int64(h)/2)/int64(h))), side
}

// flatten draws src scaled to w×h over an opaque white background.
func flatten(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, xdraw.Src)
	if src.Bounds().Dx() == w && src.Bounds().Dy() == h {
		xdraw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, xdraw.Over)
		return dst
	}
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func encode(img image.Image, quality int) ([]byte, error) {
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return out.Bytes(), nil
}

// orient applies an EXIF orientation (1 to 8) so the image displays upright.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var u, v int
			switch o {
			case 2:
				u, v = w-1-x, y
			case 3:
				u, v = w-1-x, h-1-y
			case 4:
				u, v = x, h-1-y
			case 5:
				u, v = y, x
			case 6:
				u, v = h-1-y, x
			case 7:
				u, v = h-1-y, w-1-x
			case 8:
				u, v = y, w-1-x
			}
			si := y*src.Stride + x*4
			di := v*dst.Stride + u*4
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}
