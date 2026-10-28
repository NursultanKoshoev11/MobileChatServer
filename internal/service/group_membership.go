package service

import (
	"context"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

func (s *Service) LeaveGroup(ctx context.Context, userID, groupID string) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return NewValidationError("group_id is required")
	}
	if err := s.recordAuditedMutation(ctx, userID, "group_left", "group", groupID, func(repo *storage.Repository) error {
		return repo.LeaveGroup(ctx, groupID, userID)
	}); err != nil {
		return err
	}
	return nil
}
