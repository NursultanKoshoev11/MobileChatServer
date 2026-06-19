package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

const (
	maxPublicRequestTitleLen   = 120
	maxPublicRequestTextLen    = 4000
	maxPublicRequestPayloadLen = 2 * 1024 * 1024
	maxPublicRequestPhotos     = 1
	maxPublicRequestPhotoBytes = 900 * 1024
	maxPublicCommentLen        = 1500
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

type publicRequestBodyMeta struct {
	Text       string
	PhotoCount int
}

type publicRequestPayload struct {
	Version int                         `json:"version"`
	Text    string                      `json:"text"`
	Photos  []publicRequestPayloadPhoto `json:"photos"`
}

type publicRequestPayloadPhoto struct {
	Name      string `json:"name"`
	SizeBytes int    `json:"size_bytes"`
	Base64    string `json:"base64"`
}

func (s *Service) CreatePublicRequest(ctx context.Context, authorID, groupID string, input CreatePublicRequestInput) (domain.PublicRequest, error) {
	return s.createPublicRequest(ctx, authorID, groupID, input, true)
}

func (s *Service) createPublicRequest(ctx context.Context, authorID, groupID string, input CreatePublicRequestInput, runModeration bool) (domain.PublicRequest, error) {
	groupID = strings.TrimSpace(groupID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if groupID == "" {
		return domain.PublicRequest{}, NewValidationError("group_id is required")
	}
	if !validPublicRequestType(input.RequestType) {
		return domain.PublicRequest{}, NewValidationError("request_type is invalid")
	}
	if _, err := s.repo.GetMemberRole(ctx, groupID, authorID); err != nil {
		return domain.PublicRequest{}, err
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
	bodyMeta, err := validatePublicRequestBody(body)
	if err != nil {
		return domain.PublicRequest{}, err
	}
	if runModeration {
		if err := s.moderateContent(ctx, domain.ContentModerationItem{
			GroupID:         groupID,
			ContentType:     domain.ContentTypePublicRequest,
			AuthorID:        authorID,
			Title:           title,
			Body:            body,
			RequestType:     input.RequestType,
			InteractionMode: mode,
		}); err != nil {
			return domain.PublicRequest{}, err
		}
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
	go s.notifyGroupAboutNewPublicRequest(context.Background(), authorID, groupID, request.ID, request.Title, bodyMeta.NotificationBody())
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
	return s.createPublicRequestComment(ctx, authorID, requestID, input, true)
}

func (s *Service) createPublicRequestComment(ctx context.Context, authorID, requestID string, input CreatePublicRequestCommentInput, runModeration bool) (domain.PublicRequestComment, error) {
	requestID = strings.TrimSpace(requestID)
	body := strings.TrimSpace(input.Body)
	if requestID == "" {
		return domain.PublicRequestComment{}, NewValidationError("request_id is required")
	}
	if len(body) < 1 || len(body) > maxPublicCommentLen {
		return domain.PublicRequestComment{}, NewValidationError(fmt.Sprintf("comment must be between 1 and %d characters", maxPublicCommentLen))
	}
	if err := s.ensureCanCommentPublicRequest(ctx, authorID, requestID); err != nil {
		return domain.PublicRequestComment{}, err
	}
	if runModeration {
		requestContext, err := s.GetPublicRequestRealtimeContext(ctx, requestID)
		if err != nil {
			return domain.PublicRequestComment{}, err
		}
		if err := s.moderateContent(ctx, domain.ContentModerationItem{
			GroupID:     requestContext.GroupID,
			ContentType: domain.ContentTypePublicRequestComment,
			AuthorID:    authorID,
			TargetID:    requestID,
			Body:        body,
		}); err != nil {
			return domain.PublicRequestComment{}, err
		}
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
	go s.notifyGroupAboutNewPublicRequestComment(context.Background(), authorID, requestID, comment.Body)
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

func (s *Service) HidePublicRequest(ctx context.Context, moderatorID, requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return NewValidationError("request_id is required")
	}
	if err := s.repo.HidePublicRequest(ctx, requestID, moderatorID); err != nil {
		return err
	}
	s.RecordEvent(ctx, moderatorID, "public_request_hidden", "public_request", requestID)
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

func validatePublicRequestBody(body string) (publicRequestBodyMeta, error) {
	if len(body) > maxPublicRequestPayloadLen {
		return publicRequestBodyMeta{}, NewValidationError(fmt.Sprintf("body must be less than %d bytes", maxPublicRequestPayloadLen))
	}
	meta, isPayload, err := parsePublicRequestBodyPayload(body)
	if err != nil {
		return publicRequestBodyMeta{}, err
	}
	if isPayload {
		if strings.TrimSpace(meta.Text) == "" && meta.PhotoCount == 0 {
			return publicRequestBodyMeta{}, NewValidationError("body text or photo is required")
		}
		if meta.PhotoCount == 0 && (len(meta.Text) < 5 || len(meta.Text) > maxPublicRequestTextLen) {
			return publicRequestBodyMeta{}, NewValidationError(fmt.Sprintf("body text must be between 5 and %d characters", maxPublicRequestTextLen))
		}
		if len(meta.Text) > maxPublicRequestTextLen {
			return publicRequestBodyMeta{}, NewValidationError(fmt.Sprintf("body text must be less than %d characters", maxPublicRequestTextLen))
		}
		return meta, nil
	}
	if len(body) < 5 || len(body) > maxPublicRequestTextLen {
		return publicRequestBodyMeta{}, NewValidationError(fmt.Sprintf("body must be between 5 and %d characters", maxPublicRequestTextLen))
	}
	return publicRequestBodyMeta{Text: body}, nil
}

func parsePublicRequestBodyPayload(body string) (publicRequestBodyMeta, bool, error) {
	raw := strings.TrimSpace(body)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return publicRequestBodyMeta{}, false, nil
	}
	var payload publicRequestPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return publicRequestBodyMeta{}, false, nil
	}
	text := strings.TrimSpace(payload.Text)
	if len(payload.Photos) > maxPublicRequestPhotos {
		return publicRequestBodyMeta{}, true, NewValidationError(fmt.Sprintf("only %d photo is allowed", maxPublicRequestPhotos))
	}
	for _, photo := range payload.Photos {
		if err := validatePublicRequestPhotoPayload(photo); err != nil {
			return publicRequestBodyMeta{}, true, err
		}
	}
	return publicRequestBodyMeta{Text: text, PhotoCount: len(payload.Photos)}, true, nil
}

func validatePublicRequestPhotoPayload(photo publicRequestPayloadPhoto) error {
	data := strings.TrimSpace(photo.Base64)
	if data == "" {
		return NewValidationError("photo data is required")
	}
	if comma := strings.LastIndex(data, ","); comma >= 0 {
		data = data[comma+1:]
	}
	if len(data) > ((maxPublicRequestPhotoBytes+2)/3)*4+8 {
		return NewValidationError(fmt.Sprintf("photo must be less than %d bytes", maxPublicRequestPhotoBytes))
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return NewValidationError("photo data is invalid")
	}
	if len(decoded) == 0 {
		return NewValidationError("photo data is required")
	}
	if len(decoded) > maxPublicRequestPhotoBytes {
		return NewValidationError(fmt.Sprintf("photo must be less than %d bytes", maxPublicRequestPhotoBytes))
	}
	if photo.SizeBytes > 0 && photo.SizeBytes > maxPublicRequestPhotoBytes {
		return NewValidationError(fmt.Sprintf("photo must be less than %d bytes", maxPublicRequestPhotoBytes))
	}
	return nil
}

func moderationBodyForInput(item domain.ContentModerationItem) string {
	if item.ContentType != domain.ContentTypePublicRequest {
		return item.Body
	}
	meta, isPayload, err := parsePublicRequestBodyPayload(item.Body)
	if err != nil || !isPayload {
		return item.Body
	}
	if strings.TrimSpace(meta.Text) != "" {
		return meta.Text
	}
	if meta.PhotoCount > 0 {
		return "[photo]"
	}
	return ""
}

func (m publicRequestBodyMeta) NotificationBody() string {
	if strings.TrimSpace(m.Text) != "" {
		return m.Text
	}
	if m.PhotoCount > 0 {
		return "[photo]"
	}
	return ""
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
