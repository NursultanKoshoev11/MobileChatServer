package storage

import (
	"context"
	"fmt"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (r *Repository) LeaveGroup(ctx context.Context, groupID, userID string) error {
	role, err := r.GetMemberRole(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if role == domain.RoleOwner {
		return ErrForbidden
	}
	result, err := r.db.Exec(ctx, `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID)
	if err != nil {
		return fmt.Errorf("leave group: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
