package storage

import (
	"context"
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

func (r *Repository) CreatePhoneCode(ctx context.Context, id, phoneNumber, codeHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO phone_verification_codes (id, phone_number, code_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, id, phoneNumber, codeHash, expiresAt)
	if err != nil {
		return fmt.Errorf("create phone code: %w", err)
	}
	return nil
}

func (r *Repository) CountPhoneCodesSince(ctx context.Context, phoneNumber string, since time.Time) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM phone_verification_codes
		WHERE phone_number = $1 AND created_at >= $2
	`, phoneNumber, since).Scan(&count); err != nil {
		return 0, fmt.Errorf("count phone codes: %w", err)
	}
	return count, nil
}

func (r *Repository) LatestPhoneCodeCreatedAt(ctx context.Context, phoneNumber string) (time.Time, error) {
	var createdAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT created_at
		FROM phone_verification_codes
		WHERE phone_number = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, phoneNumber).Scan(&createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrNotFound
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("latest phone code created at: %w", err)
	}
	return createdAt, nil
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

func (r *Repository) IncrementPhoneCodeAttempts(ctx context.Context, codeID string) error {
	_, err := r.db.Exec(ctx, `UPDATE phone_verification_codes SET attempts = attempts + 1 WHERE id = $1`, codeID)
	if err != nil {
		return fmt.Errorf("increment phone code attempts: %w", err)
	}
	return nil
}

func (r *Repository) ConsumePhoneCode(ctx context.Context, codeID string) error {
	_, err := r.db.Exec(ctx, `UPDATE phone_verification_codes SET consumed_at = now() WHERE id = $1 AND consumed_at IS NULL`, codeID)
	if err != nil {
		return fmt.Errorf("consume phone code: %w", err)
	}
	return nil
}

func (r *Repository) GetPhoneUserByMobile(ctx context.Context, mobile string) (domain.PhoneAuthUser, error) {
	query := `SELECT id, COALESCE(phone_number, ''), display_name, created_at FROM users WHERE phone_number = $1`
	var user domain.PhoneAuthUser
	err := r.db.QueryRow(ctx, query, mobile).Scan(&user.ID, &user.Mobile, &user.DisplayName, &user.CreatedAt)
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
	_, err := r.db.Exec(ctx, `UPDATE users SET phone_verified_at = now(), updated_at = now() WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("mark phone verified: %w", err)
	}
	return nil
}

func (r *Repository) GetAuthUserByID(ctx context.Context, userID string) (domain.User, error) {
	query := `SELECT id, COALESCE(email, ''), COALESCE(phone_number, ''), display_name, created_at FROM users WHERE id = $1`
	var id string
	var email string
	var mobile string
	var displayName string
	var createdAt time.Time
	err := r.db.QueryRow(ctx, query, userID).Scan(&id, &email, &mobile, &displayName, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get auth user by id: %w", err)
	}
	return domain.User{ID: id, Email: email, DisplayName: displayName, CreatedAt: createdAt}, nil
}

func NormalizePhoneDigits(raw string) string {
	return strings.TrimSpace(raw)
}
