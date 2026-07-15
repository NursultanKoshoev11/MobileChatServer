package storage

import (
	"context"
	"fmt"
	"strings"
)

func (r *Repository) DeleteGroupAsPlatformAdmin(ctx context.Context, groupID string) error {
	result, err := r.db.Exec(ctx, `DELETE FROM groups WHERE id = $1`, strings.TrimSpace(groupID))
	if err != nil {
		return fmt.Errorf("delete group as platform admin: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
