package service

import (
	"encoding/base64"
	"testing"
)

func TestValidatePublicRequestVideoPayloadAcceptsUploadedFileReference(t *testing.T) {
	fileID := "PF-0123456789ABCDEF"
	video := publicRequestPayloadVideo{
		SizeBytes: 12 * 1024 * 1024,
		FileID:    fileID,
		URL:       "/api/public-files/G-1/" + fileID,
		MimeType:  "video/mp4",
	}
	if err := validatePublicRequestVideoPayload(video, "G-1"); err != nil {
		t.Fatalf("uploaded video reference should be accepted: %v", err)
	}
}

func TestValidatePublicRequestVideoPayloadRejectsExternalAndMismatchedURLs(t *testing.T) {
	fileID := "PF-0123456789ABCDEF"
	tests := []struct {
		name string
		url  string
	}{
		{name: "external origin", url: "https://koom.com.kg/api/public-files/G-1/" + fileID},
		{name: "other file id", url: "/api/public-files/G-1/PF-FFFFFFFFFFFFFFFF"},
		{name: "other group", url: "/api/public-files/G-2/" + fileID},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			video := publicRequestPayloadVideo{FileID: fileID, URL: test.url}
			if err := validatePublicRequestVideoPayload(video, "G-1"); err == nil {
				t.Fatal("invalid video file reference was accepted")
			}
		})
	}
}

func TestValidatePublicRequestVideoPayloadKeepsLegacyBase64Support(t *testing.T) {
	video := publicRequestPayloadVideo{
		SizeBytes: 12,
		Base64:    base64.StdEncoding.EncodeToString([]byte("legacy video")),
	}
	if err := validatePublicRequestVideoPayload(video, "G-1"); err != nil {
		t.Fatalf("legacy base64 video should remain readable: %v", err)
	}
}
