package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

const (
	maxApplicantNameLen      = 120
	maxOrganizationNameLen   = 160
	maxOrganizationTypeLen   = 80
	maxRegionLen             = 120
	maxOfficialContactLen    = 160
	maxGroupRequestReasonLen = 2000
	maxDocumentsLen          = 16 * 1024 * 1024
)

type CreateGroupCreationRequestInput struct {
	ApplicantName    string `json:"applicant_name"`
	Position         string `json:"position"`
	OrganizationName string `json:"organization_name"`
	OrganizationType string `json:"organization_type"`
	Region           string `json:"region"`
	OfficialPhone    string `json:"official_phone"`
	OfficialEmail    string `json:"official_email"`
	Website          string `json:"website"`
	GroupTitle       string `json:"group_title"`
	GroupDescription string `json:"group_description"`
	Reason           string `json:"reason"`
	Documents        string `json:"documents"`
}

type ReviewGroupCreationRequestInput struct {
	AdminComment string `json:"admin_comment"`
}

func (s *Service) CreateGroupCreationRequest(ctx context.Context, requester domain.User, input CreateGroupCreationRequestInput) (domain.GroupCreationRequest, error) {
	request := domain.GroupCreationRequest{
		ID:               "GCR-" + strings.ToUpper(randomHex(12)),
		RequesterID:      requester.ID,
		ApplicantName:    strings.TrimSpace(input.ApplicantName),
		Position:         strings.TrimSpace(input.Position),
		OrganizationName: strings.TrimSpace(input.OrganizationName),
		OrganizationType: strings.TrimSpace(input.OrganizationType),
		Region:           strings.TrimSpace(input.Region),
		OfficialPhone:    strings.TrimSpace(input.OfficialPhone),
		OfficialEmail:    strings.TrimSpace(input.OfficialEmail),
		Website:          strings.TrimSpace(input.Website),
		GroupTitle:       strings.TrimSpace(input.GroupTitle),
		GroupDescription: strings.TrimSpace(input.GroupDescription),
		Reason:           strings.TrimSpace(input.Reason),
		Documents:        strings.TrimSpace(input.Documents),
	}
	if request.ApplicantName == "" {
		request.ApplicantName = requester.DisplayName
	}
	if err := validateGroupCreationRequest(request); err != nil {
		return domain.GroupCreationRequest{}, err
	}
	created, err := s.repo.CreateGroupCreationRequest(ctx, request)
	if err != nil {
		return domain.GroupCreationRequest{}, err
	}
	s.RecordEvent(ctx, requester.ID, "group_creation_request_submitted", "group_creation_request", created.ID)
	return created, nil
}

func (s *Service) ListMyGroupCreationRequests(ctx context.Context, requester domain.User) ([]domain.GroupCreationRequest, error) {
	return s.repo.ListMyGroupCreationRequests(ctx, requester.ID)
}

func (s *Service) ListGroupCreationRequestsForAdmin(ctx context.Context, admin domain.User, status string, limit int) ([]domain.GroupCreationRequest, error) {
	if !isPlatformAdmin(admin) {
		return nil, storage.ErrForbidden
	}
	return s.repo.ListGroupCreationRequestsForAdmin(ctx, strings.TrimSpace(status), limit)
}

func (s *Service) ApproveGroupCreationRequest(ctx context.Context, admin domain.User, requestID string, input ReviewGroupCreationRequestInput) (domain.GroupCreationRequest, error) {
	if !isPlatformAdmin(admin) {
		return domain.GroupCreationRequest{}, storage.ErrForbidden
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return domain.GroupCreationRequest{}, NewValidationError("request_id is required")
	}
	request, err := s.repo.GetGroupCreationRequestByID(ctx, requestID)
	if err != nil {
		return domain.GroupCreationRequest{}, err
	}
	group := domain.Group{
		ID:          "G-" + strings.ToUpper(randomHex(8)),
		Title:       request.GroupTitle,
		Description: request.GroupDescription,
		Visibility:  domain.VisibilityPublic,
		OwnerID:     request.RequesterID,
		InviteCode:  randomInviteCode(),
	}
	var approved domain.GroupCreationRequest
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		group.InviteCode = randomInviteCode()
		approved, err = s.repo.ApproveGroupCreationRequest(ctx, requestID, admin.ID, strings.TrimSpace(input.AdminComment), group)
		if err == nil {
			s.RecordEvent(ctx, admin.ID, "group_creation_request_approved", "group_creation_request", approved.ID)
			return approved, nil
		}
		lastErr = err
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return domain.GroupCreationRequest{}, err
		}
	}
	return domain.GroupCreationRequest{}, fmt.Errorf("generate group invite code: %w", lastErr)
}

func (s *Service) RejectGroupCreationRequest(ctx context.Context, admin domain.User, requestID string, input ReviewGroupCreationRequestInput) (domain.GroupCreationRequest, error) {
	if !isPlatformAdmin(admin) {
		return domain.GroupCreationRequest{}, storage.ErrForbidden
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return domain.GroupCreationRequest{}, NewValidationError("request_id is required")
	}
	request, err := s.repo.RejectGroupCreationRequest(ctx, requestID, admin.ID, strings.TrimSpace(input.AdminComment))
	if err != nil {
		return domain.GroupCreationRequest{}, err
	}
	s.RecordEvent(ctx, admin.ID, "group_creation_request_rejected", "group_creation_request", request.ID)
	return request, nil
}

func (s *Service) NeedMoreInfoForGroupCreationRequest(ctx context.Context, admin domain.User, requestID string, input ReviewGroupCreationRequestInput) (domain.GroupCreationRequest, error) {
	if !isPlatformAdmin(admin) {
		return domain.GroupCreationRequest{}, storage.ErrForbidden
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return domain.GroupCreationRequest{}, NewValidationError("request_id is required")
	}
	request, err := s.repo.MarkGroupCreationRequestNeedsMoreInfo(ctx, requestID, admin.ID, strings.TrimSpace(input.AdminComment))
	if err != nil {
		return domain.GroupCreationRequest{}, err
	}
	s.RecordEvent(ctx, admin.ID, "group_creation_request_needs_more_info", "group_creation_request", request.ID)
	return request, nil
}

func validateGroupCreationRequest(request domain.GroupCreationRequest) error {
	if len(request.ApplicantName) < 2 || len(request.ApplicantName) > maxApplicantNameLen {
		return NewValidationError(fmt.Sprintf("applicant_name must be between 2 and %d characters", maxApplicantNameLen))
	}
	if request.Position == "" || len(request.Position) > maxOrganizationTypeLen {
		return NewValidationError(fmt.Sprintf("position is required and must be at most %d characters", maxOrganizationTypeLen))
	}
	if len(request.OrganizationName) < 2 || len(request.OrganizationName) > maxOrganizationNameLen {
		return NewValidationError(fmt.Sprintf("organization_name must be between 2 and %d characters", maxOrganizationNameLen))
	}
	if request.OrganizationType == "" || len(request.OrganizationType) > maxOrganizationTypeLen {
		return NewValidationError(fmt.Sprintf("organization_type is required and must be at most %d characters", maxOrganizationTypeLen))
	}
	if request.Region == "" || len(request.Region) > maxRegionLen {
		return NewValidationError(fmt.Sprintf("region is required and must be at most %d characters", maxRegionLen))
	}
	if request.OfficialPhone == "" || len(request.OfficialPhone) > maxOfficialContactLen {
		return NewValidationError(fmt.Sprintf("official_phone is required and must be at most %d characters", maxOfficialContactLen))
	}
	if request.OfficialEmail == "" || len(request.OfficialEmail) > maxOfficialContactLen {
		return NewValidationError(fmt.Sprintf("official_email is required and must be at most %d characters", maxOfficialContactLen))
	}
	if len(request.GroupTitle) < 3 || len(request.GroupTitle) > maxGroupTitleLen {
		return NewValidationError(fmt.Sprintf("group_title must be between 3 and %d characters", maxGroupTitleLen))
	}
	if len(request.GroupDescription) > maxDescriptionLen {
		return NewValidationError(fmt.Sprintf("group_description must be at most %d characters", maxDescriptionLen))
	}
	if len(request.Reason) < 5 || len(request.Reason) > maxGroupRequestReasonLen {
		return NewValidationError(fmt.Sprintf("reason must be between 5 and %d characters", maxGroupRequestReasonLen))
	}
	if len(request.Documents) > maxDocumentsLen {
		return NewValidationError(fmt.Sprintf("documents must be at most %d characters", maxDocumentsLen))
	}
	return nil
}

func isPlatformAdmin(user domain.User) bool {
	return user.Role == domain.UserRolePlatformAdmin || user.Role == domain.UserRoleSuperAdmin
}
