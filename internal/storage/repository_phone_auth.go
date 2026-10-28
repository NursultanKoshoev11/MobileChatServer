package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

type PhoneCodeRecord struct {
	ID          string
	PhoneNumber string
	CodeHash    string
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
	Attempts    int
	CreatedAt   time.Time
}

func (r *Repository) CreatePhoneCodeIfWithinLimit(ctx context.Context, id, phoneNumber, codeHash string, expiresAt, since, nextAllowedAt time.Time, maxRequests int) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin phone code transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	// Serialize issuance checks for a phone number so concurrent requests cannot
	// all pass the count and minimum-gap checks before any of them inserts a row.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, phoneNumber); err != nil {
		return false, fmt.Errorf("lock phone code issuance: %w", err)
	}

	var count int
	var latest sql.NullTime
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE created_at >= $2)::int, MAX(created_at)
		FROM phone_verification_codes
		WHERE phone_number = $1
	`, phoneNumber, since).Scan(&count, &latest); err != nil {
		return false, fmt.Errorf("check phone code issuance limit: %w", err)
	}
	if count >= maxRequests || latest.Valid && latest.Time.After(nextAllowedAt) {
		return false, nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO phone_verification_codes (id, phone_number, code_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, id, phoneNumber, codeHash, expiresAt); err != nil {
		return false, fmt.Errorf("create phone code: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit phone code transaction: %w", err)
	}
	return true, nil
}

func (r *Repository) GetLatestPhoneCode(ctx context.Context, phoneNumber string) (PhoneCodeRecord, error) {
	query := `
		SELECT id, phone_number, code_hash, expires_at, consumed_at, attempts, created_at
		FROM phone_verification_codes
		WHERE phone_number = $1
		ORDER BY created_at DESC
		LIMIT 1`
	var code PhoneCodeRecord
	err := r.db.QueryRow(ctx, query, phoneNumber).Scan(&code.ID, &code.PhoneNumber, &code.CodeHash, &code.ExpiresAt, &code.ConsumedAt, &code.Attempts, &code.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PhoneCodeRecord{}, ErrNotFound
	}
	if err != nil {
		return PhoneCodeRecord{}, fmt.Errorf("get latest phone code: %w", err)
	}
	return code, nil
}

func (r *Repository) IncrementPhoneCodeAttempts(ctx context.Context, codeID string, maxAttempts int) (bool, error) {
	result, err := r.db.Exec(ctx, `
		UPDATE phone_verification_codes
		SET attempts = attempts + 1
		WHERE id = $1 AND consumed_at IS NULL AND expires_at > now() AND attempts < $2
	`, codeID, maxAttempts)
	if err != nil {
		return false, fmt.Errorf("increment phone code attempts: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

func (r *Repository) ConsumePhoneCode(ctx context.Context, codeID string, maxAttempts int) (bool, error) {
	result, err := r.db.Exec(ctx, `
		UPDATE phone_verification_codes
		SET consumed_at = now()
		WHERE id = $1 AND consumed_at IS NULL AND expires_at > now() AND attempts < $2
	`, codeID, maxAttempts)
	if err != nil {
		return false, fmt.Errorf("consume phone code: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

func (r *Repository) GetPhoneUserByMobile(ctx context.Context, mobile string) (domain.PhoneAuthUser, error) {
	query := `SELECT id, COALESCE(phone_number, ''), display_name, COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at FROM users WHERE phone_number = $1`
	var user domain.PhoneAuthUser
	err := r.db.QueryRow(ctx, query, mobile).Scan(&user.ID, &user.Mobile, &user.DisplayName, &user.Role, &user.AvatarData, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PhoneAuthUser{}, ErrNotFound
	}
	if err != nil {
		return domain.PhoneAuthUser{}, fmt.Errorf("get user by mobile: %w", err)
	}
	return user, nil
}

func (r *Repository) CreatePhoneUser(ctx context.Context, user domain.PhoneAuthUser) (domain.PhoneAuthUser, error) {
	query := `
		INSERT INTO users (id, phone_number, phone_verified_at, display_name, created_at, updated_at)
		VALUES ($1, $2, now(), $3, now(), now())
		RETURNING created_at`
	if err := r.db.QueryRow(ctx, query, user.ID, user.Mobile, user.DisplayName).Scan(&user.CreatedAt); err != nil {
		return domain.PhoneAuthUser{}, fmt.Errorf("create phone user: %w", err)
	}
	return user, nil
}

func (r *Repository) MarkPhoneVerified(ctx context.Context, userID string) error {
	result, err := r.db.Exec(ctx, `UPDATE users SET phone_verified_at = now(), updated_at = now() WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("mark phone verified: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("mark phone verified: %w", ErrNotFound)
	}
	return nil
}

func (r *Repository) GetAuthUserByID(ctx context.Context, userID string) (domain.User, error) {
	query := `SELECT id, COALESCE(email, ''), COALESCE(NULLIF(phone, ''), phone_number, '') AS phone, display_name, COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at FROM users WHERE id = $1`
	var id string
	var email string
	var mobile string
	var displayName string
	var role domain.UserRole
	var avatarData string
	var createdAt time.Time
	err := r.db.QueryRow(ctx, query, userID).Scan(&id, &email, &mobile, &displayName, &role, &avatarData, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get auth user by id: %w", err)
	}
	return domain.User{ID: id, Email: email, Phone: mobile, DisplayName: displayName, Role: role, AvatarData: avatarData, CreatedAt: createdAt}, nil
}

func NormalizePhoneDigits(raw string) string {
	return strings.TrimSpace(raw)
}
