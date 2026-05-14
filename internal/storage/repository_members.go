package storage

import (
	"context"
	"fmt"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (r *Repository) ListGroupMembers(ctx context.Context, groupID, requesterID string) ([]domain.GroupMember, error) {
	isMember, err := r.IsGroupMember(ctx, groupID, requesterID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrForbidden
	}

	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.display_name, gm.role, gm.joined_at
		FROM group_members gm
		JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1
		ORDER BY
			CASE gm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,
			u.display_name ASC`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}
	defer rows.Close()

	members := make([]domain.GroupMember, 0)
	for rows.Next() {
		var member domain.GroupMember
		if err := rows.Scan(&member.UserID, &member.DisplayName, &member.Role, &member.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan group member: %w", err)
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *Repository) SetMemberRole(ctx context.Context, groupID, actorID, targetUserID string, role domain.GroupRole) error {
	actorRole, err := r.GetMemberRole(ctx, groupID, actorID)
	if err != nil {
		return err
	}
	if actorRole != domain.RoleOwner {
		return ErrForbidden
	}
	if role != domain.RoleAdmin && role != domain.RoleMember {
		return ErrForbidden
	}
	result, err := r.db.Exec(ctx, `
		UPDATE group_members
		SET role = $1
		WHERE group_id = $2 AND user_id = $3 AND role <> 'owner'`, role, groupID, targetUserID)
	if err != nil {
		return fmt.Errorf("set member role: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) RemoveGroupMember(ctx context.Context, groupID, actorID, targetUserID string) error {
	actorRole, err := r.GetMemberRole(ctx, groupID, actorID)
	if err != nil {
		return err
	}
	if actorRole != domain.RoleOwner && actorRole != domain.RoleAdmin && actorID != targetUserID {
		return ErrForbidden
	}
	if actorID != targetUserID {
		targetRole, err := r.GetMemberRole(ctx, groupID, targetUserID)
		if err != nil {
			return err
		}
		if targetRole == domain.RoleOwner || (actorRole == domain.RoleAdmin && targetRole == domain.RoleAdmin) {
			return ErrForbidden
		}
	}
	result, err := r.db.Exec(ctx, `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2 AND role <> 'owner'`, groupID, targetUserID)
	if err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
