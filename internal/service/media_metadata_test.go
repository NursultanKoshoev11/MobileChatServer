package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"strings"
	"testing"
)

// jpegWithGPSExif returns a valid JPEG carrying an APP1 Exif segment with a
// recognisable secret.
func jpegWithGPSExif(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatal(err)
	}
	payload := append([]byte("Exif\x00\x00II\x2a\x00\x08\x00\x00\x00\x00\x00\x00\x00\x00\x00"), []byte("GPS-SECRET")...)
	segment := []byte{0xff, 0xe1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	out := append([]byte{0xff, 0xd8}, segment...)
	out = append(out, payload...)
	return append(out, buf.Bytes()[2:]...)
}

func TestSanitizeImageDataURLStripsExifAndChecksMagic(t *testing.T) {
	data := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpegWithGPSExif(t))
	out, err := sanitizeImageDataURL(data, maxAvatarBytes, "avatar")
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(out, "data:image/jpeg;base64,"))
	if bytes.Contains(decoded, []byte("GPS-SECRET")) {
		t.Fatal("EXIF survived")
	}
	// Declared PNG but actually JPEG: stored with the real type.
	relabeled := "data:image/png;base64," + base64.StdEncoding.EncodeToString(jpegWithGPSExif(t))
	if out, err := sanitizeImageDataURL(relabeled, maxAvatarBytes, "avatar"); err != nil || !strings.HasPrefix(out, "data:image/jpeg;base64,") {
		t.Fatalf("relabeled image: %q %v", out[:30], err)
	}
	// Not an image at all, whatever the declared type.
	html := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("<html><script>alert(1)</script>"))
	if _, err := sanitizeImageDataURL(html, maxAvatarBytes, "avatar"); err == nil || !strings.Contains(err.Error(), "JPEG, PNG, or WebP") {
		t.Fatalf("expected magic-number rejection, got %v", err)
	}
}

func TestUpdateUserAvatarRejectsNonImageBytes(t *testing.T) {
	svc := New(nil, Config{})
	_, err := svc.UpdateUserAvatar(context.Background(), "U-1", "data:image/webp;base64,"+base64.StdEncoding.EncodeToString([]byte("GIF89a-not-webp")))
	if err == nil || !strings.Contains(err.Error(), "JPEG, PNG, or WebP") {
		t.Fatalf("expected magic-number rejection, got %v", err)
	}
}

func TestStripInlinePublicRequestPhotoMetadata(t *testing.T) {
	photo := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpegWithGPSExif(t))
	body, _ := json.Marshal(map[string]any{
		"version":         1,
		"text":            "hello",
		"moderation_text": "kept",
		"photos":          []map[string]any{{"name": "a.jpg", "size_bytes": 1, "base64": photo, "extra": true}},
	})
	out := stripInlinePublicRequestPhotoMetadata(string(body))
	var parsed struct {
		Text           string `json:"text"`
		ModerationText string `json:"moderation_text"`
		Photos         []struct {
			Base64    string `json:"base64"`
			SizeBytes int    `json:"size_bytes"`
			Extra     bool   `json:"extra"`
		} `json:"photos"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if parsed.Text != "hello" || parsed.ModerationText != "kept" || len(parsed.Photos) != 1 || !parsed.Photos[0].Extra {
		t.Fatalf("payload fields not preserved: %s", out)
	}
	if !strings.HasPrefix(parsed.Photos[0].Base64, "data:image/jpeg;base64,") {
		t.Fatalf("data URL prefix lost")
	}
	decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(parsed.Photos[0].Base64, "data:image/jpeg;base64,"))
	if bytes.Contains(decoded, []byte("GPS-SECRET")) || parsed.Photos[0].SizeBytes != len(decoded) {
		t.Fatalf("metadata not stripped or size not updated")
	}
	for _, unchanged := range []string{"plain text body", `{"text":"no photos"}`, `{"photos":[{"file_id":"PF-ABCDEFGH"}]}`} {
		if got := stripInlinePublicRequestPhotoMetadata(unchanged); got != unchanged {
			t.Fatalf("body changed: %q -> %q", unchanged, got)
		}
	}
}
