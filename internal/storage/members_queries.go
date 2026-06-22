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
		SELECT u.id, u.display_name, COALESCE(NULLIF(u.phone, ''), u.phone_number, '') AS phone, gm.role, gm.joined_at
		FROM group_members gm
		JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1
		ORDER BY
			CASE gm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,
			u.display_name ASC
		LIMIT $2 OFFSET $3`, groupID, limit, offset)
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

func (r *Repository) SearchUsers(ctx context.Context, queryText string, limit int) ([]domain.User, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	like := "%" + strings.ToLower(queryText) + "%"
	rows, err := r.db.Query(ctx, `
		SELECT id, COALESCE(email, ''), COALESCE(NULLIF(phone, ''), phone_number, '') AS phone, display_name, COALESCE(role, 'user'), created_at
		FROM users
		WHERE lower(display_name) LIKE $1 OR phone = $2 OR phone_number = $2
		ORDER BY display_name ASC
		LIMIT $3`, like, queryText, limit)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()
	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Email, &user.Phone, &user.DisplayName, &user.Role, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user search: %w", err)
		}
		users = append(users, user)
	}
	return users, rows.Err()
}
