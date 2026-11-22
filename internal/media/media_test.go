package media

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodeJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		img.Set(x, x, color.RGBA{R: 200, A: 255})
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func segment(marker byte, payload []byte) []byte {
	out := []byte{0xff, marker, 0, 0}
	binary.BigEndian.PutUint16(out[2:], uint16(len(payload)+2))
	return append(out, payload...)
}

// exifWithGPS builds a little-endian Exif payload with Orientation=6 and a
// GPS IFD pointer plus a recognisable marker string.
func exifWithGPS() []byte {
	tiff := []byte{'I', 'I', 0x2a, 0, 8, 0, 0, 0, 2, 0,
		0x12, 0x01, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, // Orientation = 6
		0x25, 0x88, 4, 0, 1, 0, 0, 0, 38, 0, 0, 0, // GPS IFD pointer
		0, 0, 0, 0}
	tiff = append(tiff, []byte("GPS-SECRET-42.87N-74.59E")...)
	return append([]byte("Exif\x00\x00"), tiff...)
}

func TestStripJPEGRemovesExifXMPAndKeepsOrientation(t *testing.T) {
	original := encodeJPEG(t)
	withMeta := append([]byte{0xff, 0xd8}, segment(0xe1, exifWithGPS())...)
	withMeta = append(withMeta, segment(0xe1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>XMP-SECRET</x:xmpmeta>"))...)
	withMeta = append(withMeta, segment(0xfe, []byte("COMMENT-SECRET"))...)
	withMeta = append(withMeta, original[2:]...)

	out := StripImageMetadata(withMeta)
	for _, secret := range []string{"GPS-SECRET", "XMP-SECRET", "COMMENT-SECRET"} {
		if bytes.Contains(out, []byte(secret)) {
			t.Fatalf("%s survived stripping", secret)
		}
	}
	if !bytes.Contains(out, []byte("Exif\x00\x00MM")) {
		t.Fatal("orientation Exif segment was not re-added")
	}
	idx := bytes.Index(out, []byte("Exif\x00\x00"))
	if got := exifOrientation(out[idx+6:]); got != 6 {
		t.Fatalf("orientation = %d, want 6", got)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("stripped JPEG does not decode: %v", err)
	}
	// Image data is untouched (no re-encoding).
	if !bytes.HasSuffix(out, original[len(original)-200:]) {
		t.Fatal("image data changed")
	}
}

func TestStripJPEGWithoutMetadataIsUnchanged(t *testing.T) {
	original := encodeJPEG(t)
	if out := StripImageMetadata(original); !bytes.Equal(out, original) {
		t.Fatal("JPEG without metadata changed")
	}
}

func pngChunk(kind string, data []byte) []byte {
	out := make([]byte, 8, 12+len(data))
	binary.BigEndian.PutUint32(out[:4], uint32(len(data)))
	copy(out[4:], kind)
	out = append(out, data...)
	crc := crc32.ChecksumIEEE(append([]byte(kind), data...))
	return binary.BigEndian.AppendUint32(out, crc)
}

func TestStripPNGRemovesTextAndExifChunks(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	encoded := buf.Bytes()
	// Insert metadata chunks after IHDR (8 + 25 bytes).
	withMeta := append([]byte{}, encoded[:33]...)
	withMeta = append(withMeta, pngChunk("tEXt", []byte("Comment\x00TEXT-SECRET"))...)
	withMeta = append(withMeta, pngChunk("eXIf", []byte("MM\x00\x2aEXIF-SECRET"))...)
	withMeta = append(withMeta, pngChunk("iTXt", []byte("XML\x00\x00\x00\x00\x00ITXT-SECRET"))...)
	withMeta = append(withMeta, pngChunk("zTXt", []byte("k\x00\x00ZTXT-SECRET"))...)
	withMeta = append(withMeta, encoded[33:]...)

	out := StripImageMetadata(withMeta)
	if !bytes.Equal(out, encoded) {
		t.Fatalf("stripped PNG differs from the original encoding")
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("stripped PNG does not decode: %v", err)
	}
}

func TestMalformedImagesAreReturnedUnchanged(t *testing.T) {
	for _, data := range [][]byte{
		append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 64)...),
		{0xff, 0xd8, 0xff, 0xe1, 0xff, 0xff},
		[]byte("not an image"),
	} {
		if out := StripImageMetadata(data); !bytes.Equal(out, data) {
			t.Fatalf("malformed input changed: %x", data)
		}
	}
}

func TestVideoTypeAcceptsOnlyRealVideoBrands(t *testing.T) {
	ftyp := func(brand string) []byte {
		return append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p'}, []byte(brand)...)
	}
	cases := map[string]string{
		"isom": TypeMP4, "mp42": TypeMP4, "qt  ": TypeQuickTime, "3gp4": TypeMP4, "hvc1": TypeMP4,
		"heic": "", "heix": "", "heim": "", "heis": "", "mif1": "", "msf1": "", "avif": "", "avis": "",
	}
	for brand, want := range cases {
		if got, _ := VideoType(ftyp(brand)); got != want {
			t.Errorf("brand %q: got %q, want %q", brand, got, want)
		}
	}
	if got, ext := VideoType([]byte{0x1a, 0x45, 0xdf, 0xa3, 0, 0}); got != TypeWebM || ext != ".webm" {
		t.Errorf("webm: got %q %q", got, ext)
	}
}

func webpChunk(fourCC string, data []byte) []byte {
	out := append([]byte(fourCC), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(data)))
	out = append(out, data...)
	if len(data)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

func riff(chunks ...[]byte) []byte {
	body := []byte("WEBP")
	for _, c := range chunks {
		body = append(body, c...)
	}
	out := append([]byte("RIFF"), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(body)))
	return append(out, body...)
}

func TestStripWebPRemovesExifAndXMP(t *testing.T) {
	vp8x := make([]byte, 10)
	vp8x[0] = 0x10 | webpFlagEXIF | webpFlagXMP // alpha + EXIF + XMP
	image := webpChunk("VP8L", []byte{0x2f, 1, 2, 3, 4})
	withMeta := riff(webpChunk("VP8X", vp8x), webpChunk("ICCP", []byte("icc")), image,
		webpChunk("EXIF", []byte("GPS-SECRET")), webpChunk("XMP ", []byte("XMP-SECRET!")))

	out := StripImageMetadata(withMeta)
	if bytes.Contains(out, []byte("SECRET")) {
		t.Fatal("WebP metadata survived")
	}
	want := make([]byte, 10)
	want[0] = 0x10
	expected := riff(webpChunk("VP8X", want), webpChunk("ICCP", []byte("icc")), image)
	if !bytes.Equal(out, expected) {
		t.Fatalf("stripped WebP:\n got %x\nwant %x", out, expected)
	}
	if got := binary.LittleEndian.Uint32(out[4:8]); int(got) != len(out)-8 {
		t.Fatalf("RIFF size %d, want %d", got, len(out)-8)
	}

	simple := riff(webpChunk("VP8 ", []byte{1, 2, 3, 4}))
	if got := StripImageMetadata(simple); !bytes.Equal(got, simple) {
		t.Fatal("simple WebP without metadata changed")
	}
	truncated := withMeta[:len(withMeta)-5]
	if got := StripImageMetadata(truncated); !bytes.Equal(got, truncated) {
		t.Fatal("malformed WebP must be returned unchanged")
	}
}

func TestImageType(t *testing.T) {
	if ImageType(encodeJPEG(t)) != TypeJPEG {
		t.Error("jpeg not detected")
	}
	if ImageType([]byte("RIFF\x00\x00\x00\x00WEBPVP8 ")) != TypeWebP {
		t.Error("webp not detected")
	}
	if ImageType([]byte("GIF89a......")) != "" {
		t.Error("gif must be rejected")
	}
}
