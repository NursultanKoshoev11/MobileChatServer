package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetUserProfileByID(ctx context.Context, userID string) (domain.User, error) {
	var user domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, COALESCE(email, ''), COALESCE(NULLIF(phone, ''), phone_number, ''),
		       display_name, COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at
		FROM users
		WHERE id = $1`, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Phone,
		&user.DisplayName,
		&user.Role,
		&user.AvatarData,
		&user.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user profile: %w", err)
	}
	return user, nil
}

func (r *Repository) UpdateUserAvatar(ctx context.Context, userID, avatarData string) (domain.User, error) {
	command, err := r.db.Exec(ctx, `
		UPDATE users
		SET avatar_data = $2, updated_at = now()
		WHERE id = $1`, userID, avatarData)
	if err != nil {
		return domain.User{}, fmt.Errorf("update user avatar: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.User{}, ErrNotFound
	}
	return r.GetUserProfileByID(ctx, userID)
}
