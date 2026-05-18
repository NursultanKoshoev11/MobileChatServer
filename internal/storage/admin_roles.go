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

func (r *Repository) SyncAdminPhoneAllowlist(ctx context.Context, superAdminPhones, platformAdminPhones []string) error {
	for _, phone := range superAdminPhones {
		if err := r.upsertAdminPhone(ctx, phone, domain.UserRoleSuperAdmin); err != nil {
			return err
		}
	}
	for _, phone := range platformAdminPhones {
		if err := r.upsertAdminPhone(ctx, phone, domain.UserRolePlatformAdmin); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) upsertAdminPhone(ctx context.Context, phone string, role domain.UserRole) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO admin_phone_allowlist (phone, role, enabled)
		VALUES ($1, $2, true)
		ON CONFLICT (phone)
		DO UPDATE SET role = EXCLUDED.role, enabled = true`, phone, role); err != nil {
		return fmt.Errorf("sync admin phone allowlist: %w", err)
	}
	return nil
}
