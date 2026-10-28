package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/moderation"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

const (
	moderationRejectWindow       = 15 * time.Minute
	moderationRejectThreshold    = 3
	moderationAutoMuteDuration   = 60 * time.Minute
	moderationTemporaryBlockText = "content is temporarily limited after repeated moderation rejects"
)

type ContentModerationPendingError struct {
	Item domain.ContentModerationItem
}

func (e ContentModerationPendingError) Error() string {
	return "content requires moderation review"
}

type ContentModerationReviewNotice struct {
	Item domain.ContentModerationItem
}

func (s *Service) SetContentModerator(moderator moderation.Moderator) {
	s.contentModerator = moderator
}

func (s *Service) moderateContent(ctx context.Context, item domain.ContentModerationItem) (*ContentModerationReviewNotice, error) {
	if s.contentModerator == nil {
		return nil, nil
	}
	if err := s.enforceModerationRateLimit(ctx, item); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("moderate content: %w", err)
	}
	return s.handleModerationDecision(ctx, item, decision)
}

func (s *Service) handleModerationDecision(ctx context.Context, item domain.ContentModerationItem, decision moderation.Decision) (*ContentModerationReviewNotice, error) {
	if decision.Action == moderation.ActionAllow {
		return nil, nil
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
	if decision.Action != moderation.ActionBlock {
		// The publication and pending moderation row are persisted together by
		// persistModeratedPublication once the resource ID is known.
		return &ContentModerationReviewNotice{Item: queued}, nil
	}
	var created domain.ContentModerationItem
	if err := s.recordAuditedMutation(ctx, item.AuthorID, "content_moderation_auto_rejected", string(item.ContentType), queued.ID, func(repo *storage.Repository) error {
		var createErr error
		created, createErr = repo.CreateContentModerationItem(ctx, queued)
		if createErr != nil {
			return createErr
		}
		if decision.Action == moderation.ActionBlock {
			return s.applyModerationAutoMuteInTransaction(ctx, repo, created)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if decision.Action == moderation.ActionBlock {
		return nil, NewValidationError("content is not allowed")
	}
	return &ContentModerationReviewNotice{Item: created}, nil
}

func (s *Service) persistModeratedPublication(ctx context.Context, notice *ContentModerationReviewNotice, publish func(*storage.Repository) (string, error)) (string, error) {
	return s.persistPublication(ctx, notice, publish, nil)
}

func (s *Service) persistModeratedPublicationWithAudit(ctx context.Context, notice *ContentModerationReviewNotice, publish func(*storage.Repository) (string, error), event storage.AuditEvent) (string, error) {
	return s.persistPublication(ctx, notice, publish, &event)
}

func (s *Service) persistPublication(ctx context.Context, notice *ContentModerationReviewNotice, publish func(*storage.Repository) (string, error), additionalEvent *storage.AuditEvent) (string, error) {
	if publish == nil {
		return "", fmt.Errorf("publication callback is required")
	}
	if notice == nil {
		if additionalEvent == nil {
			return publish(s.repo)
		}
		var publishedResourceID string
		err := s.repo.WithAuditEvent(ctx, *additionalEvent, func(repo *storage.Repository) error {
			resourceID, err := publish(repo)
			if err != nil {
				return err
			}
			publishedResourceID = resourceID
			return nil
		})
		if err != nil {
			return "", err
		}
		return publishedResourceID, nil
	}
	item := notice.Item
	if strings.TrimSpace(item.ID) == "" || item.Status != domain.ContentModerationStatusPending {
		return "", fmt.Errorf("pending moderation item is invalid")
	}

	var publishedResourceID string
	if err := s.recordAuditedMutation(ctx, item.AuthorID, "content_moderation_queued", string(item.ContentType), item.ID, func(repo *storage.Repository) error {
		resourceID, err := publish(repo)
		if err != nil {
			return err
		}
		resourceID = strings.TrimSpace(resourceID)
		if resourceID == "" {
			return fmt.Errorf("moderated publication returned an empty resource id")
		}
		item.PublishedResourceID = resourceID
		created, err := repo.CreateContentModerationItem(ctx, item)
		if err != nil {
			return err
		}
		notice.Item = created
		if additionalEvent != nil {
			if err := repo.CreateAuditEvent(ctx, additionalEvent.ID, additionalEvent.ActorUserID, additionalEvent.EventType, additionalEvent.ResourceType, additionalEvent.ResourceID, additionalEvent.MetadataJSON); err != nil {
				return fmt.Errorf("persist publication audit event: %w", err)
			}
		}
		publishedResourceID = resourceID
		return nil
	}); err != nil {
		return "", err
	}
	go s.NotifyAdminsAboutContentModerationPending(context.Background(), notice.Item)
	return publishedResourceID, nil
}

func (s *Service) enforceModerationRateLimit(ctx context.Context, item domain.ContentModerationItem) error {
	count, err := s.repo.CountRecentRejectedContentModerationItems(ctx, item.GroupID, item.AuthorID, time.Now().UTC().Add(-moderationRejectWindow))
	if err != nil {
		return nil
	}
	if count >= moderationRejectThreshold {
		return NewValidationError(moderationTemporaryBlockText)
	}
	return nil
}

func (s *Service) applyModerationAutoMuteInTransaction(ctx context.Context, repo *storage.Repository, item domain.ContentModerationItem) error {
	count, err := repo.CountRecentRejectedContentModerationItems(ctx, item.GroupID, item.AuthorID, time.Now().UTC().Add(-moderationRejectWindow))
	if err != nil {
		return err
	}
	if count < moderationRejectThreshold {
		return nil
	}
	until := time.Now().UTC().Add(moderationAutoMuteDuration)
	applied, err := repo.SetGroupCommentAutoMute(ctx, item.GroupID, item.AuthorID, &until, "Auto-muted after repeated moderation blocks/rejects")
	if err != nil {
		return err
	}
	if !applied {
		return nil
	}
	return repo.CreateAuditEvent(ctx, "EVT-"+strings.ToUpper(randomHex(12)), item.AuthorID, "content_moderation_auto_muted", string(item.ContentType), item.ID, "{}")
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

func (s *Service) CountContentModerationItems(ctx context.Context, reviewerID, groupID, status string) (int, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return 0, NewValidationError("group_id is required")
	}
	if err := s.ensureGroupModerator(ctx, reviewerID, groupID); err != nil {
		return 0, err
	}
	moderationStatus := domain.ContentModerationStatus(strings.TrimSpace(status))
	if moderationStatus != "" && !validContentModerationStatus(moderationStatus) {
		return 0, NewValidationError("status is invalid")
	}
	return s.repo.CountContentModerationItems(ctx, groupID, moderationStatus)
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
	result := domain.ContentModerationReviewResult{}
	var reviewedSource domain.ContentModerationItem
	requestNotificationBody := ""
	if err := s.recordAuditedMutation(ctx, reviewerID, "content_moderation_approved", string(item.ContentType), itemID, func(repo *storage.Repository) error {
		current, err := repo.LockContentModerationItem(ctx, itemID)
		if err != nil {
			return err
		}
		if current.Status != domain.ContentModerationStatusPending {
			return NewValidationError("moderation item is already reviewed")
		}
		if err := repo.LockGroupModerator(ctx, reviewerID, current.GroupID); err != nil {
			return err
		}
		reviewedSource = current

		publishedID := strings.TrimSpace(current.PublishedResourceID)
		if publishedID == "" {
			switch current.ContentType {
			case domain.ContentTypeGroupMessage:
				body := strings.TrimSpace(current.Body)
				if body == "" {
					return NewValidationError("text is required")
				}
				if len(body) > maxMessageLen {
					return NewValidationError(fmt.Sprintf("text must be at most %d characters", maxMessageLen))
				}
				message, err := repo.CreateMessage(ctx, domain.Message{
					ID:       "M-" + strings.ToUpper(randomHex(12)),
					GroupID:  current.GroupID,
					SenderID: current.AuthorID,
					Text:     body,
				})
				if err != nil {
					return err
				}
				publishedID = message.ID
				result.Message = &message
			case domain.ContentTypePublicRequest:
				title := strings.TrimSpace(current.Title)
				if !validPublicRequestType(current.RequestType) {
					return NewValidationError("request_type is invalid")
				}
				mode := current.InteractionMode
				if mode == "" {
					mode = domain.InteractionModeDiscussion
				}
				if !validInteractionMode(mode) {
					return NewValidationError("interaction_mode is invalid")
				}
				if len(title) < 3 || len(title) > maxPublicRequestTitleLen {
					return NewValidationError(fmt.Sprintf("title must be between 3 and %d characters", maxPublicRequestTitleLen))
				}
				bodyMeta, err := validatePublicRequestBody(current.Body, current.GroupID)
				if err != nil {
					return err
				}
				request, err := repo.CreatePublicRequest(ctx, domain.PublicRequest{
					ID:              "REQ-" + strings.ToUpper(randomHex(12)),
					GroupID:         current.GroupID,
					AuthorID:        current.AuthorID,
					RequestType:     current.RequestType,
					InteractionMode: mode,
					Title:           title,
					Body:            strings.TrimSpace(current.Body),
				})
				if err != nil {
					return err
				}
				publishedID = request.ID
				result.PublicRequest = &request
				requestNotificationBody = bodyMeta.NotificationBody()
			case domain.ContentTypePublicRequestComment:
				body := strings.TrimSpace(current.Body)
				if len(body) < 1 || len(body) > maxPublicCommentLen {
					return NewValidationError(fmt.Sprintf("comment must be between 1 and %d characters", maxPublicCommentLen))
				}
				comment, err := repo.CreatePublicRequestComment(ctx, domain.PublicRequestComment{
					ID:        "COM-" + strings.ToUpper(randomHex(12)),
					RequestID: current.TargetID,
					AuthorID:  current.AuthorID,
					Body:      body,
				})
				if err != nil {
					return err
				}
				publishedID = comment.ID
				result.Comment = &comment
			default:
				return NewValidationError("content_type is invalid")
			}
		}

		reviewed, err := repo.ReviewContentModerationItem(ctx, itemID, domain.ContentModerationStatusApproved, reviewerID, publishedID)
		if err != nil {
			return err
		}
		result.Item = reviewed
		return nil
	}); err != nil {
		return domain.ContentModerationReviewResult{}, err
	}
	_ = s.storeAdaptiveModerationFeedback(ctx, reviewedSource, "allow", itemID)
	if result.PublicRequest != nil {
		request := result.PublicRequest
		go s.notifyGroupAboutNewPublicRequest(context.Background(), request.AuthorID, request.GroupID, request.ID, request.Title, requestNotificationBody)
	}
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
	var reviewed domain.ContentModerationItem
	var deletedMessage *domain.Message
	if err := s.recordAuditedMutation(ctx, reviewerID, "content_moderation_rejected", string(item.ContentType), itemID, func(repo *storage.Repository) error {
		current, err := repo.LockContentModerationItem(ctx, itemID)
		if err != nil {
			return err
		}
		if current.Status != domain.ContentModerationStatusPending {
			return NewValidationError("moderation item is already reviewed")
		}
		if err := repo.LockGroupModerator(ctx, reviewerID, current.GroupID); err != nil {
			return err
		}
		deletedMessage, err = s.removePublishedModerationResource(ctx, repo, reviewerID, current)
		if err != nil {
			return err
		}
		reviewed, err = repo.ReviewContentModerationItem(ctx, itemID, domain.ContentModerationStatusRejected, reviewerID, current.PublishedResourceID)
		if err != nil {
			return err
		}
		return s.applyModerationAutoMuteInTransaction(ctx, repo, reviewed)
	}); err != nil {
		return domain.ContentModerationItem{}, err
	}
	_ = s.storeAdaptiveModerationFeedback(ctx, item, "deny", itemID)
	if deletedMessage != nil {
		go s.NotifyGroupAboutMessageDeleted(context.Background(), reviewerID, *deletedMessage)
	}
	return reviewed, nil
}

func (s *Service) removePublishedModerationResource(ctx context.Context, repo *storage.Repository, reviewerID string, item domain.ContentModerationItem) (*domain.Message, error) {
	publishedID := strings.TrimSpace(item.PublishedResourceID)
	if publishedID == "" {
		return nil, nil
	}
	switch item.ContentType {
	case domain.ContentTypeGroupMessage:
		message, err := repo.DeleteMessage(ctx, item.GroupID, reviewerID, publishedID)
		if err != nil {
			return nil, err
		}
		return &message, nil
	case domain.ContentTypePublicRequest:
		return nil, repo.HidePublicRequest(ctx, publishedID, reviewerID)
	case domain.ContentTypePublicRequestComment:
		_, err := repo.DeletePublicRequestComment(ctx, publishedID, reviewerID)
		return nil, err
	default:
		return nil, NewValidationError("content_type is invalid")
	}
}

func (s *Service) ensureGroupModerator(ctx context.Context, userID, groupID string) error {
	role, err := s.repo.GetMemberRole(ctx, groupID, userID)
	if err == nil && (role == domain.RoleOwner || role == domain.RoleAdmin) {
		return nil
	}
	user, userErr := s.repo.GetUserByID(ctx, userID)
	if userErr == nil && user.Role.CanModerateAnyGroup() {
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
