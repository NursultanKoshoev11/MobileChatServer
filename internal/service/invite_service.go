package service

import (
	"context"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

func (s *Service) CreateInviteRequest(ctx context.Context, inviterID, groupID, targetUserID string) (domain.InviteRequest, error) {
	groupID = strings.TrimSpace(groupID)
	targetUserID = strings.TrimSpace(targetUserID)
	if groupID == "" || targetUserID == "" {
		return domain.InviteRequest{}, NewValidationError("group_id and target_user_id are required")
	}
	if err := s.repo.CanInviteToGroup(ctx, groupID, inviterID, targetUserID); err != nil {
		return domain.InviteRequest{}, err
	}
	return s.repo.CreateInviteRequest(ctx, domain.InviteRequest{
		ID:           "INV-" + strings.ToUpper(randomHex(12)),
		GroupID:      groupID,
		InviterID:    inviterID,
		TargetUserID: targetUserID,
	})
}

func (s *Service) ListPendingInvites(ctx context.Context, userID string) ([]domain.InviteRequest, error) {
	return s.repo.ListPendingInvites(ctx, userID)
}

func (s *Service) AcceptInviteRequest(ctx context.Context, userID, inviteID string) error {
	inviteID = strings.TrimSpace(inviteID)
	if inviteID == "" {
		return NewValidationError("invite_id is required")
	}
	return s.repo.AcceptInviteRequest(ctx, inviteID, userID)
}

func (s *Service) DeclineInviteRequest(ctx context.Context, userID, inviteID string) error {
	inviteID = strings.TrimSpace(inviteID)
	if inviteID == "" {
		return NewValidationError("invite_id is required")
	}
	return s.repo.DeclineInviteRequest(ctx, inviteID, userID)
}

var _ = storage.ErrNotFound
