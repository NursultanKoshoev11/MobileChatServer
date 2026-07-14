package storage

import (
	"context"
	"fmt"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (r *Repository) UpdateGroupAvatar(ctx context.Context, actorID, groupID, avatarData string) (domain.Group, error) {
	role, err := r.GetMemberRole(ctx, groupID, actorID)
	if err != nil {
		return domain.Group{}, err
	}
	if role != domain.RoleOwner && role != domain.RoleAdmin {
		return domain.Group{}, ErrForbidden
	}

	command, err := r.db.Exec(ctx, `
		UPDATE groups
		SET avatar_data = $2, updated_at = now()
		WHERE id = $1`, groupID, avatarData)
	if err != nil {
		return domain.Group{}, fmt.Errorf("update group avatar: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.Group{}, ErrNotFound
	}

	var group domain.Group
	var currentRole domain.GroupRole
	err = r.db.QueryRow(ctx, `
		SELECT g.id, g.title, g.description, g.visibility, g.owner_id,
		       COALESCE(g.avatar_data, ''), COALESCE(g.invite_code, ''), g.created_at,
		       (SELECT COUNT(*)::int FROM group_members WHERE group_id = g.id), gm.role
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id AND gm.user_id = $2
		WHERE g.id = $1`, groupID, actorID).Scan(
		&group.ID,
		&group.Title,
		&group.Description,
		&group.Visibility,
		&group.OwnerID,
		&group.AvatarData,
		&group.InviteCode,
		&group.CreatedAt,
		&group.MemberCount,
		&currentRole,
	)
	if err != nil {
		return domain.Group{}, fmt.Errorf("load updated group avatar: %w", err)
	}
	group.MyRole = &currentRole
	return group, nil
}
