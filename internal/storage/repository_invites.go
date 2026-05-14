package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateInviteRequest(ctx context.Context, invite domain.InviteRequest) (domain.InviteRequest, error) {
	query := `
		INSERT INTO group_invite_requests (id, group_id, inviter_id, target_user_id, status, created_at)
		VALUES ($1, $2, $3, $4, 'pending', now())
		ON CONFLICT (group_id, target_user_id, status) DO UPDATE SET created_at = EXCLUDED.created_at
		RETURNING created_at`
	if err := r.db.QueryRow(ctx, query, invite.ID, invite.GroupID, invite.InviterID, invite.TargetUserID).Scan(&invite.CreatedAt); err != nil {
		return domain.InviteRequest{}, fmt.Errorf("create invite request: %w", err)
	}
	invite.Status = "pending"
	return invite, nil
}

func (r *Repository) ListPendingInvites(ctx context.Context, userID string) ([]domain.InviteRequest, error) {
	query := `
		SELECT i.id, i.group_id, g.title, i.inviter_id, u.display_name, i.target_user_id, i.status, i.created_at, i.responded_at
		FROM group_invite_requests i
		JOIN groups g ON g.id = i.group_id
		JOIN users u ON u.id = i.inviter_id
		WHERE i.target_user_id = $1 AND i.status = 'pending'
		ORDER BY i.created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list pending invites: %w", err)
	}
	defer rows.Close()

	invites := make([]domain.InviteRequest, 0)
	for rows.Next() {
		var invite domain.InviteRequest
		if err := rows.Scan(&invite.ID, &invite.GroupID, &invite.GroupTitle, &invite.InviterID, &invite.InviterName, &invite.TargetUserID, &invite.Status, &invite.CreatedAt, &invite.RespondedAt); err != nil {
			return nil, fmt.Errorf("scan invite request: %w", err)
		}
		invites = append(invites, invite)
	}
	return invites, rows.Err()
}

func (r *Repository) AcceptInviteRequest(ctx context.Context, inviteID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin accept invite: %w", err)
	}
	defer tx.Rollback(ctx)

	var groupID string
	query := `
		UPDATE group_invite_requests
		SET status = 'accepted', responded_at = now()
		WHERE id = $1 AND target_user_id = $2 AND status = 'pending'
		RETURNING group_id`
	if err := tx.QueryRow(ctx, query, inviteID, userID).Scan(&groupID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("accept invite request: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'member') ON CONFLICT (group_id, user_id) DO NOTHING`, groupID, userID); err != nil {
		return fmt.Errorf("add invited member: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit accept invite: %w", err)
	}
	return nil
}

func (r *Repository) DeclineInviteRequest(ctx context.Context, inviteID, userID string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE group_invite_requests
		SET status = 'declined', responded_at = now()
		WHERE id = $1 AND target_user_id = $2 AND status = 'pending'`, inviteID, userID)
	if err != nil {
		return fmt.Errorf("decline invite request: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CanInviteToGroup(ctx context.Context, groupID, inviterID, targetUserID string) error {
	role, err := r.GetMemberRole(ctx, groupID, inviterID)
	if err != nil {
		return err
	}
	if role != domain.RoleOwner && role != domain.RoleAdmin {
		return ErrForbidden
	}
	if _, err := r.GetUserByID(ctx, targetUserID); err != nil {
		return err
	}
	return nil
}

var _ = time.Time{}
