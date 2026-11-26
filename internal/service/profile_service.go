package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

const maxAvatarBytes = 512 * 1024

func (s *Service) UpdateUserDisplayName(ctx context.Context, userID, displayName string) (domain.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.User{}, ErrUnauthorized
	}
	displayName = strings.TrimSpace(displayName)
	nameLength := utf8.RuneCountInString(displayName)
	if nameLength < 2 || nameLength > maxDisplayNameLen {
		return domain.User{}, NewValidationError(fmt.Sprintf("display_name must be between 2 and %d characters", maxDisplayNameLen))
	}
	return s.repo.UpdateUserDisplayName(ctx, userID, displayName)
}

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
