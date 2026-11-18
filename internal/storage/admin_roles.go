package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

// SyncAdminPhoneAllowlist makes the allowlist match the configured phones in
// one transaction: listed phones are granted (super_admin wins over
// platform_admin), every other entry is disabled, and users whose phone is no
// longer allowlisted lose their admin role immediately instead of at their
// next login.
func (r *Repository) SyncAdminPhoneAllowlist(ctx context.Context, superAdminPhones, platformAdminPhones []string) error {
	desired := map[string]domain.UserRole{}
	for _, phone := range platformAdminPhones {
		if phone = domain.NormalizeMobileInput(phone); phone != "" {
			desired[phone] = domain.UserRolePlatformAdmin
		}
	}
	for _, phone := range superAdminPhones {
		if phone = domain.NormalizeMobileInput(phone); phone != "" {
			desired[phone] = domain.UserRoleSuperAdmin
		}
	}
	phones := make([]string, 0, len(desired))
	for phone := range desired {
		phones = append(phones, phone)
	}
	sort.Strings(phones)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin admin allowlist sync: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	for _, phone := range phones {
		if _, err := tx.Exec(ctx, `
			INSERT INTO admin_phone_allowlist (phone, role, enabled)
			VALUES ($1, $2, true)
			ON CONFLICT (phone)
			DO UPDATE SET role = EXCLUDED.role, enabled = true, updated_at = now()`, phone, desired[phone]); err != nil {
			return fmt.Errorf("sync admin phone allowlist: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE admin_phone_allowlist
		SET enabled = false, updated_at = now()
		WHERE enabled AND NOT (phone = ANY($1::text[]))`, phones); err != nil {
		return fmt.Errorf("disable removed admin phones: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users u
		SET role = 'user', updated_at = now()
		WHERE u.role IN ('platform_admin', 'super_admin')
		  AND NOT EXISTS (
			SELECT 1 FROM admin_phone_allowlist a
			WHERE a.enabled AND (a.phone = u.phone OR a.phone = u.phone_number)
		  )`); err != nil {
		return fmt.Errorf("revoke removed admin roles: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users u
		SET role = a.role, updated_at = now()
		FROM admin_phone_allowlist a
		WHERE a.enabled AND (a.phone = u.phone OR a.phone = u.phone_number) AND u.role IS DISTINCT FROM a.role`); err != nil {
		return fmt.Errorf("apply allowlisted admin roles: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit admin allowlist sync: %w", err)
	}
	return nil
}
