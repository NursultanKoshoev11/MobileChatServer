package service

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func TestUpdateUserAvatarRejectsUnsupportedMediaType(t *testing.T) {
	svc := New(nil, Config{})
	_, err := svc.UpdateUserAvatar(context.Background(), "U-1", "data:image/gif;base64,R0lGODlh")
	if err == nil || !strings.Contains(err.Error(), "JPEG, PNG, or WebP") {
		t.Fatalf("expected unsupported image validation error, got %v", err)
	}
}

func TestUpdateUserAvatarRejectsInvalidBase64(t *testing.T) {
	svc := New(nil, Config{})
	_, err := svc.UpdateUserAvatar(context.Background(), "U-1", "data:image/jpeg;base64,not-base64")
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid data validation error, got %v", err)
	}
}

func TestUpdateUserAvatarRejectsOversizedImage(t *testing.T) {
	svc := New(nil, Config{})
	payload := base64.StdEncoding.EncodeToString(make([]byte, maxAvatarBytes+1))
	_, err := svc.UpdateUserAvatar(context.Background(), "U-1", "data:image/jpeg;base64,"+payload)
	if err == nil || !strings.Contains(err.Error(), "512 KB") {
		t.Fatalf("expected image size validation error, got %v", err)
	}
}
