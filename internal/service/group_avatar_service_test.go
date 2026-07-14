package service

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func TestUpdateGroupAvatarRejectsUnsupportedMediaType(t *testing.T) {
	svc := New(nil, Config{})
	_, err := svc.UpdateGroupAvatar(context.Background(), "U-1", "G-1", "data:image/gif;base64,R0lGODlh")
	if err == nil || !strings.Contains(err.Error(), "JPEG, PNG, or WebP") {
		t.Fatalf("expected unsupported image validation error, got %v", err)
	}
}

func TestUpdateGroupAvatarRejectsInvalidBase64(t *testing.T) {
	svc := New(nil, Config{})
	_, err := svc.UpdateGroupAvatar(context.Background(), "U-1", "G-1", "data:image/jpeg;base64,not-base64")
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid data validation error, got %v", err)
	}
}

func TestUpdateGroupAvatarRejectsOversizedImage(t *testing.T) {
	svc := New(nil, Config{})
	payload := base64.StdEncoding.EncodeToString(make([]byte, maxGroupAvatarBytes+1))
	_, err := svc.UpdateGroupAvatar(context.Background(), "U-1", "G-1", "data:image/jpeg;base64,"+payload)
	if err == nil || !strings.Contains(err.Error(), "512 KB") {
		t.Fatalf("expected image size validation error, got %v", err)
	}
}
