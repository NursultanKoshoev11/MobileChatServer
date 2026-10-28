package service

import (
	"context"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

func (s *Service) recordAuditedMutation(ctx context.Context, actorUserID, eventType, resourceType, resourceID string, mutate func(*storage.Repository) error) error {
	return s.repo.WithAuditEvent(ctx, storage.AuditEvent{
		ID:           "EVT-" + strings.ToUpper(randomHex(12)),
		ActorUserID:  actorUserID,
		EventType:    eventType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		MetadataJSON: "{}",
	}, mutate)
}
