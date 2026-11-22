// Package media sniffs uploaded media types and removes privacy-sensitive
// metadata (EXIF/GPS, XMP, text chunks) from images without re-encoding them.
package media

import (
	"bytes"
	"encoding/binary"
)

const (
	TypeJPEG      = "image/jpeg"
	TypePNG       = "image/png"
	TypeWebP      = "image/webp"
	TypeMP4       = "video/mp4"
	TypeQuickTime = "video/quicktime"
	TypeWebM      = "video/webm"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

// ImageType returns the content type of a JPEG, PNG or WebP image identified by
// its magic number, or "" for anything else.
func ImageType(data []byte) string {
	switch {
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return TypeJPEG
	case len(data) >= 8 && bytes.Equal(data[:8], pngSignature):
		return TypePNG
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return TypeWebP
	}
	return ""
}

// imageBrands are ISO base media major brands of still images (HEIF/HEIC and
// AVIF, including image sequences). They share the ftyp container with video
// but must not be accepted or served as video.
var imageBrands = map[string]bool{
	"heic": true, "heix": true, "heim": true, "heis": true,
	"mif1": true, "msf1": true, "avif": true, "avis": true,
}

// VideoType identifies MP4, QuickTime (MOV) and WebM video by magic number.
// Every ISO base media file (ftyp) is video/mp4, or video/quicktime for the
// "qt  " brand, except the still-image brands in imageBrands (so a HEIC photo
// is not treated as a video while 3GP or HEVC phone videos still are).
func VideoType(data []byte) (contentType, ext string) {
	if len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")) {
		brand := string(data[8:12])
		if imageBrands[brand] {
			return "", ""
		}
		if brand == "qt  " {
			return TypeQuickTime, ".mov"
		}
		return TypeMP4, ".mp4"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) {
		return TypeWebM, ".webm"
	}
	return "", ""
}

// StripImageMetadata removes metadata from JPEG, PNG and WebP images. Other data, and
// images it cannot parse, are returned unchanged. The pixel data is never
// re-encoded.
func StripImageMetadata(data []byte) []byte {
	switch ImageType(data) {
	case TypeJPEG:
		if out, ok := stripJPEG(data); ok {
			return out
		}
	case TypePNG:
		if out, ok := stripPNG(data); ok {
			return out
		}
	case TypeWebP:
		if out, ok := stripWebP(data); ok {
			return out
		}
	}
	return data
}

// VP8X feature flags (first byte of the VP8X chunk payload).
const (
	webpFlagEXIF = 0x08
	webpFlagXMP  = 0x04
)

// stripWebP drops the EXIF and "XMP " chunks of an extended (VP8X) WebP,
// clears the matching VP8X flags and rewrites the RIFF size.
func stripWebP(data []byte) ([]byte, bool) {
	if len(data) < 12 {
		return nil, false
	}
	riffSize := int(binary.LittleEndian.Uint32(data[4:8]))
	if riffSize+8 > len(data) || riffSize < 4 {
		return nil, false
	}
	body := data[12 : riffSize+8]
	out := make([]byte, 12, len(data))
	copy(out, data[:12])
	vp8xFlags := -1
	removed := false
	for pos := 0; pos < len(body); {
		if pos+8 > len(body) {
			return nil, false
		}
		fourCC := string(body[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(body[pos+4 : pos+8]))
		end := pos + 8 + size + size%2 // chunks are padded to an even size
		if size < 0 || end > len(body) || end < pos {
			if pos+8+size == len(body) {
				end = len(body) // tolerate a missing final pad byte
			} else {
				return nil, false
			}
		}
		switch fourCC {
		case "EXIF", "XMP ":
			removed = true
		default:
			if fourCC == "VP8X" && size >= 1 {
				vp8xFlags = len(out) + 8
			}
			out = append(out, body[pos:end]...)
		}
		pos = end
	}
	if !removed {
		return data, true
	}
	if vp8xFlags >= 0 {
		out[vp8xFlags] &^= webpFlagEXIF | webpFlagXMP
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out, true
}

// stripJPEG drops APP1 (Exif, XMP) and APP13 (Photoshop/IPTC) segments and
// comments before the image data. The EXIF orientation, when present, is kept
// in a minimal new Exif segment so photos are not shown rotated.
func stripJPEG(data []byte) ([]byte, bool) {
	out := make([]byte, 0, len(data))
	out = append(out, 0xff, 0xd8)
	orientation := uint16(0)
	insertAt := -1
	pos := 2
	for {
		if pos >= len(data) || data[pos] != 0xff {
			return nil, false
		}
		markerStart := pos
		for pos < len(data) && data[pos] == 0xff {
			pos++
		}
		if pos >= len(data) {
			return nil, false
		}
		marker := data[pos]
		pos++
		// Markers without a length field.
		if marker == 0xd9 || marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			out = append(out, data[markerStart:pos]...)
			if marker == 0xd9 {
				out = append(out, data[pos:]...)
				break
			}
			continue
		}
		if pos+2 > len(data) {
			return nil, false
		}
		length := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if length < 2 || pos+length > len(data) {
			return nil, false
		}
		segment := data[markerStart : pos+length]
		payload := data[pos+2 : pos+length]
		pos += length
		if marker == 0xda {
			// Start of scan: everything after it is image data.
			if insertAt < 0 {
				insertAt = len(out)
			}
			out = append(out, segment...)
			out = append(out, data[pos:]...)
			break
		}
		switch {
		case marker == 0xe1:
			if orientation == 0 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
				orientation = exifOrientation(payload[6:])
			}
			if insertAt < 0 {
				insertAt = len(out)
			}
			continue
		case marker == 0xed || marker == 0xfe:
			continue
		case marker == 0xe0 || marker == 0xe2 || marker == 0xee:
			// JFIF, ICC profile and Adobe segments stay; the new Exif segment
			// goes after them.
		default:
			if insertAt < 0 {
				insertAt = len(out)
			}
		}
		out = append(out, segment...)
	}
	if orientation > 1 && orientation <= 8 {
		if insertAt < 0 {
			insertAt = len(out)
		}
		segment := orientationExifSegment(orientation)
		withExif := make([]byte, 0, len(out)+len(segment))
		withExif = append(withExif, out[:insertAt]...)
		withExif = append(withExif, segment...)
		withExif = append(withExif, out[insertAt:]...)
		out = withExif
	}
	return out, true
}

// exifOrientation reads tag 0x0112 from IFD0 of a TIFF structure.
func exifOrientation(tiff []byte) uint16 {
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
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[ifd : ifd+2]))
	for i := 0; i < count; i++ {
		entry := ifd + 2 + i*12
		if entry+12 > len(tiff) {
			return 0
		}
		if order.Uint16(tiff[entry:entry+2]) == 0x0112 && order.Uint16(tiff[entry+2:entry+4]) == 3 {
			return order.Uint16(tiff[entry+8 : entry+10])
		}
	}
	return 0
}

// orientationExifSegment builds an APP1 Exif segment whose only tag is the
// orientation.
func orientationExifSegment(orientation uint16) []byte {
	tiff := []byte{'M', 'M', 0, 0x2a, 0, 0, 0, 8, // header, IFD0 at offset 8
		0, 1, // one entry
		0x01, 0x12, 0, 3, 0, 0, 0, 1, byte(orientation >> 8), byte(orientation), 0, 0, // Orientation SHORT
		0, 0, 0, 0} // no next IFD
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
	return append(segment, payload...)
}

var pngMetadataChunks = map[string]bool{"eXIf": true, "tEXt": true, "iTXt": true, "zTXt": true}

func stripPNG(data []byte) ([]byte, bool) {
	out := make([]byte, 0, len(data))
	out = append(out, pngSignature...)
	pos := len(pngSignature)
	for pos < len(data) {
		if pos+12 > len(data) {
			return nil, false
		}
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		end := pos + 12 + length
		if length < 0 || end > len(data) || end < pos {
			return nil, false
		}
		chunkType := string(data[pos+4 : pos+8])
		if !pngMetadataChunks[chunkType] {
			out = append(out, data[pos:end]...)
		}
		pos = end
		if chunkType == "IEND" {
			break
		}
	}
	return out, true
}
