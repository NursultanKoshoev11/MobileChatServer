package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/moderation"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

type ContentModerationPendingError struct {
	Item domain.ContentModerationItem
}

func (e ContentModerationPendingError) Error() string {
	return "content requires moderation review"
}

func (s *Service) SetContentModerator(moderator moderation.Moderator) {
	s.contentModerator = moderator
}

func (s *Service) moderateContent(ctx context.Context, item domain.ContentModerationItem) error {
	if s.contentModerator == nil {
		return nil
	}
	body := moderationBodyForInput(item)
	if matches, err := s.repo.MatchLearnedModerationRules(ctx, item.GroupID, normalizeAdaptiveModerationText(body), 5); err == nil {
		for _, match := range matches {
			if strings.EqualFold(match.Action, "deny") {
				decision := moderation.NewDecision(moderation.ActionBlock, "learned_rules", "learned_admin_reject")
				return s.handleModerationDecision(ctx, item, decision)
			}
		}
	}
	decision, err := s.contentModerator.Moderate(ctx, moderation.Input{
		ContentType: moderation.ContentType(item.ContentType),
		GroupID:     item.GroupID,
		AuthorID:    item.AuthorID,
		TargetID:    item.TargetID,
		Title:       item.Title,
		Body:        body,
	})
	if err != nil {
		return fmt.Errorf("moderate content: %w", err)
	}
	return s.handleModerationDecision(ctx, item, decision)
}

func (s *Service) handleModerationDecision(ctx context.Context, item domain.ContentModerationItem, decision moderation.Decision) error {
	if decision.Action == moderation.ActionAllow {
		return nil
	}
	queued := item
	queued.ID = "MOD-" + strings.ToUpper(randomHex(12))
	queued.Status = domain.ContentModerationStatusPending
	if decision.Action == moderation.ActionBlock {
		queued.Status = domain.ContentModerationStatusRejected
	}
	queued.Decision = string(decision.Action)
	queued.Reasons = decision.Reasons
	queued.Provider = firstNonEmpty(decision.Provider, "rules")
	queued.ProviderModel = decision.ProviderModel
	queued.ProviderResponseID = decision.ProviderResponseID
	queued.ProviderScoresJSON = firstNonEmpty(decision.ProviderScoresJSON, "{}")
	created, err := s.repo.CreateContentModerationItem(ctx, queued)
	if err != nil {
		return err
	}
	if decision.Action == moderation.ActionBlock {
		s.RecordEvent(ctx, item.AuthorID, "content_moderation_auto_rejected", string(item.ContentType), created.ID)
		return NewValidationError("content is not allowed")
	}
	s.RecordEvent(ctx, item.AuthorID, "content_moderation_queued", string(item.ContentType), created.ID)
	return ContentModerationPendingError{Item: created}
}

func (s *Service) ListContentModerationItems(ctx context.Context, reviewerID, groupID, status string, limit int) ([]domain.ContentModerationItem, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	if err := s.ensureGroupModerator(ctx, reviewerID, groupID); err != nil {
		return nil, err
	}
	moderationStatus := domain.ContentModerationStatus(strings.TrimSpace(status))
	if moderationStatus != "" && !validContentModerationStatus(moderationStatus) {
		return nil, NewValidationError("status is invalid")
	}
	return s.repo.ListContentModerationItems(ctx, groupID, moderationStatus, limit)
}

func (s *Service) ApproveContentModerationItem(ctx context.Context, reviewerID, itemID string) (domain.ContentModerationReviewResult, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return domain.ContentModerationReviewResult{}, NewValidationError("item_id is required")
	}
	item, err := s.repo.GetContentModerationItem(ctx, itemID)
	if err != nil {
		return domain.ContentModerationReviewResult{}, err
	}
	if item.Status != domain.ContentModerationStatusPending {
		return domain.ContentModerationReviewResult{}, NewValidationError("moderation item is already reviewed")
	}
	if err := s.ensureGroupModerator(ctx, reviewerID, item.GroupID); err != nil {
		return domain.ContentModerationReviewResult{}, err
	}

	var publishedID string
	result := domain.ContentModerationReviewResult{}
	switch item.ContentType {
	case domain.ContentTypeGroupMessage:
		message, err := s.createMessage(ctx, item.AuthorID, item.GroupID, SendMessageInput{Text: item.Body}, false)
		if err != nil {
			return domain.ContentModerationReviewResult{}, err
		}
		publishedID = message.ID
		result.Message = &message
	case domain.ContentTypePublicRequest:
		request, err := s.createPublicRequest(ctx, item.AuthorID, item.GroupID, CreatePublicRequestInput{
			RequestType:     item.RequestType,
			InteractionMode: item.InteractionMode,
			Title:           item.Title,
			Body:            item.Body,
		}, false)
		if err != nil {
			return domain.ContentModerationReviewResult{}, err
		}
		publishedID = request.ID
		result.PublicRequest = &request
	case domain.ContentTypePublicRequestComment:
		comment, err := s.createPublicRequestComment(ctx, item.AuthorID, item.TargetID, CreatePublicRequestCommentInput{Body: item.Body}, false)
		if err != nil {
			return domain.ContentModerationReviewResult{}, err
		}
		publishedID = comment.ID
		result.Comment = &comment
	default:
		return domain.ContentModerationReviewResult{}, NewValidationError("content_type is invalid")
	}

	reviewed, err := s.repo.ReviewContentModerationItem(ctx, itemID, domain.ContentModerationStatusApproved, reviewerID, publishedID)
	if err != nil {
		return domain.ContentModerationReviewResult{}, err
	}
	_ = s.storeAdaptiveModerationFeedback(ctx, item, "allow", itemID)
	s.RecordEvent(ctx, reviewerID, "content_moderation_approved", string(item.ContentType), itemID)
	result.Item = reviewed
	return result, nil
}

func (s *Service) RejectContentModerationItem(ctx context.Context, reviewerID, itemID string) (domain.ContentModerationItem, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return domain.ContentModerationItem{}, NewValidationError("item_id is required")
	}
	item, err := s.repo.GetContentModerationItem(ctx, itemID)
	if err != nil {
		return domain.ContentModerationItem{}, err
	}
	if item.Status != domain.ContentModerationStatusPending {
		return domain.ContentModerationItem{}, NewValidationError("moderation item is already reviewed")
	}
	if err := s.ensureGroupModerator(ctx, reviewerID, item.GroupID); err != nil {
		return domain.ContentModerationItem{}, err
	}
	reviewed, err := s.repo.ReviewContentModerationItem(ctx, itemID, domain.ContentModerationStatusRejected, reviewerID, "")
	if err != nil {
		return domain.ContentModerationItem{}, err
	}
	_ = s.storeAdaptiveModerationFeedback(ctx, item, "deny", itemID)
	s.RecordEvent(ctx, reviewerID, "content_moderation_rejected", string(item.ContentType), itemID)
	return reviewed, nil
}

func (s *Service) ensureGroupModerator(ctx context.Context, userID, groupID string) error {
	role, err := s.repo.GetMemberRole(ctx, groupID, userID)
	if err == nil && (role == domain.RoleOwner || role == domain.RoleAdmin) {
		return nil
	}
	user, userErr := s.repo.GetUserByID(ctx, userID)
	if userErr == nil && (user.Role == domain.UserRolePlatformAdmin || user.Role == domain.UserRoleSuperAdmin) {
		return nil
	}
	if err != nil {
		return err
	}
	return storage.ErrForbidden
}

func validContentModerationStatus(value domain.ContentModerationStatus) bool {
	switch value {
	case domain.ContentModerationStatusPending, domain.ContentModerationStatusApproved, domain.ContentModerationStatusRejected:
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
