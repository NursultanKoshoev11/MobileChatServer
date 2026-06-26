package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

type UpdateMessageInput struct {
	Text string `json:"text"`
}

func (s *Service) UpdateMessage(ctx context.Context, actorID, groupID, messageID string, input UpdateMessageInput) (domain.Message, error) {
	groupID = strings.TrimSpace(groupID)
	messageID = strings.TrimSpace(messageID)
	text := strings.TrimSpace(input.Text)
	if groupID == "" || messageID == "" {
		return domain.Message{}, NewValidationError("group_id and message_id are required")
	}
	if text == "" {
		return domain.Message{}, NewValidationError("text is required")
	}
	if len(text) > maxMessageLen {
		return domain.Message{}, NewValidationError(fmt.Sprintf("text must be at most %d characters", maxMessageLen))
	}
	if err := s.moderateContent(ctx, domain.ContentModerationItem{GroupID: groupID, ContentType: domain.ContentTypeGroupMessage, AuthorID: actorID, Body: text}); err != nil {
		return domain.Message{}, err
	}
	message, err := s.repo.UpdateMessage(ctx, groupID, actorID, messageID, text)
	if err != nil {
		return domain.Message{}, err
	}
	go s.NotifyGroupAboutMessageUpdated(context.Background(), actorID, message)
	return message, nil
}

func (s *Service) DeleteMessage(ctx context.Context, actorID, groupID, messageID string) (domain.Message, error) {
	groupID = strings.TrimSpace(groupID)
	messageID = strings.TrimSpace(messageID)
	if groupID == "" || messageID == "" {
		return domain.Message{}, NewValidationError("group_id and message_id are required")
	}
	message, err := s.repo.DeleteMessage(ctx, groupID, actorID, messageID)
	if err != nil {
		return domain.Message{}, err
	}
	go s.NotifyGroupAboutMessageDeleted(context.Background(), actorID, message)
	return message, nil
}
