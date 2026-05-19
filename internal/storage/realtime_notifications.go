package storage

import (
	"context"
	"fmt"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (r *Repository) ListPlatformAdminUserIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id
		FROM users
		WHERE role IN ($1, $2)`, domain.UserRolePlatformAdmin, domain.UserRoleSuperAdmin)
	if err != nil {
		return nil, fmt.Errorf("list platform admin user ids: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan platform admin user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) ListPushTokensForUsers(ctx context.Context, userIDs []string) ([]PushToken, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT user_id, token, platform
		FROM push_tokens
		WHERE user_id = ANY($1)`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("list push tokens for users: %w", err)
	}
	defer rows.Close()

	tokens := make([]PushToken, 0)
	for rows.Next() {
		var token PushToken
		if err := rows.Scan(&token.UserID, &token.Token, &token.Platform); err != nil {
			return nil, fmt.Errorf("scan push token for user: %w", err)
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}
