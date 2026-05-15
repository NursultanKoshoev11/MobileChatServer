package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ResolveUserRoleByPhone(ctx context.Context, phone string) (domain.UserRole, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return domain.UserRoleUser, nil
	}
	var role domain.UserRole
	err := r.db.QueryRow(ctx, `SELECT role FROM admin_phone_allowlist WHERE phone = $1 AND enabled = true`, phone).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.UserRoleUser, nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve user role by phone: %w", err)
	}
	return role, nil
}

func (r *Repository) UpsertUserRoleFromAllowlist(ctx context.Context, userID, phone string) error {
	role, err := r.ResolveUserRoleByPhone(ctx, phone)
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(ctx, `UPDATE users SET phone = COALESCE(NULLIF($2, ''), phone), role = $3, updated_at = now() WHERE id = $1`, userID, strings.TrimSpace(phone), role); err != nil {
		return fmt.Errorf("upsert user role from allowlist: %w", err)
	}
	return nil
}
