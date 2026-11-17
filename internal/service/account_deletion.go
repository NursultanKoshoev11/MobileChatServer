package service

import "context"

func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	return s.repo.DeleteAccount(ctx, userID, "EVT-"+randomHex(12))
}
