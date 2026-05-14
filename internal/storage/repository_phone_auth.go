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

func (r *Repository) GetUserByPhone(ctx context.Context, phoneNumber string) (domain.User, error) {
	query := `SELECT id, COALESCE(email, ''), COALESCE(phone_number, ''), display_name, created_at FROM users WHERE phone_number = $1`
	var user domain.User
	err := r.db.QueryRow(ctx, query, phoneNumber).Scan(&user.ID, &user.Email, &user.PhoneNumber, &user.DisplayName, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by phone: %w", err)
	}
	return user, nil
}

func (r *Repository) CreatePhoneUser(ctx context.Context, user domain.User) (domain.User, error) {
	query := `
		INSERT INTO users (id, phone_number, phone_verified_at, display_name, created_at, updated_at)
		VALUES ($1, $2, now(), $3, now(), now())
		RETURNING created_at`
	if err := r.db.QueryRow(ctx, query, user.ID, user.PhoneNumber, user.DisplayName).Scan(&user.CreatedAt); err != nil {
		return domain.User{}, fmt.Errorf("create phone user: %w", err)
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

func NormalizePhoneDigits(raw string) string {
	return strings.TrimSpace(raw)
}
