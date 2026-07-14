package service

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

const maxAvatarBytes = 512 * 1024

func (s *Service) GetUserProfile(ctx context.Context, userID string) (domain.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.User{}, ErrUnauthorized
	}
	return s.repo.GetUserProfileByID(ctx, userID)
}

func (s *Service) UpdateUserAvatar(ctx context.Context, userID, avatarData string) (domain.User, error) {
	userID = strings.TrimSpace(userID)
	avatarData = strings.TrimSpace(avatarData)
	if userID == "" {
		return domain.User{}, ErrUnauthorized
	}
	if avatarData == "" {
		return s.repo.UpdateUserAvatar(ctx, userID, "")
	}

	allowedPrefixes := []string{
		"data:image/jpeg;base64,",
		"data:image/png;base64,",
		"data:image/webp;base64,",
	}
	payload := ""
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(avatarData, prefix) {
			payload = strings.TrimPrefix(avatarData, prefix)
			break
		}
	}
	if payload == "" {
		return domain.User{}, NewValidationError("avatar must be a JPEG, PNG, or WebP image")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(decoded) == 0 {
		return domain.User{}, NewValidationError("avatar data is invalid")
	}
	if len(decoded) > maxAvatarBytes {
		return domain.User{}, NewValidationError("avatar image must be at most 512 KB")
	}
	return s.repo.UpdateUserAvatar(ctx, userID, avatarData)
}
