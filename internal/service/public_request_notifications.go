package service

import (
	"context"
	"log"
)

func (s *Service) notifyGroupAboutNewPublicRequest(ctx context.Context, authorID string, groupID string, requestID string, title string, body string) {
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
		values = append(values, item.Token)
	}
	messageBody := body
	if len(messageBody) > 120 {
		messageBody = messageBody[:120] + "..."
	}
	if messageBody == "" {
		messageBody = "A new post was published."
	}
	if err := s.notifier.SendToTokens(ctx, values, PushMessage{
		Title: title,
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
