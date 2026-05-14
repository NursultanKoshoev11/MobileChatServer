package service

import (
	"context"
	"strings"
)

func (s *Service) RecordEvent(ctx context.Context, actorUserID, eventType, resourceType, resourceID string) {
	if eventType == "" || resourceType == "" {
		return
	}
	_ = s.repo.CreateAuditEvent(
		ctx,
		"EVT-"+strings.ToUpper(randomHex(12)),
		actorUserID,
		eventType,
		resourceType,
		resourceID,
		"{}",
	)
}
