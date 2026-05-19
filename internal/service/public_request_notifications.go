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
		log.Printf("push public_request.created skipped: list tokens failed group_id=%s request_id=%s error=%v", groupID, requestID, err)
		return
	}
	values := collectPushTokenValues(tokens)
	if len(values) == 0 {
		return
	}

	message := PushMessage{
		Title: "Новая публикация",
		Body:  pushBody(title, body),
		Data: map[string]string{
			"type":       "public_request.created",
			"group_id":   groupID,
			"request_id": requestID,
		},
	}
	if err := s.notifier.SendToTokens(ctx, values, message); err != nil {
		log.Printf("push public_request.created failed group_id=%s request_id=%s tokens=%d error=%v", groupID, requestID, len(values), err)
	}
}

func (s *Service) notifyGroupAboutNewPublicRequestComment(ctx context.Context, authorID string, requestID string, commentBody string) {
	ctx, cancel := context.WithTimeout(ctx, pushNotificationTimeout)
	defer cancel()

	request, err := s.repo.GetPublicRequestPushContext(ctx, requestID)
	if err != nil {
		log.Printf("push public_request.comment_created skipped: load request failed request_id=%s error=%v", requestID, err)
		return
	}
	tokens, err := s.repo.ListGroupPushTokensExceptUser(ctx, request.GroupID, authorID)
	if err != nil {
		log.Printf("push public_request.comment_created skipped: list tokens failed group_id=%s request_id=%s error=%v", request.GroupID, requestID, err)
		return
	}
	values := collectPushTokenValues(tokens)
	if len(values) == 0 {
		return
	}

	message := PushMessage{
		Title: "Новый комментарий",
		Body:  pushBody(request.Title, commentBody),
		Data: map[string]string{
			"type":       "public_request.comment_created",
			"group_id":   request.GroupID,
			"request_id": requestID,
		},
	}
	if err := s.notifier.SendToTokens(ctx, values, message); err != nil {
		log.Printf("push public_request.comment_created failed group_id=%s request_id=%s tokens=%d error=%v", request.GroupID, requestID, len(values), err)
	}
}

func collectPushTokenValues(tokens []struct {
	UserID   string
	Token    string
	Platform string
}) []string {
	values := make([]string, 0, len(tokens))
	seen := map[string]bool{}
	for _, item := range tokens {
		token := strings.TrimSpace(item.Token)
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		values = append(values, token)
	}
	return values
}

func pushBody(title, body string) string {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" && body == "" {
		return "Откройте Коом, чтобы посмотреть обновление."
	}
	if title == "" {
		return truncateRunes(body, 120)
	}
	if body == "" {
		return truncateRunes(title, 120)
	}
	if title == body {
		return truncateRunes(title, 120)
	}
	return truncateRunes(title+": "+body, 120)
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "..."
}
