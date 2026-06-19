package service

import (
	"context"
	"log"
	"strconv"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (s *Service) ListPlatformAdminUserIDs(ctx context.Context) ([]string, error) {
	return s.repo.ListPlatformAdminUserIDs(ctx)
}

func (s *Service) NotifyUsers(ctx context.Context, userIDs []string, message PushMessage) {
	ctx, cancel := context.WithTimeout(ctx, pushNotificationTimeout)
	defer cancel()

	tokens, err := s.repo.ListPushTokensForUsers(ctx, userIDs)
	if err != nil {
		log.Printf("push users notification skipped: list tokens failed users=%d error=%v", len(userIDs), err)
		return
	}
	values := collectPushTokenValues(tokens)
	if len(values) == 0 {
		return
	}
	if err := s.notifier.SendToTokens(ctx, values, message); err != nil {
		log.Printf("push users notification failed users=%d tokens=%d error=%v", len(userIDs), len(values), err)
	}
}

func (s *Service) NotifyAdminsAboutGroupCreationRequest(ctx context.Context, request domain.GroupCreationRequest) {
	adminIDs, err := s.ListPlatformAdminUserIDs(ctx)
	if err != nil {
		log.Printf("push group_creation_request.created skipped: list admins failed request_id=%s error=%v", request.ID, err)
		return
	}
	s.NotifyUsers(ctx, adminIDs, PushMessage{
		Title: "Новая заявка на группу",
		Body:  pushBody(request.GroupTitle, request.OrganizationName),
		Data: map[string]string{
			"type":       "group_creation_request.created",
			"request_id": request.ID,
		},
	})
}

func (s *Service) NotifyUserAboutInvite(ctx context.Context, userID string, invite domain.InviteRequest) {
	s.NotifyUsers(ctx, []string{userID}, PushMessage{
		Title: "Новое приглашение в группу",
		Body:  pushBody(invite.GroupTitle, invite.InviterName),
		Data: map[string]string{
			"type":      "invite.created",
			"invite_id": invite.ID,
			"group_id":  invite.GroupID,
		},
	})
}

func (s *Service) NotifyUserAboutGroupCreationReview(ctx context.Context, userID string, request domain.GroupCreationRequest) {
	s.NotifyUsers(ctx, []string{userID}, PushMessage{
		Title: "Статус заявки обновлён",
		Body:  pushBody(request.GroupTitle, string(request.Status)),
		Data: map[string]string{
			"type":       "group_creation_request.reviewed",
			"request_id": request.ID,
			"status":     string(request.Status),
		},
	})
}

func (s *Service) NotifyAdminsAboutContentModerationPending(ctx context.Context, item domain.ContentModerationItem) {
	adminIDs, err := s.repo.ListContentModerationAdminUserIDs(ctx, item.GroupID)
	if err != nil {
		log.Printf("push content_moderation.pending_review skipped: list admins failed item_id=%s error=%v", item.ID, err)
		return
	}
	count, err := s.repo.CountContentModerationItems(ctx, item.GroupID, domain.ContentModerationStatusPending)
	if err != nil {
		log.Printf("push content_moderation.pending_review count skipped item_id=%s error=%v", item.ID, err)
	}
	body := pushBody(item.Title, item.Body)
	if body == "" {
		body = string(item.ContentType)
	}
	s.NotifyUsers(ctx, adminIDs, PushMessage{
		Title: "Контент ожидает проверки",
		Body:  body,
		Data: map[string]string{
			"type":        "content_moderation.pending_review",
			"group_id":    item.GroupID,
			"item_id":     item.ID,
			"queue_count": strconv.Itoa(count),
		},
	})
}
