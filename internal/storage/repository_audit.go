package storage

import (
	"context"
	"fmt"
)

func (r *Repository) CreateAuditEvent(ctx context.Context, id, actorUserID, eventType, resourceType, resourceID, metadataJSON string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO audit_events (id, actor_user_id, event_type, resource_type, resource_id, metadata)
		VALUES ($1, NULLIF($2, ''), $3, $4, NULLIF($5, ''), $6::jsonb)`, id, actorUserID, eventType, resourceType, resourceID, metadataJSON)
	if err != nil {
		return fmt.Errorf("create audit event: %w", err)
	}
	return nil
}
