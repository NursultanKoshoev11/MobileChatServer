package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

const maxReportDetailsLen = 1000

// RemovedContent describes content taken down while resolving a report.
type RemovedContent = storage.RemovedContent

type CreateReportInput struct {
	TargetType domain.ReportTargetType `json:"target_type"`
	TargetID   string                  `json:"target_id"`
	Reason     domain.ReportReason     `json:"reason"`
	Details    string                  `json:"details"`
}

type ResolveReportInput struct {
	Action domain.ReportResolution `json:"action"`
}

// ValidateCreateReportInput normalizes and validates a report request.
func ValidateCreateReportInput(input CreateReportInput) (CreateReportInput, error) {
	input.TargetType = domain.ReportTargetType(strings.TrimSpace(string(input.TargetType)))
	input.TargetID = strings.TrimSpace(input.TargetID)
	input.Reason = domain.ReportReason(strings.TrimSpace(string(input.Reason)))
	input.Details = strings.TrimSpace(input.Details)
	if !input.TargetType.Valid() {
		return input, NewValidationError("target_type must be public_request, comment, message or user")
	}
	if input.TargetID == "" || len(input.TargetID) > 128 {
		return input, NewValidationError("target_id is required")
	}
	if !input.Reason.Valid() {
		return input, NewValidationError("reason must be spam, abuse, illegal or other")
	}
	if utf8.RuneCountInString(input.Details) > maxReportDetailsLen {
		return input, NewValidationError(fmt.Sprintf("details must be at most %d characters", maxReportDetailsLen))
	}
	return input, nil
}

// ParseReportStatus validates a status filter; empty means open.
func ParseReportStatus(raw string) (domain.ReportStatus, error) {
	switch strings.TrimSpace(raw) {
	case "", string(domain.ReportStatusOpen):
		return domain.ReportStatusOpen, nil
	case string(domain.ReportStatusResolved):
		return domain.ReportStatusResolved, nil
	default:
		return "", NewValidationError("status must be open or resolved")
	}
}

// CreateReport files a report. created is false when the reporter already had
// an open report on the same target; that report is returned unchanged.
func (s *Service) CreateReport(ctx context.Context, reporterID string, input CreateReportInput) (domain.ContentReport, bool, error) {
	input, err := ValidateCreateReportInput(input)
	if err != nil {
		return domain.ContentReport{}, false, err
	}
	if input.TargetType == domain.ReportTargetUser && input.TargetID == reporterID {
		return domain.ContentReport{}, false, NewValidationError("you cannot report yourself")
	}
	target, err := s.repo.ResolveReportTarget(ctx, input.TargetType, input.TargetID, reporterID)
	if err != nil {
		return domain.ContentReport{}, false, err
	}
	report := domain.ContentReport{
		ID:           "RPT-" + strings.ToUpper(randomHex(12)),
		TargetType:   input.TargetType,
		TargetID:     input.TargetID,
		GroupID:      target.GroupID,
		TargetUserID: target.AuthorID,
		Reason:       input.Reason,
		Details:      input.Details,
		ReporterID:   reporterID,
	}
	var created bool
	var stored domain.ContentReport
	err = s.recordAuditedMutation(ctx, reporterID, "content_reported", "content_report", report.ID, func(repo *storage.Repository) error {
		var createErr error
		stored, created, createErr = repo.CreateContentReport(ctx, report)
		return createErr
	})
	if err != nil {
		return domain.ContentReport{}, false, err
	}
	return stored, created, nil
}

func canReviewPlatformReports(role domain.UserRole) bool {
	return role.IsPlatformAdmin() || role.IsSuperAdmin()
}

// ListGroupReports returns reports about content in a group to its owner or admins.
func (s *Service) ListGroupReports(ctx context.Context, viewerID, groupID string, status domain.ReportStatus) ([]domain.ContentReport, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	isModerator, err := s.repo.IsGroupModeratorMember(ctx, groupID, viewerID)
	if err != nil {
		return nil, err
	}
	if !isModerator {
		return nil, storage.ErrForbidden
	}
	return s.repo.ListGroupContentReports(ctx, groupID, status)
}

// ListAllReports returns platform-wide reports to platform admins. The role is
// re-read from the database so a revoked admin loses access immediately.
func (s *Service) ListAllReports(ctx context.Context, viewerID string, status domain.ReportStatus) ([]domain.ContentReport, error) {
	viewer, err := s.repo.GetUserByID(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	if !canReviewPlatformReports(viewer.Role) {
		return nil, storage.ErrForbidden
	}
	return s.repo.ListAllContentReports(ctx, status)
}

// ResolveReport closes a report (and every other open report on the same
// target). content_removed also takes the content down.
func (s *Service) ResolveReport(ctx context.Context, actorID, reportID string, input ResolveReportInput) (domain.ContentReport, storage.RemovedContent, error) {
	reportID = strings.TrimSpace(reportID)
	if reportID == "" {
		return domain.ContentReport{}, storage.RemovedContent{}, NewValidationError("report_id is required")
	}
	action := domain.ReportResolution(strings.TrimSpace(string(input.Action)))
	if !action.Valid() {
		return domain.ContentReport{}, storage.RemovedContent{}, NewValidationError("action must be dismissed or content_removed")
	}
	report, err := s.repo.GetContentReport(ctx, reportID)
	if err != nil {
		return domain.ContentReport{}, storage.RemovedContent{}, err
	}
	if err := s.ensureCanResolveReport(ctx, actorID, report); err != nil {
		return domain.ContentReport{}, storage.RemovedContent{}, err
	}
	var removed storage.RemovedContent
	err = s.recordAuditedMutation(ctx, actorID, "content_report_"+string(action), "content_report", report.ID, func(repo *storage.Repository) error {
		if action == domain.ReportResolutionContentRemoved {
			var removeErr error
			removed, removeErr = repo.RemoveReportedContent(ctx, report.TargetType, report.TargetID, actorID)
			if removeErr != nil {
				return removeErr
			}
		}
		return repo.ResolveContentReports(ctx, report, action, actorID)
	})
	if err != nil {
		return domain.ContentReport{}, storage.RemovedContent{}, err
	}
	updated, err := s.repo.GetContentReport(ctx, report.ID)
	if err != nil {
		return domain.ContentReport{}, storage.RemovedContent{}, err
	}
	return updated, removed, nil
}

func (s *Service) ensureCanResolveReport(ctx context.Context, actorID string, report domain.ContentReport) error {
	if report.GroupID != "" {
		isModerator, err := s.repo.IsGroupModeratorMember(ctx, report.GroupID, actorID)
		if err != nil {
			return err
		}
		if isModerator {
			return nil
		}
	}
	actor, err := s.repo.GetUserByID(ctx, actorID)
	if err != nil {
		return err
	}
	if canReviewPlatformReports(actor.Role) {
		return nil
	}
	return storage.ErrForbidden
}

func (s *Service) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	blockedID = strings.TrimSpace(blockedID)
	if blockedID == "" {
		return NewValidationError("user_id is required")
	}
	if blockedID == blockerID {
		return NewValidationError("you cannot block yourself")
	}
	return s.repo.BlockUser(ctx, blockerID, blockedID)
}

func (s *Service) UnblockUser(ctx context.Context, blockerID, blockedID string) error {
	blockedID = strings.TrimSpace(blockedID)
	if blockedID == "" {
		return NewValidationError("user_id is required")
	}
	return s.repo.UnblockUser(ctx, blockerID, blockedID)
}

// ListUserIDsBlocking returns the users who blocked userID. Content authored by
// userID is not pushed or broadcast to them.
func (s *Service) ListUserIDsBlocking(ctx context.Context, userID string) ([]string, error) {
	return s.repo.ListUserIDsBlocking(ctx, userID)
}

func (s *Service) ListUserBlocks(ctx context.Context, blockerID string) ([]domain.UserBlock, error) {
	return s.repo.ListUserBlocks(ctx, blockerID)
}
