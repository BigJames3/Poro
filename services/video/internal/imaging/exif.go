package imaging

import (
	"bytes"
	"encoding/binary"
)

const orientationTag = 0x0112

// Orientation reads the EXIF orientation (1 to 8) of a JPEG (APP1), PNG
// (eXIf chunk) or WebP (EXIF chunk). Anything missing, malformed or out of
// range reads as 1: a broken EXIF block never fails an image, it only
// leaves it unrotated. Every offset is bounds-checked.
func Orientation(raw []byte, kind string) int {
	var tiff []byte
	switch kind {
	case "image/jpeg":
		tiff = jpegExif(raw)
	case "image/png":
		tiff = pngExif(raw)
	case "image/webp":
		tiff = webpExif(raw)
	}
	if o := tiffOrientation(tiff); o >= 1 && o <= 8 {
		return o
	}
	return 1
}

var exifHeader = []byte("Exif\x00\x00")

// jpegExif returns the TIFF block of the first APP1 Exif segment, stopping
// at the start of the image data.
func jpegExif(raw []byte) []byte {
	i := 2
	for i+4 <= len(raw) {
		if raw[i] != 0xFF {
			return nil
		}
		marker := raw[i+1]
		if marker == 0xFF {
			i++ // fill byte
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // start of scan, end of image
			return nil
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2 // markers without a length
			continue
		}
		length := int(binary.BigEndian.Uint16(raw[i+2 : i+4]))
		if length < 2 || i+2+length > len(raw) {
			return nil
		}
		data := raw[i+4 : i+2+length]
		if marker == 0xE1 && bytes.HasPrefix(data, exifHeader) {
			return data[len(exifHeader):]
		}
		i += 2 + length
	}
	return nil
}

// pngExif returns the data of the eXIf chunk.
func pngExif(raw []byte) []byte {
	i := 8
	for i+8 <= len(raw) {
		length := int(binary.BigEndian.Uint32(raw[i : i+4]))
		kind := string(raw[i+4 : i+8])
		if length < 0 || i+12+length > len(raw) {
			return nil
		}
		if kind == "eXIf" {
			return raw[i+8 : i+8+length]
		}
		if kind == "IEND" {
			return nil
		}
		i += 12 + length
	}
	return nil
}

// webpExif returns the data of the EXIF chunk of an extended WebP. Some
// writers keep the "Exif\0\0" prefix of JPEG: it is skipped.
func webpExif(raw []byte) []byte {
	i := 12
	for i+8 <= len(raw) {
		kind := string(raw[i : i+4])
		length := int(binary.LittleEndian.Uint32(raw[i+4 : i+8]))
		if length < 0 || i+8+length > len(raw) {
			return nil
		}
		if kind == "EXIF" {
			return bytes.TrimPrefix(raw[i+8:i+8+length], exifHeader)
		}
		i += 8 + length + length%2
	}
	return nil
}

// tiffOrientation reads tag 0x0112 of IFD0 of a TIFF block; 0 if absent.
func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 0
	}
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[ifd : ifd+2]))
	for n := 0; n < count; n++ {
		entry := ifd + 2 + n*12
		if entry+12 > len(tiff) {
			return 0
		}
		if order.Uint16(tiff[entry:entry+2]) != orientationTag {
			continue
		}
		// SHORT, count 1: the value sits in the first two bytes of the field.
		if order.Uint16(tiff[entry+2:entry+4]) != 3 || order.Uint32(tiff[entry+4:entry+8]) != 1 {
			return 0
		}
		return int(order.Uint16(tiff[entry+8 : entry+10]))
	}
	return 0
}
