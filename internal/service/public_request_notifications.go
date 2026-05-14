package service

import (
	"context"
	"log"
	"strings"
	"time"
)

const pushNotificationTimeout = 10 * time.Second

func (s *Service) notifyGroupAboutNewPublicRequest(ctx context.Context, authorID string, groupID string, requestID string, title string, body string) {
	ctx, cancel := context.WithTimeout(ctx, pushNotificationTimeout)
	defer cancel()

	tokens, err := s.repo.ListGroupPushTokensExceptUser(ctx, groupID, authorID)
	if err != nil {
		log.Printf("push notification skipped: list tokens failed: %v", err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	values := make([]string, 0, len(tokens))
	for _, item := range tokens {
		if strings.TrimSpace(item.Token) == "" {
			continue
		}
		values = append(values, item.Token)
	}
	if len(values) == 0 {
		return
	}
	messageTitle := strings.TrimSpace(title)
	if messageTitle == "" {
		messageTitle = "New post"
	}
	messageBody := truncateRunes(strings.TrimSpace(body), 120)
	if messageBody == "" {
		messageBody = "A new post was published."
	}
	if err := s.notifier.SendToTokens(ctx, values, PushMessage{
		Title: messageTitle,
		Body:  messageBody,
		Data: map[string]string{
			"type":       "new_post",
			"group_id":   groupID,
			"request_id": requestID,
		},
	}); err != nil {
		log.Printf("push notification skipped: send failed: %v", err)
	}
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}
