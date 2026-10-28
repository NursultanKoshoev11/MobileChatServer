package storage

import (
	"context"
	"fmt"
	"strings"
)

type AuditEvent struct {
	ID           string
	ActorUserID  string
	EventType    string
	ResourceType string
	ResourceID   string
	MetadataJSON string
}

// WithAuditEvent commits the database mutation and its audit event together.
// A failure in either operation rolls back both, so callers must not perform
// non-transactional side effects from mutate.
func (r *Repository) WithAuditEvent(ctx context.Context, event AuditEvent, mutate func(*Repository) error) error {
	if mutate == nil {
		return fmt.Errorf("audited mutation callback is required")
	}
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.EventType) == "" || strings.TrimSpace(event.ResourceType) == "" {
		return fmt.Errorf("audit event id, event type, and resource type are required")
	}
	if strings.TrimSpace(event.ActorUserID) == "" {
		return fmt.Errorf("audit event actor is required")
	}
	if event.MetadataJSON == "" {
		event.MetadataJSON = "{}"
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin audited mutation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	txRepo := &Repository{db: tx, pool: r.pool}
	if err := mutate(txRepo); err != nil {
		return err
	}
	if err := txRepo.CreateAuditEvent(ctx, event.ID, event.ActorUserID, event.EventType, event.ResourceType, event.ResourceID, event.MetadataJSON); err != nil {
		return fmt.Errorf("persist audit event for mutation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit audited mutation: %w", err)
	}
	return nil
}

func (r *Repository) CreateAuditEvent(ctx context.Context, id, actorUserID, eventType, resourceType, resourceID, metadataJSON string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO audit_events (id, actor_user_id, event_type, resource_type, resource_id, metadata)
		VALUES ($1, NULLIF($2, ''), $3, $4, NULLIF($5, ''), $6::jsonb)`, id, actorUserID, eventType, resourceType, resourceID, metadataJSON)
	if err != nil {
		return fmt.Errorf("create audit event: %w", err)
	}
	return nil
}
