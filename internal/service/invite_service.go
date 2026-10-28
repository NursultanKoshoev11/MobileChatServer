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
	inviteInput := domain.InviteRequest{
		ID:           "INV-" + strings.ToUpper(randomHex(12)),
		GroupID:      groupID,
		InviterID:    inviterID,
		TargetUserID: targetUserID,
	}
	var invite domain.InviteRequest
	err := s.recordAuditedMutation(ctx, inviterID, "invite_created", "group", groupID, func(repo *storage.Repository) error {
		if err := repo.CanInviteToGroup(ctx, groupID, inviterID, targetUserID); err != nil {
			return err
		}
		var createErr error
		invite, createErr = repo.CreateInviteRequest(ctx, inviteInput)
		return createErr
	})
	if err != nil {
		return domain.InviteRequest{}, err
	}
	return invite, nil
}

func (s *Service) ListPendingInvites(ctx context.Context, userID string) ([]domain.InviteRequest, error) {
	return s.repo.ListPendingInvites(ctx, userID)
}

func (s *Service) AcceptInviteRequest(ctx context.Context, userID, inviteID string) error {
	inviteID = strings.TrimSpace(inviteID)
	if inviteID == "" {
		return NewValidationError("invite_id is required")
	}
	if err := s.recordAuditedMutation(ctx, userID, "invite_accepted", "invite", inviteID, func(repo *storage.Repository) error {
		return repo.AcceptInviteRequestInTransaction(ctx, inviteID, userID)
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) DeclineInviteRequest(ctx context.Context, userID, inviteID string) error {
	inviteID = strings.TrimSpace(inviteID)
	if inviteID == "" {
		return NewValidationError("invite_id is required")
	}
	if err := s.recordAuditedMutation(ctx, userID, "invite_declined", "invite", inviteID, func(repo *storage.Repository) error {
		return repo.DeclineInviteRequest(ctx, inviteID, userID)
	}); err != nil {
		return err
	}
	return nil
}
