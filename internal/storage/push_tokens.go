package storage

import (
	"context"
	"fmt"
	"strings"
)

type PushToken struct {
	UserID   string
	Token    string
	Platform string
}

func (r *Repository) UpsertPushToken(ctx context.Context, userID, token, platform string) error {
	userID = strings.TrimSpace(userID)
	token = strings.TrimSpace(token)
	platform = strings.TrimSpace(platform)
	if platform == "" {
		platform = "unknown"
	}
	if userID == "" || token == "" {
		return ErrNotFound
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO push_tokens (user_id, token, platform, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (user_id, token) DO UPDATE SET platform = EXCLUDED.platform, updated_at = now()`, userID, token, platform)
	if err != nil {
		return fmt.Errorf("upsert push token: %w", err)
	}
	return nil
}

func (r *Repository) DeletePushToken(ctx context.Context, userID, token string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM push_tokens WHERE user_id = $1 AND token = $2`, userID, token)
	if err != nil {
		return fmt.Errorf("delete push token: %w", err)
	}
	return nil
}

func (r *Repository) ListGroupPushTokensExceptUser(ctx context.Context, groupID, exceptUserID string) ([]PushToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pt.user_id, pt.token, pt.platform
		FROM push_tokens pt
		JOIN group_members gm ON gm.user_id = pt.user_id
		WHERE gm.group_id = $1 AND pt.user_id <> $2`, groupID, exceptUserID)
	if err != nil {
		return nil, fmt.Errorf("list group push tokens: %w", err)
	}
	defer rows.Close()

	tokens := make([]PushToken, 0)
	for rows.Next() {
		var token PushToken
		if err := rows.Scan(&token.UserID, &token.Token, &token.Platform); err != nil {
			return nil, fmt.Errorf("scan push token: %w", err)
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}
