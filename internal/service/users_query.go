package service

import (
	"context"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (s *Service) ListGroupMembersPage(ctx context.Context, requesterID, groupID string, limit, offset int) ([]domain.GroupMember, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	return s.repo.ListGroupMembersPage(ctx, groupID, requesterID, limit, offset)
}

func (s *Service) SearchUsers(ctx context.Context, query string, limit int) ([]domain.User, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 {
		return nil, NewValidationError("query must be at least 2 characters")
	}
	return s.repo.SearchUsers(ctx, query, limit)
}
