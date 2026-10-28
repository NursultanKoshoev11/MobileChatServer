package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (r *Repository) ListGroupMembersPage(ctx context.Context, groupID, requesterID string, limit, offset int) ([]domain.GroupMember, error) {
	isMember, err := r.IsGroupMember(ctx, groupID, requesterID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrForbidden
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.display_name,
		       CASE WHEN requester.role IN ('owner', 'admin')
	            THEN COALESCE(NULLIF(u.phone, ''), u.phone_number, '')
	            ELSE ''
	       END AS phone,
	       gm.role, gm.joined_at
		FROM group_members gm
		JOIN group_members requester ON requester.group_id = gm.group_id AND requester.user_id = $4
		JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1
		ORDER BY
			CASE gm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,
			u.display_name ASC
		LIMIT $2 OFFSET $3`, groupID, limit, offset, requesterID)
	if err != nil {
		return nil, fmt.Errorf("list group members page: %w", err)
	}
	defer rows.Close()
	members := make([]domain.GroupMember, 0)
	for rows.Next() {
		var member domain.GroupMember
		if err := rows.Scan(&member.UserID, &member.DisplayName, &member.Phone, &member.Role, &member.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan group member page: %w", err)
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *Repository) SearchUsers(ctx context.Context, queryText string, limit int) ([]domain.UserSearchResult, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	escapedQuery := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(strings.ToLower(queryText))
	like := "%" + escapedQuery + "%"
	rows, err := r.db.Query(ctx, `
		SELECT id, display_name
		FROM users
		WHERE lower(display_name) LIKE $1 ESCAPE E'\\'
		ORDER BY display_name ASC
		LIMIT $2`, like, limit)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()
	users := make([]domain.UserSearchResult, 0)
	for rows.Next() {
		var user domain.UserSearchResult
		if err := rows.Scan(&user.ID, &user.DisplayName); err != nil {
			return nil, fmt.Errorf("scan user search: %w", err)
		}
		users = append(users, user)
	}
	return users, rows.Err()
}
