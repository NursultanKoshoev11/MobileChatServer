package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

const (
	maxPublicRequestTitleLen = 120
	maxPublicRequestBodyLen  = 4000
	maxPublicCommentLen      = 1500
)

type CreatePublicRequestInput struct {
	RequestType     domain.PublicRequestType            `json:"request_type"`
	InteractionMode domain.PublicRequestInteractionMode `json:"interaction_mode"`
	Title           string                              `json:"title"`
	Body            string                              `json:"body"`
}

type CreatePublicRequestCommentInput struct {
	Body string `json:"body"`
}

type UpdatePublicRequestStatusInput struct {
	Status domain.PublicRequestStatus `json:"status"`
}

func (s *Service) CreatePublicRequest(ctx context.Context, authorID, groupID string, input CreatePublicRequestInput) (domain.PublicRequest, error) {
	groupID = strings.TrimSpace(groupID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if groupID == "" {
		return domain.PublicRequest{}, NewValidationError("group_id is required")
	}
	if !validPublicRequestType(input.RequestType) {
		return domain.PublicRequest{}, NewValidationError("request_type is invalid")
	}
	mode := input.InteractionMode
	if mode == "" {
		mode = domain.InteractionModeDiscussion
	}
	if !validInteractionMode(mode) {
		return domain.PublicRequest{}, NewValidationError("interaction_mode is invalid")
	}
	if len(title) < 3 || len(title) > maxPublicRequestTitleLen {
		return domain.PublicRequest{}, NewValidationError(fmt.Sprintf("title must be between 3 and %d characters", maxPublicRequestTitleLen))
	}
	if len(body) < 5 || len(body) > maxPublicRequestBodyLen {
		return domain.PublicRequest{}, NewValidationError(fmt.Sprintf("body must be between 5 and %d characters", maxPublicRequestBodyLen))
	}
	request, err := s.repo.CreatePublicRequest(ctx, domain.PublicRequest{
		ID:              "REQ-" + strings.ToUpper(randomHex(12)),
		GroupID:         groupID,
		AuthorID:        authorID,
		RequestType:     input.RequestType,
		InteractionMode: mode,
		Title:           title,
		Body:            body,
	})
	if err != nil {
		return domain.PublicRequest{}, err
	}
	s.RecordEvent(ctx, authorID, "public_request_created", "group", groupID)
	return request, nil
}

func (s *Service) ListPublicRequests(ctx context.Context, viewerID, groupID string, limit int, before time.Time, mineOnly bool) ([]domain.PublicRequest, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	return s.repo.ListPublicRequests(ctx, groupID, viewerID, limit, before, mineOnly)
}

func (s *Service) VotePublicRequest(ctx context.Context, userID, requestID, voteType string) error {
	requestID = strings.TrimSpace(requestID)
	voteType = strings.TrimSpace(voteType)
	if requestID == "" {
		return NewValidationError("request_id is required")
	}
	if voteType != "support" && voteType != "oppose" {
		return NewValidationError("vote_type must be support or oppose")
	}
	if err := s.repo.VotePublicRequest(ctx, requestID, userID, voteType); err != nil {
		return err
	}
	s.RecordEvent(ctx, userID, "public_request_voted", "public_request", requestID)
	return nil
}

func (s *Service) ClearPublicRequestVote(ctx context.Context, userID, requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return NewValidationError("request_id is required")
	}
	if err := s.repo.ClearPublicRequestVote(ctx, requestID, userID); err != nil {
		return err
	}
	s.RecordEvent(ctx, userID, "public_request_vote_cleared", "public_request", requestID)
	return nil
}

func (s *Service) CreatePublicRequestComment(ctx context.Context, authorID, requestID string, input CreatePublicRequestCommentInput) (domain.PublicRequestComment, error) {
	requestID = strings.TrimSpace(requestID)
	body := strings.TrimSpace(input.Body)
	if requestID == "" {
		return domain.PublicRequestComment{}, NewValidationError("request_id is required")
	}
	if len(body) < 1 || len(body) > maxPublicCommentLen {
		return domain.PublicRequestComment{}, NewValidationError(fmt.Sprintf("comment must be between 1 and %d characters", maxPublicCommentLen))
	}
	comment, err := s.repo.CreatePublicRequestComment(ctx, domain.PublicRequestComment{
		ID:        "COM-" + strings.ToUpper(randomHex(12)),
		RequestID: requestID,
		AuthorID:  authorID,
		Body:      body,
	})
	if err != nil {
		return domain.PublicRequestComment{}, err
	}
	s.RecordEvent(ctx, authorID, "public_request_commented", "public_request", requestID)
	return comment, nil
}

func (s *Service) ListPublicRequestComments(ctx context.Context, viewerID, requestID string) ([]domain.PublicRequestComment, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, NewValidationError("request_id is required")
	}
	return s.repo.ListPublicRequestComments(ctx, requestID, viewerID)
}

func (s *Service) DeletePublicRequestComment(ctx context.Context, moderatorID, commentID string) error {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return NewValidationError("comment_id is required")
	}
	if err := s.repo.DeletePublicRequestComment(ctx, commentID, moderatorID); err != nil {
		return err
	}
	s.RecordEvent(ctx, moderatorID, "public_request_comment_deleted", "comment", commentID)
	return nil
}

func (s *Service) UpdatePublicRequestStatus(ctx context.Context, adminID, requestID string, status domain.PublicRequestStatus) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return NewValidationError("request_id is required")
	}
	if !validPublicRequestStatus(status) {
		return NewValidationError("status is invalid")
	}
	if err := s.repo.UpdatePublicRequestStatus(ctx, requestID, adminID, status); err != nil {
		return err
	}
	s.RecordEvent(ctx, adminID, "public_request_status_updated", "public_request", requestID)
	return nil
}

func validPublicRequestType(value domain.PublicRequestType) bool {
	switch value {
	case domain.PublicRequestAnnouncement, domain.PublicRequestSuggestion, domain.PublicRequestComplaint, domain.PublicRequestRequirement, domain.PublicRequestProblem, domain.PublicRequestIdea:
		return true
	default:
		return false
	}
}

func validInteractionMode(value domain.PublicRequestInteractionMode) bool {
	switch value {
	case domain.InteractionModeReadOnly, domain.InteractionModeVoteOnly, domain.InteractionModeDiscussion:
		return true
	default:
		return false
	}
}

func validPublicRequestStatus(value domain.PublicRequestStatus) bool {
	switch value {
	case domain.PublicRequestStatusNew, domain.PublicRequestStatusUnderReview, domain.PublicRequestStatusAccepted, domain.PublicRequestStatusRejected, domain.PublicRequestStatusResolved:
		return true
	default:
		return false
	}
}
