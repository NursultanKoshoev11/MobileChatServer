package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) DeleteAccount(ctx context.Context, userID, auditEventID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin account deletion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var lockedUserID string
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&lockedUserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock account before deletion: %w", err)
	}

	rows, err := tx.Query(ctx, `SELECT id FROM groups WHERE owner_id = $1 ORDER BY id FOR UPDATE`, userID)
	if err != nil {
		return fmt.Errorf("list owned groups before account deletion: %w", err)
	}
	ownedGroupIDs := make([]string, 0)
	for rows.Next() {
		var groupID string
		if err := rows.Scan(&groupID); err != nil {
			rows.Close()
			return fmt.Errorf("scan owned group before account deletion: %w", err)
		}
		ownedGroupIDs = append(ownedGroupIDs, groupID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read owned groups before account deletion: %w", err)
	}
	rows.Close()

	for _, groupID := range ownedGroupIDs {
		var nextOwnerID string
		err := tx.QueryRow(ctx, `
			SELECT user_id
			FROM group_members
			WHERE group_id = $1 AND user_id <> $2
			ORDER BY CASE role WHEN 'admin' THEN 0 ELSE 1 END, joined_at ASC, user_id ASC
			LIMIT 1
			FOR UPDATE`, groupID, userID).Scan(&nextOwnerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrGroupOwnershipTransferRequired
		}
		if err != nil {
			return fmt.Errorf("select successor for owned group %s: %w", groupID, err)
		}
		if _, err := tx.Exec(ctx, `UPDATE group_members SET role = 'member' WHERE group_id = $1 AND user_id = $2`, groupID, userID); err != nil {
			return fmt.Errorf("release group ownership for %s: %w", groupID, err)
		}
		if _, err := tx.Exec(ctx, `UPDATE group_members SET role = 'owner' WHERE group_id = $1 AND user_id = $2`, groupID, nextOwnerID); err != nil {
			return fmt.Errorf("assign new group owner for %s: %w", groupID, err)
		}
		if _, err := tx.Exec(ctx, `UPDATE groups SET owner_id = $2, updated_at = now() WHERE id = $1`, groupID, nextOwnerID); err != nil {
			return fmt.Errorf("transfer group owner for %s: %w", groupID, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_events (id, actor_user_id, event_type, resource_type, metadata)
		VALUES ($1, $2, 'account_deleted', 'account', '{"public_content":"anonymized"}'::jsonb)`, auditEventID, userID); err != nil {
		return fmt.Errorf("record account deletion: %w", err)
	}

	result, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user account: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit account deletion: %w", err)
	}
	return nil
}
