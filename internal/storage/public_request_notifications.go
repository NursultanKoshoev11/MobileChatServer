package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type PublicRequestPushContext struct {
	GroupID string
	Title   string
}

func (r *Repository) GetPublicRequestPushContext(ctx context.Context, requestID string) (PublicRequestPushContext, error) {
	var result PublicRequestPushContext
	err := r.db.QueryRow(ctx, `
		SELECT group_id, title
		FROM public_requests
		WHERE id = $1 AND hidden_at IS NULL`, requestID).Scan(&result.GroupID, &result.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicRequestPushContext{}, ErrNotFound
	}
	if err != nil {
		return PublicRequestPushContext{}, fmt.Errorf("get public request push context: %w", err)
	}
	return result, nil
}
