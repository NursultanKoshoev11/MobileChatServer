package service

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
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
		payload, err := validateImageDataURL(avatarData, maxGroupAvatarBytes)
		if err != nil {
			return domain.Group{}, err
		}
		if len(payload) == 0 {
			return domain.Group{}, NewValidationError("group avatar data is invalid")
		}
	}
	return s.repo.UpdateGroupAvatar(ctx, actorID, groupID, avatarData)
}

func validateImageDataURL(value string, maxBytes int) ([]byte, error) {
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
		return nil, NewValidationError("group avatar must be a JPEG, PNG, or WebP image")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(decoded) == 0 {
		return nil, NewValidationError("group avatar data is invalid")
	}
	if len(decoded) > maxBytes {
		return nil, NewValidationError("group avatar image must be at most 512 KB")
	}
	return decoded, nil
}
