package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetAuthPhoneUserByID(ctx context.Context, userID string) (domain.PhoneAuthUser, error) {
	query := `SELECT id, COALESCE(phone_number, ''), display_name, created_at FROM users WHERE id = $1`
	var user domain.PhoneAuthUser
	err := r.db.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Mobile, &user.DisplayName, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PhoneAuthUser{}, ErrNotFound
	}
	if err != nil {
		return domain.PhoneAuthUser{}, fmt.Errorf("get auth phone user by id: %w", err)
	}
	return user, nil
}

func (r *Repository) GetPhoneUserCreatedAt(ctx context.Context, userID string) (time.Time, error) {
	var createdAt time.Time
	if err := r.db.QueryRow(ctx, `SELECT created_at FROM users WHERE id = $1`, userID).Scan(&createdAt); err != nil {
		return time.Time{}, err
	}
	return createdAt, nil
}
