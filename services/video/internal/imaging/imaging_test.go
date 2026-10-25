package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return raw
}

func process(t *testing.T, raw []byte, kind string) *Result {
	t.Helper()
	res, err := Process(raw, kind, int64(len(raw)))
	require.NoError(t, err)
	return res
}

func decoded(t *testing.T, raw []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(raw))
	require.NoError(t, err, "outputs are JPEG")
	return img
}

func isRed(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return r>>8 > 170 && g>>8 < 90 && b>>8 < 90
}

func isBlue(img image.Image, x, y int) bool {
	r, _, b, _ := img.At(x, y).RGBA()
	return b>>8 > 170 && r>>8 < 90
}

// noMetadata checks that a JPEG output carries no EXIF (so no GPS), XMP or
// ICC segment: nothing of the original but its pixels.
func noMetadata(t *testing.T, out []byte) {
	t.Helper()
	require.False(t, bytes.Contains(out, []byte("Exif")), "no EXIF block")
	require.False(t, bytes.Contains(out, []byte("http://ns.adobe.com/xap")), "no XMP")
	require.False(t, bytes.Contains(out, []byte("ICC_PROFILE")), "no ICC profile")
	for i := 0; i+1 < len(out); i++ {
		if out[i] == 0xFF && out[i+1] == 0xDA {
			break // start of scan: no metadata segment after this point
		}
		if out[i] == 0xFF && out[i+1] >= 0xE1 && out[i+1] <= 0xEF {
			t.Fatalf("APP%d segment at %d", out[i+1]-0xE0, i)
		}
	}
}

// Every orientation of a phone photo displays upright: the red block stored
// in the top-left corner lands where EXIF says it is displayed.
func TestEveryExifOrientationIsApplied(t *testing.T) {
	type corner struct{ x, y int }
	tl, tr, br, bl := corner{0, 0}, corner{1, 0}, corner{1, 1}, corner{0, 1}
	cases := map[int]struct {
		w, h int
		red  corner
	}{
		1: {64, 32, tl}, 2: {64, 32, tr}, 3: {64, 32, br}, 4: {64, 32, bl},
		5: {32, 64, tl}, 6: {32, 64, tr}, 7: {32, 64, br}, 8: {32, 64, bl},
	}
	for o, c := range cases {
		raw := fixture(t, "jpeg_o"+string(rune('0'+o))+".jpg")
		require.Equal(t, o, Orientation(raw, "image/jpeg"), "orientation %d is read", o)
		res := process(t, raw, "image/jpeg")
		require.Equal(t, c.w, res.Width, "orientation %d width", o)
		require.Equal(t, c.h, res.Height, "orientation %d height", o)
		img := decoded(t, res.Image)
		require.Equal(t, image.Rect(0, 0, c.w, c.h), img.Bounds())
		at := func(k corner) (int, int) { return 4 + k.x*(c.w-9), 4 + k.y*(c.h-9) }
		x, y := at(c.red)
		require.True(t, isRed(img, x, y), "orientation %d: red block at (%d,%d)", o, x, y)
		ox, oy := at(corner{1 - c.red.x, 1 - c.red.y})
		require.True(t, isBlue(img, ox, oy), "orientation %d: opposite corner stays blue", o)
		noMetadata(t, res.Image)
		noMetadata(t, res.Thumbnail)
	}
}

func TestPngAndWebpAreOrientedToo(t *testing.T) {
	for name, kind := range map[string]string{"png_o6.png": "image/png", "webp_o6.webp": "image/webp"} {
		raw := fixture(t, name)
		require.Equal(t, 6, Orientation(raw, kind), name)
		res := process(t, raw, kind)
		require.Equal(t, 32, res.Width, name)
		require.Equal(t, 64, res.Height, name)
		img := decoded(t, res.Image)
		require.True(t, isRed(img, 27, 4), "%s: red block top right after rotation", name)
		noMetadata(t, res.Image)
	}
}

func TestTheSourceReallyCarriesGPSAndTheOutputDoesNot(t *testing.T) {
	raw := fixture(t, "jpeg_o1.jpg")
	require.True(t, bytes.Contains(raw, []byte("Exif")), "the fixture has an EXIF block")
	tiff := jpegExif(raw)
	require.NotNil(t, tiff)
	require.True(t, hasTag(tiff, 0x8825), "the fixture has a GPS IFD pointer")
	res := process(t, raw, "image/jpeg")
	noMetadata(t, res.Image)
	require.Nil(t, jpegExif(res.Image))
}

// hasTag reports whether IFD0 of a little-endian TIFF block holds tag.
func hasTag(tiff []byte, tag uint16) bool {
	order := binary.ByteOrder(binary.LittleEndian)
	if string(tiff[:2]) == "MM" {
		order = binary.BigEndian
	}
	ifd := int(order.Uint32(tiff[4:8]))
	count := int(order.Uint16(tiff[ifd : ifd+2]))
	for n := 0; n < count; n++ {
		if order.Uint16(tiff[ifd+2+n*12:]) == tag {
			return true
		}
	}
	return false
}

func TestTransparencyIsFlattenedOnWhite(t *testing.T) {
	for name, kind := range map[string]string{"png_alpha.png": "image/png", "webp_alpha.webp": "image/webp"} {
		res := process(t, fixture(t, name), kind)
		img := decoded(t, res.Image)
		r, g, b, _ := img.At(50, 16).RGBA()
		require.True(t, r>>8 > 240 && g>>8 > 240 && b>>8 > 240, "%s: transparent half is white", name)
		require.True(t, isRed(img, 4, 4), "%s: opaque pixels kept", name)
	}
}

func TestLargeImagesAreReducedSmallOnesNotEnlarged(t *testing.T) {
	big := process(t, fixture(t, "big.jpg"), "image/jpeg")
	require.Equal(t, 2048, big.Width)
	require.Equal(t, 683, big.Height, "3000x1000 keeps its ratio (682.67 rounded)")
	thumb := decoded(t, big.Thumbnail)
	require.Equal(t, image.Rect(0, 0, 540, 180), thumb.Bounds())

	small := process(t, fixture(t, "small.jpg"), "image/jpeg")
	require.Equal(t, 800, small.Width)
	require.Equal(t, 600, small.Height, "never enlarged")
	require.Equal(t, image.Rect(0, 0, 540, 405), decoded(t, small.Thumbnail).Bounds())
}

// pngHeader builds a valid PNG signature and IHDR of the given size, followed
// by an empty IDAT: enough for DecodeConfig, never decodable.
func pngHeader(w, h uint32) []byte {
	chunk := func(kind string, data []byte) []byte {
		var b bytes.Buffer
		_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.WriteString(kind)
		b.Write(data)
		_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), data...)))
		return b.Bytes()
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // 8 bits, RGBA
	out := []byte("\x89PNG\r\n\x1a\n")
	out = append(out, chunk("IHDR", ihdr)...)
	out = append(out, chunk("IDAT", nil)...)
	return append(out, chunk("IEND", nil)...)
}

func TestHostileOrWrongFilesAreRefusedBeforeDecoding(t *testing.T) {
	jpeg1 := fixture(t, "jpeg_o1.jpg")
	heic := append([]byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic"), make([]byte, 64)...)
	cases := map[string]struct {
		raw      []byte
		declared string
		size     int64
	}{
		"bomb 8000x6000 (48 Mpx)":          {pngHeader(8000, 6000), "image/png", 0},
		"zero width":                       {pngHeader(0, 10), "image/png", 0},
		"replaced by a png after complete": {fixture(t, "png_o6.png"), "image/jpeg", 0},
		"replaced by a heic":               {heic, "image/jpeg", 0},
		"replaced by a longer file":        {append(append([]byte{}, jpeg1...), 0), "image/jpeg", int64(len(jpeg1))},
		"truncated jpeg":                   {jpeg1[:len(jpeg1)/2], "image/jpeg", 0},
		"empty":                            {nil, "image/jpeg", 0},
		"html":                             {[]byte("<html><body>x</body></html>"), "image/png", 0},
	}
	for name, c := range cases {
		size := c.size
		if size == 0 {
			size = int64(len(c.raw))
		}
		res, err := Process(c.raw, c.declared, size)
		require.Nil(t, res, name)
		require.True(t, errors.Is(err, ErrInvalid), "%s: %v", name, err)
	}
	// The bomb is refused on its header, before any pixel is decoded.
	_, err := Process(pngHeader(8000, 6000), "image/png", int64(len(pngHeader(8000, 6000))))
	require.ErrorContains(t, err, "8000x6000 pixels", "refused on the size read from the header")
	// 40 Mpx exactly is allowed through the header check (it is not decoded here).
	_, err = Process(pngHeader(8000, 5000), "image/png", int64(len(pngHeader(8000, 5000))))
	require.ErrorIs(t, err, ErrInvalid)
	require.NotContains(t, err.Error(), "pixels", "40 Mpx passes the size check, then fails to decode the empty data")
}

func TestBrokenExifNeverFailsAnImage(t *testing.T) {
	raw := append([]byte{}, fixture(t, "jpeg_o6.jpg")...)
	i := bytes.Index(raw, []byte("Exif\x00\x00"))
	require.Positive(t, i)
	// Corrupt the TIFF byte order and the IFD offset.
	raw[i+6], raw[i+7] = 'X', 'X'
	require.Equal(t, 1, Orientation(raw, "image/jpeg"))
	res := process(t, raw, "image/jpeg")
	require.Equal(t, 64, res.Width, "unrotated, not refused")

	for _, junk := range [][]byte{nil, {0xFF, 0xD8}, {0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF}} {
		require.Equal(t, 1, Orientation(junk, "image/jpeg"))
	}
	require.Equal(t, 1, Orientation([]byte("\x89PNG\r\n\x1a\n\x00\x00\xff\xffeXIf"), "image/png"))
	require.Equal(t, 1, Orientation([]byte("RIFF\x00\x00\x00\x00WEBPEXIF\xff\xff\xff\x7f"), "image/webp"))
}
