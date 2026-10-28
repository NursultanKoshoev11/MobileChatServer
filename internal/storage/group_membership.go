package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) LeaveGroup(ctx context.Context, groupID, userID string) error {
	var role domain.GroupRole
	if err := r.db.QueryRow(ctx, `SELECT role FROM group_members WHERE group_id=$1 AND user_id=$2 FOR UPDATE`, groupID, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return fmt.Errorf("lock group membership before leave: %w", err)
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
