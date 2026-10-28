package service

import (
	"context"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
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
	if err := s.recordAuditedMutation(ctx, actorID, "member_promoted", "group", groupID, func(repo *storage.Repository) error {
		return repo.SetMemberRole(ctx, groupID, actorID, targetUserID, domain.RoleAdmin)
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) DemoteMember(ctx context.Context, actorID, groupID, targetUserID string) error {
	groupID = strings.TrimSpace(groupID)
	targetUserID = strings.TrimSpace(targetUserID)
	if groupID == "" || targetUserID == "" {
		return NewValidationError("group_id and target_user_id are required")
	}
	if err := s.recordAuditedMutation(ctx, actorID, "member_demoted", "group", groupID, func(repo *storage.Repository) error {
		return repo.SetMemberRole(ctx, groupID, actorID, targetUserID, domain.RoleMember)
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, actorID, groupID, targetUserID string) error {
	groupID = strings.TrimSpace(groupID)
	targetUserID = strings.TrimSpace(targetUserID)
	if groupID == "" || targetUserID == "" {
		return NewValidationError("group_id and target_user_id are required")
	}
	eventType := "member_removed"
	if actorID == targetUserID {
		eventType = "member_left"
	}
	return s.recordAuditedMutation(ctx, actorID, eventType, "group", groupID, func(repo *storage.Repository) error {
		return repo.RemoveGroupMember(ctx, groupID, actorID, targetUserID)
	})
}

func (s *Service) ListGroupMemberIDsExcept(ctx context.Context, groupID, excludeUserID string) ([]string, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	return s.repo.ListGroupMemberIDsExcept(ctx, groupID, excludeUserID)
}
