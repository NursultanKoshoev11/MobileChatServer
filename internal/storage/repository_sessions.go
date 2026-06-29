package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type RefreshSessionRecord struct {
	ID         string
	UserID     string
	SecretHash string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

func (r *Repository) CreateRefreshSession(ctx context.Context, id, userID, secretHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, id, userID, secretHash, expiresAt)
	if err != nil {
		return fmt.Errorf("create refresh session: %w", err)
	}
	return nil
}

func (r *Repository) GetRefreshSession(ctx context.Context, secretHash string) (RefreshSessionRecord, error) {
	query := `SELECT id, user_id, token_hash, expires_at, revoked_at, created_at FROM refresh_tokens WHERE token_hash = $1`
	var session RefreshSessionRecord
	err := r.db.QueryRow(ctx, query, secretHash).Scan(&session.ID, &session.UserID, &session.SecretHash, &session.ExpiresAt, &session.RevokedAt, &session.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshSessionRecord{}, ErrNotFound
	}
	if err != nil {
		return RefreshSessionRecord{}, fmt.Errorf("get refresh session: %w", err)
	}
	return session, nil
}

func (r *Repository) RevokeRefreshSession(ctx context.Context, sessionID string) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, sessionID)
	if err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	return nil
}

func (r *Repository) RotateRefreshSession(ctx context.Context, oldSessionID, newSessionID, userID, newSecretHash string, expiresAt time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin refresh rotation: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, oldSessionID, userID)
	if err != nil {
		return fmt.Errorf("revoke old refresh session: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, newSessionID, userID, newSecretHash, expiresAt); err != nil {
		return fmt.Errorf("create rotated refresh session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit refresh rotation: %w", err)
	}
	return nil
}
