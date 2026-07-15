package service

import (
	"context"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

func (s *Service) DeleteGroupAsPlatformAdmin(ctx context.Context, admin domain.User, groupID string) error {
	if !isPlatformAdmin(admin) {
		return storage.ErrForbidden
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return NewValidationError("group_id is required")
	}
	if err := s.repo.DeleteGroupAsPlatformAdmin(ctx, groupID); err != nil {
		return err
	}
	s.RecordEvent(ctx, admin.ID, "platform_group_deleted", "group", groupID)
	return nil
}
