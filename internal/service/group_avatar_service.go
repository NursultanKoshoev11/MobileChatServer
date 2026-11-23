package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/media"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

const maxGroupAvatarBytes = 512 * 1024

func (s *Service) UpdateGroupAvatar(ctx context.Context, actorID, groupID, avatarData string) (domain.Group, error) {
	actorID = strings.TrimSpace(actorID)
	groupID = strings.TrimSpace(groupID)
	avatarData = strings.TrimSpace(avatarData)
	if actorID == "" {
		return domain.Group{}, ErrUnauthorized
	}
	if groupID == "" {
		return domain.Group{}, NewValidationError("group_id is required")
	}
	if avatarData != "" {
		normalized, err := sanitizeImageDataURL(avatarData, maxGroupAvatarBytes, "group avatar")
		if err != nil {
			return domain.Group{}, err
		}
		avatarData = normalized
	}
	var group domain.Group
	err := s.recordAuditedMutation(ctx, actorID, "group_avatar_updated", "group", groupID, func(repo *storage.Repository) error {
		var updateErr error
		group, updateErr = repo.UpdateGroupAvatar(ctx, actorID, groupID, avatarData)
		return updateErr
	})
	if err != nil {
		return domain.Group{}, err
	}
	return group, nil
}

// sanitizeImageDataURL validates a base64 image data URL (JPEG, PNG or WebP,
// checked by magic number rather than the declared type), strips EXIF/GPS and
// text metadata, and returns the normalized data URL.
func sanitizeImageDataURL(value string, maxBytes int, label string) (string, error) {
	allowedPrefixes := []string{
		"data:image/jpeg;base64,",
		"data:image/png;base64,",
		"data:image/webp;base64,",
	}
	payload := ""
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(value, prefix) {
			payload = strings.TrimPrefix(value, prefix)
			break
		}
	}
	if payload == "" {
		return "", NewValidationError(label + " must be a JPEG, PNG, or WebP image")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(decoded) == 0 {
		return "", NewValidationError(label + " data is invalid")
	}
	if len(decoded) > maxBytes {
		return "", NewValidationError(fmt.Sprintf("%s image must be at most %d KB", label, maxBytes/1024))
	}
	contentType := media.ImageType(decoded)
	if contentType == "" {
		return "", NewValidationError(label + " must be a JPEG, PNG, or WebP image")
	}
	cleaned := media.StripImageMetadata(decoded)
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(cleaned), nil
}
