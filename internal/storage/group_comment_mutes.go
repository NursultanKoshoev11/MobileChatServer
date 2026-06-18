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

func (r *Repository) SetGroupCommentMute(ctx context.Context, groupID, actorID, targetUserID string, mutedUntil *time.Time, reason string) (domain.GroupCommentMute, error) {
	if err := r.ensureGroupCommentMutesTable(ctx); err != nil {
		return domain.GroupCommentMute{}, err
	}
	actorRole, err := r.GetMemberRole(ctx, groupID, actorID)
	if err != nil {
		return domain.GroupCommentMute{}, err
	}
	targetRole, err := r.GetMemberRole(ctx, groupID, targetUserID)
	if err != nil {
		return domain.GroupCommentMute{}, err
	}
	if targetUserID == actorID || targetRole == domain.RoleOwner {
		return domain.GroupCommentMute{}, ErrForbidden
	}
	if actorRole != domain.RoleOwner && actorRole != domain.RoleAdmin {
		return domain.GroupCommentMute{}, ErrForbidden
	}
	if actorRole == domain.RoleAdmin && targetRole != domain.RoleMember {
		return domain.GroupCommentMute{}, ErrForbidden
	}

	query := `
		INSERT INTO group_comment_mutes (group_id, user_id, muted_by, muted_until, reason, created_at, updated_at, unmuted_at)
		VALUES ($1, $2, $3, $4, $5, now(), now(), NULL)
		ON CONFLICT (group_id, user_id)
		DO UPDATE SET muted_by = EXCLUDED.muted_by, muted_until = EXCLUDED.muted_until, reason = EXCLUDED.reason, updated_at = now(), unmuted_at = NULL
		RETURNING group_id, user_id, COALESCE(muted_by, ''), muted_until, reason, created_at, updated_at`
	var mute domain.GroupCommentMute
	var until sql.NullTime
	if err := r.db.QueryRow(ctx, query, groupID, targetUserID, actorID, mutedUntil, strings.TrimSpace(reason)).Scan(&mute.GroupID, &mute.UserID, &mute.MutedBy, &until, &mute.Reason, &mute.CreatedAt, &mute.UpdatedAt); err != nil {
		return domain.GroupCommentMute{}, fmt.Errorf("set group comment mute: %w", err)
	}
	if until.Valid {
		mute.MutedUntil = &until.Time
	}
	return mute, nil
}

func (r *Repository) ClearGroupCommentMute(ctx context.Context, groupID, actorID, targetUserID string) error {
	if err := r.ensureGroupCommentMutesTable(ctx); err != nil {
		return err
	}
	actorRole, err := r.GetMemberRole(ctx, groupID, actorID)
	if err != nil {
		return err
	}
	targetRole, err := r.GetMemberRole(ctx, groupID, targetUserID)
	if err != nil {
		return err
	}
	if actorRole != domain.RoleOwner && actorRole != domain.RoleAdmin {
		return ErrForbidden
	}
	if actorRole == domain.RoleAdmin && targetRole != domain.RoleMember {
		return ErrForbidden
	}
	result, err := r.db.Exec(ctx, `UPDATE group_comment_mutes SET unmuted_at = now(), updated_at = now() WHERE group_id = $1 AND user_id = $2 AND unmuted_at IS NULL`, groupID, targetUserID)
	if err != nil {
		return fmt.Errorf("clear group comment mute: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetActiveCommentMuteForRequest(ctx context.Context, requestID, userID string) (domain.GroupCommentMute, bool, error) {
	if err := r.ensureGroupCommentMutesTable(ctx); err != nil {
		return domain.GroupCommentMute{}, false, err
	}
	query := `
		SELECT m.group_id, m.user_id, COALESCE(m.muted_by, ''), m.muted_until, m.reason, m.created_at, m.updated_at
		FROM public_requests pr
		JOIN group_comment_mutes m ON m.group_id = pr.group_id AND m.user_id = $2
		WHERE pr.id = $1
		  AND pr.hidden_at IS NULL
		  AND m.unmuted_at IS NULL
		  AND (m.muted_until IS NULL OR m.muted_until > now())`
	var mute domain.GroupCommentMute
	var until sql.NullTime
	err := r.db.QueryRow(ctx, query, requestID, userID).Scan(&mute.GroupID, &mute.UserID, &mute.MutedBy, &until, &mute.Reason, &mute.CreatedAt, &mute.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GroupCommentMute{}, false, nil
	}
	if err != nil {
		return domain.GroupCommentMute{}, false, fmt.Errorf("get active comment mute: %w", err)
	}
	if until.Valid {
		mute.MutedUntil = &until.Time
	}
	return mute, true, nil
}

func (r *Repository) ensureGroupCommentMutesTable(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS group_comment_mutes (
			group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			muted_by TEXT REFERENCES users(id) ON DELETE SET NULL,
			muted_until TIMESTAMPTZ,
			reason TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			unmuted_at TIMESTAMPTZ,
			PRIMARY KEY (group_id, user_id)
		)`)
	if err != nil {
		return fmt.Errorf("ensure group comment mutes table: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_group_comment_mutes_active
		ON group_comment_mutes (group_id, user_id, muted_until)
		WHERE unmuted_at IS NULL`)
	if err != nil {
		return fmt.Errorf("ensure group comment mutes index: %w", err)
	}
	return nil
}
