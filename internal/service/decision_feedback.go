package service

import (
	"context"
	"strings"
	"unicode"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

func (s *Service) storeAdaptiveModerationFeedback(ctx context.Context, item domain.ContentModerationItem, action, sourceItemID string) error {
	pattern := adaptiveModerationPattern(moderationBodyForInput(item))
	if pattern == "" {
		return nil
	}
	return s.repo.UpsertLearnedModerationRule(ctx, storage.LearnedModerationRule{
		GroupID:    item.GroupID,
		Pattern:    pattern,
		Action:     strings.TrimSpace(action),
		Weight:     1,
		SourceItem: strings.TrimSpace(sourceItemID),
	})
}

func adaptiveModerationPattern(value string) string {
	normalized := normalizeAdaptiveModerationText(value)
	if len([]rune(normalized)) < 4 {
		return ""
	}
	runes := []rune(normalized)
	if len(runes) > 120 {
		runes = runes[:120]
	}
	return strings.TrimSpace(string(runes))
}

func normalizeAdaptiveModerationText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "ё", "е")
	value = strings.ReplaceAll(value, "ү", "у")
	value = strings.ReplaceAll(value, "ң", "н")
	value = strings.ReplaceAll(value, "ө", "о")
	var builder strings.Builder
	previousSpace := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			if !previousSpace {
				builder.WriteByte(' ')
				previousSpace = true
			}
			continue
		}
		builder.WriteRune(r)
		previousSpace = false
	}
	return strings.TrimSpace(builder.String())
}
