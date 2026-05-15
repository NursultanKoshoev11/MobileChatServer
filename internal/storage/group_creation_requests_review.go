package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) RejectGroupCreationRequest(ctx context.Context, requestID, adminID, comment string) (domain.GroupCreationRequest, error) {
	return r.updateGroupCreationRequestReview(ctx, requestID, adminID, domain.GroupCreationRequestRejected, comment)
}

func (r *Repository) MarkGroupCreationRequestNeedsMoreInfo(ctx context.Context, requestID, adminID, comment string) (domain.GroupCreationRequest, error) {
	return r.updateGroupCreationRequestReview(ctx, requestID, adminID, domain.GroupCreationRequestNeedsMoreInfo, comment)
}

func (r *Repository) updateGroupCreationRequestReview(ctx context.Context, requestID, adminID string, status domain.GroupCreationRequestStatus, comment string) (domain.GroupCreationRequest, error) {
	result, err := r.db.Exec(ctx, `UPDATE group_creation_requests SET status=$3, admin_comment=$4, reviewed_by=$2, reviewed_at=now(), updated_at=now() WHERE id=$1 AND status IN ('pending','needs_more_info')`, requestID, adminID, status, comment)
	if err != nil {
		return domain.GroupCreationRequest{}, fmt.Errorf("update group creation request review: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.GroupCreationRequest{}, ErrNotFound
	}
	return r.GetGroupCreationRequestByID(ctx, requestID)
}

func (r *Repository) ApproveGroupCreationRequest(ctx context.Context, requestID, adminID, comment string, group domain.Group) (domain.GroupCreationRequest, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.GroupCreationRequest{}, fmt.Errorf("begin approve group request: %w", err)
	}
	defer tx.Rollback(ctx)

	var status domain.GroupCreationRequestStatus
	var requesterID string
	if err := tx.QueryRow(ctx, `SELECT status, requester_id FROM group_creation_requests WHERE id=$1 FOR UPDATE`, requestID).Scan(&status, &requesterID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.GroupCreationRequest{}, ErrNotFound
		}
		return domain.GroupCreationRequest{}, fmt.Errorf("lock group creation request: %w", err)
	}
	if status != domain.GroupCreationRequestPending && status != domain.GroupCreationRequestNeedsMoreInfo {
		return domain.GroupCreationRequest{}, ErrForbidden
	}
	if group.OwnerID == "" {
		group.OwnerID = requesterID
	}
	if group.Visibility == "" {
		group.Visibility = domain.VisibilityPublic
	}
	_, err = tx.Exec(ctx, `INSERT INTO groups (id, title, description, visibility, owner_id, invite_code, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,now(),now())`, group.ID, group.Title, group.Description, group.Visibility, group.OwnerID, nullableInviteCode(group))
	if err != nil {
		return domain.GroupCreationRequest{}, fmt.Errorf("create approved group: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO group_members (group_id, user_id, role) VALUES ($1,$2,'owner') ON CONFLICT (group_id,user_id) DO UPDATE SET role='owner'`, group.ID, group.OwnerID)
	if err != nil {
		return domain.GroupCreationRequest{}, fmt.Errorf("create approved group owner: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE group_creation_requests SET status='approved', admin_comment=$3, reviewed_by=$2, reviewed_at=now(), created_group_id=$4, updated_at=now() WHERE id=$1`, requestID, adminID, comment, group.ID)
	if err != nil {
		return domain.GroupCreationRequest{}, fmt.Errorf("update approved group request: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.GroupCreationRequest{}, fmt.Errorf("commit approve group request: %w", err)
	}
	return r.GetGroupCreationRequestByID(ctx, requestID)
}
