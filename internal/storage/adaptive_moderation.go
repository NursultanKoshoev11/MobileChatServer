package storage

import (
	"context"
	"fmt"
	"strings"
)

type LearnedModerationRule struct {
	GroupID    string
	Pattern    string
	Action     string
	Weight     int
	SourceItem string
}

func (r *Repository) ensureLearnedModerationRules(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS learned_moderation_rules (
			id bigserial PRIMARY KEY,
			group_id text NOT NULL,
			pattern text NOT NULL,
			action text NOT NULL DEFAULT 'deny',
			weight integer NOT NULL DEFAULT 1,
			source_item_id text,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE(group_id, pattern)
		)`
	_, err := r.db.Exec(ctx, query)
	return err
}

func (r *Repository) UpsertLearnedModerationRule(ctx context.Context, rule LearnedModerationRule) error {
	if err := r.ensureLearnedModerationRules(ctx); err != nil {
		return err
	}
	groupID := strings.TrimSpace(rule.GroupID)
	pattern := strings.TrimSpace(strings.ToLower(rule.Pattern))
	if groupID == "" || pattern == "" {
		return nil
	}
	action := strings.TrimSpace(rule.Action)
	if action == "" {
		action = "deny"
	}
	weight := rule.Weight
	if weight <= 0 {
		weight = 1
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO learned_moderation_rules (group_id, pattern, action, weight, source_item_id)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		ON CONFLICT (group_id, pattern)
		DO UPDATE SET
			weight = LEAST(learned_moderation_rules.weight + EXCLUDED.weight, 10),
			action = EXCLUDED.action,
			updated_at = now()
	`, groupID, pattern, action, weight, strings.TrimSpace(rule.SourceItem))
	if err != nil {
		return fmt.Errorf("upsert learned moderation rule: %w", err)
	}
	return nil
}

func (r *Repository) MatchLearnedModerationRules(ctx context.Context, groupID, normalizedText string, limit int) ([]LearnedModerationRule, error) {
	if err := r.ensureLearnedModerationRules(ctx); err != nil {
		return nil, err
	}
	groupID = strings.TrimSpace(groupID)
	normalizedText = strings.TrimSpace(strings.ToLower(normalizedText))
	if groupID == "" || normalizedText == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	rows, err := r.db.Query(ctx, `
		SELECT group_id, pattern, action, weight, COALESCE(source_item_id, '')
		FROM learned_moderation_rules
		WHERE group_id = $1
		  AND length(pattern) >= 4
		  AND $2 LIKE '%' || pattern || '%'
		ORDER BY weight DESC, updated_at DESC
		LIMIT $3
	`, groupID, normalizedText, limit)
	if err != nil {
		return nil, fmt.Errorf("match learned moderation rules: %w", err)
	}
	defer rows.Close()
	matched := make([]LearnedModerationRule, 0)
	for rows.Next() {
		var rule LearnedModerationRule
		if err := rows.Scan(&rule.GroupID, &rule.Pattern, &rule.Action, &rule.Weight, &rule.SourceItem); err != nil {
			return nil, err
		}
		matched = append(matched, rule)
	}
	return matched, rows.Err()
}
