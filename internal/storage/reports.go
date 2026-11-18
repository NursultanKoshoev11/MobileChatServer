package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

const maxContentReportsListed = 200

// ReportTarget describes where a reported item lives and who authored it.
type ReportTarget struct {
	GroupID  string
	AuthorID string
	// RequestID is the parent public request of a reported comment.
	RequestID string
}

// ResolveReportTarget checks that the target exists and that viewerID may see
// it: content targets require membership of the owning group, user targets
// only require the user to exist. Invisible and missing targets both return
// ErrNotFound so reports cannot be used to probe other groups.
func (r *Repository) ResolveReportTarget(ctx context.Context, targetType domain.ReportTargetType, targetID, viewerID string) (ReportTarget, error) {
	var target ReportTarget
	var err error
	switch targetType {
	case domain.ReportTargetPublicRequest:
		err = r.db.QueryRow(ctx, `
			SELECT pr.group_id, COALESCE(pr.author_id, '')
			FROM public_requests pr
			JOIN group_members gm ON gm.group_id = pr.group_id AND gm.user_id = $2
			WHERE pr.id = $1 AND pr.hidden_at IS NULL`, targetID, viewerID).Scan(&target.GroupID, &target.AuthorID)
	case domain.ReportTargetComment:
		err = r.db.QueryRow(ctx, `
			SELECT pr.group_id, COALESCE(c.author_id, ''), c.request_id
			FROM public_request_comments c
			JOIN public_requests pr ON pr.id = c.request_id
			JOIN group_members gm ON gm.group_id = pr.group_id AND gm.user_id = $2
			WHERE c.id = $1 AND c.deleted_at IS NULL AND pr.hidden_at IS NULL`, targetID, viewerID).Scan(&target.GroupID, &target.AuthorID, &target.RequestID)
	case domain.ReportTargetMessage:
		err = r.db.QueryRow(ctx, `
			SELECT m.group_id, COALESCE(m.sender_id, '')
			FROM messages m
			JOIN group_members gm ON gm.group_id = m.group_id AND gm.user_id = $2
			WHERE m.id = $1 AND m.deleted_at IS NULL`, targetID, viewerID).Scan(&target.GroupID, &target.AuthorID)
	case domain.ReportTargetUser:
		err = r.db.QueryRow(ctx, `SELECT id FROM users WHERE id = $1`, targetID).Scan(&target.AuthorID)
	default:
		return ReportTarget{}, fmt.Errorf("unsupported report target type %q", targetType)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ReportTarget{}, ErrNotFound
	}
	if err != nil {
		return ReportTarget{}, fmt.Errorf("resolve report target: %w", err)
	}
	return target, nil
}

// CreateContentReport inserts an open report. When the reporter already has an
// open report on the same target, the existing report is returned with
// created=false.
func (r *Repository) CreateContentReport(ctx context.Context, report domain.ContentReport) (domain.ContentReport, bool, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO content_reports (id, reporter_id, target_type, target_id, group_id, target_user_id, reason, details, status, created_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8, 'open', now())
		ON CONFLICT (reporter_id, target_type, target_id) WHERE status = 'open' DO NOTHING
		RETURNING status, created_at`,
		report.ID, report.ReporterID, report.TargetType, report.TargetID, report.GroupID, report.TargetUserID, report.Reason, report.Details,
	).Scan(&report.Status, &report.CreatedAt)
	if err == nil {
		return report, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.ContentReport{}, false, fmt.Errorf("create content report: %w", err)
	}
	existing, err := r.scanContentReports(ctx, contentReportSelect+`
		WHERE cr.reporter_id = $1 AND cr.target_type = $2 AND cr.target_id = $3 AND cr.status = 'open'
		LIMIT 1`, report.ReporterID, report.TargetType, report.TargetID)
	if err != nil {
		return domain.ContentReport{}, false, err
	}
	if len(existing) == 0 {
		return domain.ContentReport{}, false, fmt.Errorf("create content report: conflicting open report not found")
	}
	return existing[0], false, nil
}

const contentReportSelect = `
	SELECT cr.id, cr.target_type, cr.target_id, COALESCE(cr.group_id, ''), COALESCE(cr.target_user_id, ''),
	       cr.reason, cr.details, cr.status, COALESCE(cr.resolution, ''), cr.reporter_id,
	       COALESCE(u.display_name, ''), cr.created_at, cr.resolved_at
	FROM content_reports cr
	LEFT JOIN users u ON u.id = cr.reporter_id`

func (r *Repository) scanContentReports(ctx context.Context, query string, args ...any) ([]domain.ContentReport, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list content reports: %w", err)
	}
	defer rows.Close()
	reports := make([]domain.ContentReport, 0)
	for rows.Next() {
		var report domain.ContentReport
		if err := rows.Scan(
			&report.ID,
			&report.TargetType,
			&report.TargetID,
			&report.GroupID,
			&report.TargetUserID,
			&report.Reason,
			&report.Details,
			&report.Status,
			&report.Resolution,
			&report.ReporterID,
			&report.ReporterDisplayName,
			&report.CreatedAt,
			&report.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan content report: %w", err)
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (r *Repository) GetContentReport(ctx context.Context, reportID string) (domain.ContentReport, error) {
	reports, err := r.scanContentReports(ctx, contentReportSelect+` WHERE cr.id = $1`, reportID)
	if err != nil {
		return domain.ContentReport{}, err
	}
	if len(reports) == 0 {
		return domain.ContentReport{}, ErrNotFound
	}
	return reports[0], nil
}

// ListGroupContentReports lists reports about content in one group.
func (r *Repository) ListGroupContentReports(ctx context.Context, groupID string, status domain.ReportStatus) ([]domain.ContentReport, error) {
	return r.scanContentReports(ctx, contentReportSelect+`
		WHERE cr.group_id = $1 AND cr.status = $2
		ORDER BY cr.created_at DESC, cr.id DESC
		LIMIT $3`, groupID, status, maxContentReportsListed)
}

// ListAllContentReports lists reports across the platform, including reports
// about users.
func (r *Repository) ListAllContentReports(ctx context.Context, status domain.ReportStatus) ([]domain.ContentReport, error) {
	return r.scanContentReports(ctx, contentReportSelect+`
		WHERE cr.status = $1
		ORDER BY cr.created_at DESC, cr.id DESC
		LIMIT $2`, status, maxContentReportsListed)
}

// ResolveContentReports resolves every open report on the same target as
// reportID, so moderators do not have to handle duplicates one by one.
func (r *Repository) ResolveContentReports(ctx context.Context, report domain.ContentReport, resolution domain.ReportResolution, resolverID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE content_reports
		SET status = 'resolved', resolution = $3, resolved_by = $4, resolved_at = now()
		WHERE status = 'open' AND target_type = $1 AND target_id = $2`, report.TargetType, report.TargetID, resolution, resolverID)
	if err != nil {
		return fmt.Errorf("resolve content reports: %w", err)
	}
	// The report itself may already have been resolved by a concurrent
	// moderator; only touch it when it is still open (handled above).
	return nil
}

// RemovedContent describes content taken down while resolving a report.
type RemovedContent struct {
	TargetType domain.ReportTargetType
	GroupID    string
	RequestID  string
	CommentID  string
	Message    *domain.Message
}

// RemoveReportedContent takes down reported content using the same storage
// effects as the regular moderation endpoints (hide request, soft-delete
// comment, delete message). Authorization is the caller's responsibility.
// Content that is already gone is not an error.
func (r *Repository) RemoveReportedContent(ctx context.Context, targetType domain.ReportTargetType, targetID, moderatorID string) (RemovedContent, error) {
	removed := RemovedContent{TargetType: targetType}
	switch targetType {
	case domain.ReportTargetPublicRequest:
		if err := r.db.QueryRow(ctx, `
			UPDATE public_requests SET hidden_at = now(), hidden_by = $1, updated_at = now()
			WHERE id = $2 AND hidden_at IS NULL
			RETURNING group_id`, moderatorID, targetID).Scan(&removed.GroupID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return RemovedContent{}, fmt.Errorf("hide reported public request: %w", err)
		}
		removed.RequestID = targetID
	case domain.ReportTargetComment:
		err := r.db.QueryRow(ctx, `
			UPDATE public_request_comments c SET deleted_at = now()
			FROM public_requests pr
			WHERE c.id = $1 AND c.deleted_at IS NULL AND pr.id = c.request_id
			RETURNING pr.group_id, c.request_id`, targetID).Scan(&removed.GroupID, &removed.RequestID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return RemovedContent{}, fmt.Errorf("delete reported comment: %w", err)
		}
		removed.CommentID = targetID
	case domain.ReportTargetMessage:
		var message domain.Message
		err := r.db.QueryRow(ctx, `
			WITH updated AS (
				UPDATE messages
				SET deleted_at = now(), text = ''
				WHERE id = $1 AND deleted_at IS NULL
				RETURNING id, group_id, sender_id, text, created_at, edited_at, deleted_at
			)
			SELECT m.id, m.group_id, m.sender_id, COALESCE(u.display_name, ''), m.text, m.created_at, m.edited_at, m.deleted_at
			FROM updated m
			LEFT JOIN users u ON u.id = m.sender_id`, targetID).Scan(
			&message.ID, &message.GroupID, &message.SenderID, &message.SenderName, &message.Text, &message.CreatedAt, &message.EditedAt, &message.DeletedAt,
		)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return RemovedContent{}, fmt.Errorf("delete reported message: %w", err)
		}
		if err == nil {
			removed.GroupID = message.GroupID
			removed.Message = &message
		}
	case domain.ReportTargetUser:
		// Account-level enforcement is handled by platform operators; the
		// resolution is recorded only.
	default:
		return RemovedContent{}, fmt.Errorf("unsupported report target type %q", targetType)
	}
	return removed, nil
}

// IsGroupModeratorMember reports whether userID is an owner or admin of groupID.
func (r *Repository) IsGroupModeratorMember(ctx context.Context, groupID, userID string) (bool, error) {
	role, err := r.GetMemberRole(ctx, groupID, userID)
	if errors.Is(err, ErrForbidden) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return role == domain.RoleOwner || role == domain.RoleAdmin, nil
}

func (r *Repository) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	result, err := r.db.Exec(ctx, `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		SELECT $1, id, now() FROM users WHERE id = $2
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING`, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("block user: %w", err)
	}
	if result.RowsAffected() == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, blockedID).Scan(&exists); err != nil {
			return fmt.Errorf("check blocked user: %w", err)
		}
		if !exists {
			return ErrNotFound
		}
	}
	return nil
}

func (r *Repository) UnblockUser(ctx context.Context, blockerID, blockedID string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2`, blockerID, blockedID); err != nil {
		return fmt.Errorf("unblock user: %w", err)
	}
	return nil
}

func (r *Repository) ListUserBlocks(ctx context.Context, blockerID string) ([]domain.UserBlock, error) {
	rows, err := r.db.Query(ctx, `
		SELECT b.blocked_id, COALESCE(u.display_name, ''), b.created_at
		FROM user_blocks b
		JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC, b.blocked_id`, strings.TrimSpace(blockerID))
	if err != nil {
		return nil, fmt.Errorf("list user blocks: %w", err)
	}
	defer rows.Close()
	blocks := make([]domain.UserBlock, 0)
	for rows.Next() {
		var block domain.UserBlock
		if err := rows.Scan(&block.UserID, &block.DisplayName, &block.BlockedAt); err != nil {
			return nil, fmt.Errorf("scan user block: %w", err)
		}
		blocks = append(blocks, block)
	}
	return blocks, rows.Err()
}
