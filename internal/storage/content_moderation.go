package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateContentModerationItem(ctx context.Context, item domain.ContentModerationItem) (domain.ContentModerationItem, error) {
	if item.Status == "" {
		item.Status = domain.ContentModerationStatusPending
	}
	if item.ProviderScoresJSON == "" {
		item.ProviderScoresJSON = "{}"
	}
	reasonsJSON, err := json.Marshal(item.Reasons)
	if err != nil {
		return domain.ContentModerationItem{}, fmt.Errorf("marshal moderation reasons: %w", err)
	}
	query := `
		INSERT INTO content_moderation_items (
			id, group_id, content_type, author_id, target_id, title, body, request_type,
			interaction_mode, status, decision, reasons, provider, provider_model,
			provider_response_id, provider_scores, created_at
		) VALUES (
			$1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, NULLIF($8, ''),
			NULLIF($9, ''), $10, $11, $12::jsonb, $13, NULLIF($14, ''),
			NULLIF($15, ''), $16::jsonb, now()
		)
		RETURNING created_at`
	if err := r.db.QueryRow(ctx, query,
		item.ID,
		item.GroupID,
		item.ContentType,
		item.AuthorID,
		item.TargetID,
		item.Title,
		item.Body,
		item.RequestType,
		item.InteractionMode,
		item.Status,
		item.Decision,
		string(reasonsJSON),
		item.Provider,
		item.ProviderModel,
		item.ProviderResponseID,
		item.ProviderScoresJSON,
	).Scan(&item.CreatedAt); err != nil {
		return domain.ContentModerationItem{}, fmt.Errorf("create content moderation item: %w", err)
	}
	user, err := r.GetUserByID(ctx, item.AuthorID)
	if err == nil {
		item.AuthorName = user.DisplayName
	}
	return item, nil
}

func (r *Repository) ListContentModerationItems(ctx context.Context, groupID string, status domain.ContentModerationStatus, limit int) ([]domain.ContentModerationItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
		SELECT c.id, c.group_id, c.content_type, c.author_id, COALESCE(u.display_name, ''),
		       COALESCE(c.target_id, ''), COALESCE(c.title, ''), c.body,
		       COALESCE(c.request_type, ''), COALESCE(c.interaction_mode, ''), c.status,
		       c.decision, c.reasons, c.provider, COALESCE(c.provider_model, ''),
		       COALESCE(c.provider_response_id, ''), c.provider_scores::text,
		       COALESCE(c.published_resource_id, ''), c.created_at, c.reviewed_at,
		       COALESCE(c.reviewed_by, '')
		FROM content_moderation_items c
		LEFT JOIN users u ON u.id = c.author_id
		WHERE c.group_id = $1 AND ($2::text = '' OR c.status = $2)
		ORDER BY c.created_at DESC
		LIMIT $3`
	rows, err := r.db.Query(ctx, query, groupID, string(status), limit)
	if err != nil {
		return nil, fmt.Errorf("list content moderation items: %w", err)
	}
	defer rows.Close()
	items := make([]domain.ContentModerationItem, 0)
	for rows.Next() {
		item, err := scanContentModerationItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetContentModerationItem(ctx context.Context, itemID string) (domain.ContentModerationItem, error) {
	query := `
		SELECT c.id, c.group_id, c.content_type, c.author_id, COALESCE(u.display_name, ''),
		       COALESCE(c.target_id, ''), COALESCE(c.title, ''), c.body,
		       COALESCE(c.request_type, ''), COALESCE(c.interaction_mode, ''), c.status,
		       c.decision, c.reasons, c.provider, COALESCE(c.provider_model, ''),
		       COALESCE(c.provider_response_id, ''), c.provider_scores::text,
		       COALESCE(c.published_resource_id, ''), c.created_at, c.reviewed_at,
		       COALESCE(c.reviewed_by, '')
		FROM content_moderation_items c
		LEFT JOIN users u ON u.id = c.author_id
		WHERE c.id = $1`
	row := r.db.QueryRow(ctx, query, itemID)
	item, err := scanContentModerationItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ContentModerationItem{}, ErrNotFound
	}
	if err != nil {
		return domain.ContentModerationItem{}, err
	}
	return item, nil
}

func (r *Repository) ReviewContentModerationItem(ctx context.Context, itemID string, status domain.ContentModerationStatus, reviewerID string, publishedResourceID string) (domain.ContentModerationItem, error) {
	query := `
		UPDATE content_moderation_items
		SET status = $1,
		    reviewed_by = $2,
		    reviewed_at = now(),
		    published_resource_id = NULLIF($3, '')
		WHERE id = $4 AND status = 'pending'
		RETURNING id, group_id, content_type, author_id, COALESCE(target_id, ''),
		          COALESCE(title, ''), body, COALESCE(request_type, ''), COALESCE(interaction_mode, ''),
		          status, decision, reasons, provider, COALESCE(provider_model, ''),
		          COALESCE(provider_response_id, ''), provider_scores::text,
		          COALESCE(published_resource_id, ''), created_at, reviewed_at, COALESCE(reviewed_by, '')`
	var item domain.ContentModerationItem
	var reasonsRaw []byte
	if err := r.db.QueryRow(ctx, query, status, reviewerID, publishedResourceID, itemID).Scan(
		&item.ID,
		&item.GroupID,
		&item.ContentType,
		&item.AuthorID,
		&item.TargetID,
		&item.Title,
		&item.Body,
		&item.RequestType,
		&item.InteractionMode,
		&item.Status,
		&item.Decision,
		&reasonsRaw,
		&item.Provider,
		&item.ProviderModel,
		&item.ProviderResponseID,
		&item.ProviderScoresJSON,
		&item.PublishedResourceID,
		&item.CreatedAt,
		&item.ReviewedAt,
		&item.ReviewedBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ContentModerationItem{}, ErrNotFound
		}
		return domain.ContentModerationItem{}, fmt.Errorf("review content moderation item: %w", err)
	}
	_ = json.Unmarshal(reasonsRaw, &item.Reasons)
	user, err := r.GetUserByID(ctx, item.AuthorID)
	if err == nil {
		item.AuthorName = user.DisplayName
	}
	return item, nil
}

type contentModerationScanner interface {
	Scan(dest ...any) error
}

func scanContentModerationItem(row contentModerationScanner) (domain.ContentModerationItem, error) {
	var item domain.ContentModerationItem
	var reasonsRaw []byte
	if err := row.Scan(
		&item.ID,
		&item.GroupID,
		&item.ContentType,
		&item.AuthorID,
		&item.AuthorName,
		&item.TargetID,
		&item.Title,
		&item.Body,
		&item.RequestType,
		&item.InteractionMode,
		&item.Status,
		&item.Decision,
		&reasonsRaw,
		&item.Provider,
		&item.ProviderModel,
		&item.ProviderResponseID,
		&item.ProviderScoresJSON,
		&item.PublishedResourceID,
		&item.CreatedAt,
		&item.ReviewedAt,
		&item.ReviewedBy,
	); err != nil {
		return domain.ContentModerationItem{}, err
	}
	_ = json.Unmarshal(reasonsRaw, &item.Reasons)
	return item, nil
}
