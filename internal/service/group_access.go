package service

import "context"

func (s *Service) EnsureGroupMember(ctx context.Context, userID, groupID string) error {
	_, err := s.repo.GetMemberRole(ctx, groupID, userID)
	return err
}
