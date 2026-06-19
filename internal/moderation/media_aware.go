package moderation

import (
	"context"
	"strings"
)

type MediaAwareModerator struct {
	next Moderator
}

func NewMediaAwareModerator(next Moderator) MediaAwareModerator {
	return MediaAwareModerator{next: next}
}

func (m MediaAwareModerator) Moderate(ctx context.Context, input Input) (Decision, error) {
	decision, err := m.next.Moderate(ctx, input)
	if err != nil {
		return decision, err
	}
	text := strings.ToLower(input.Title + " " + input.Body)
	if !containsMediaMarker(text) {
		return decision, nil
	}
	decision.Reasons = appendUniqueReason(decision.Reasons, "media_attachment_review")
	if decision.Action == ActionAllow {
		decision.Action = ActionReview
	}
	if strings.TrimSpace(decision.Provider) == "" {
		decision.Provider = "rules"
	}
	return decision, nil
}

func containsMediaMarker(text string) bool {
	markers := []string{
		"[photo",
		"[video",
		"attached photos",
		"attached videos",
		"photo file:",
		"video file:",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func appendUniqueReason(reasons []string, reason string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}
