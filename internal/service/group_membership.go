package service

import (
	"context"
	"strings"
)

func (s *Service) LeaveGroup(ctx context.Context, userID, groupID string) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return NewValidationError("group_id is required")
	}
	if err := s.repo.LeaveGroup(ctx, groupID, userID); err != nil {
		return err
	}
	s.RecordEvent(ctx, userID, "group_left", "group", groupID)
	return nil
}
