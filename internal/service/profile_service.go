package service

import (
	"context"
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

	normalized, err := sanitizeImageDataURL(avatarData, maxAvatarBytes, "avatar")
	if err != nil {
		return domain.User{}, err
	}
	avatarData = normalized
	return s.repo.UpdateUserAvatar(ctx, userID, avatarData)
}
