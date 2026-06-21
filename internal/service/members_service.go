package service

import (
	"context"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (s *Service) ListGroupMembers(ctx context.Context, requesterID, groupID string) ([]domain.GroupMember, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	return s.repo.ListGroupMembers(ctx, groupID, requesterID)
}

func (s *Service) PromoteMember(ctx context.Context, actorID, groupID, targetUserID string) error {
	groupID = strings.TrimSpace(groupID)
	targetUserID = strings.TrimSpace(targetUserID)
	if groupID == "" || targetUserID == "" {
		return NewValidationError("group_id and target_user_id are required")
	}
	if err := s.repo.SetMemberRole(ctx, groupID, actorID, targetUserID, domain.RoleAdmin); err != nil {
		return err
	}
	s.RecordEvent(ctx, actorID, "member_promoted", "group", groupID)
	return nil
}

func (s *Service) DemoteMember(ctx context.Context, actorID, groupID, targetUserID string) error {
	groupID = strings.TrimSpace(groupID)
	targetUserID = strings.TrimSpace(targetUserID)
	if groupID == "" || targetUserID == "" {
		return NewValidationError("group_id and target_user_id are required")
	}
	if err := s.repo.SetMemberRole(ctx, groupID, actorID, targetUserID, domain.RoleMember); err != nil {
		return err
	}
	s.RecordEvent(ctx, actorID, "member_demoted", "group", groupID)
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, actorID, groupID, targetUserID string) error {
	groupID = strings.TrimSpace(groupID)
	targetUserID = strings.TrimSpace(targetUserID)
	if groupID == "" || targetUserID == "" {
		return NewValidationError("group_id and target_user_id are required")
	}
	if err := s.repo.RemoveGroupMember(ctx, groupID, actorID, targetUserID); err != nil {
		return err
	}
	eventType := "member_removed"
	if actorID == targetUserID {
		eventType = "member_left"
	}
	s.RecordEvent(ctx, actorID, eventType, "group", groupID)
	return nil
}

func (s *Service) ListGroupMemberIDsExcept(ctx context.Context, groupID, excludeUserID string) ([]string, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	return s.repo.ListGroupMemberIDsExcept(ctx, groupID, excludeUserID)
}
