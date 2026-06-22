package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) UpdateMessage(ctx context.Context, groupID, actorID, messageID, text string) (domain.Message, error) {
	query := `
		WITH updated AS (
			UPDATE messages
			SET text = $4, edited_at = now()
			WHERE id = $1 AND group_id = $2 AND sender_id = $3 AND deleted_at IS NULL
			RETURNING id, group_id, sender_id, text, created_at, edited_at, deleted_at
		)
		SELECT m.id, m.group_id, m.sender_id, u.display_name, m.text, m.created_at, m.edited_at, m.deleted_at
		FROM updated m
		JOIN users u ON u.id = m.sender_id`
	var message domain.Message
	err := r.db.QueryRow(ctx, query, messageID, groupID, actorID, text).Scan(
		&message.ID,
		&message.GroupID,
		&message.SenderID,
		&message.SenderName,
		&message.Text,
		&message.CreatedAt,
		&message.EditedAt,
		&message.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, ErrForbidden
	}
	if err != nil {
		return domain.Message{}, fmt.Errorf("update message: %w", err)
	}
	return message, nil
}

func (r *Repository) DeleteMessage(ctx context.Context, groupID, actorID, messageID string) (domain.Message, error) {
	var senderID string
	if err := r.db.QueryRow(ctx, `SELECT sender_id FROM messages WHERE id = $1 AND group_id = $2 AND deleted_at IS NULL`, messageID, groupID).Scan(&senderID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Message{}, ErrNotFound
		}
		return domain.Message{}, fmt.Errorf("load message for delete: %w", err)
	}
	if senderID != actorID {
		role, err := r.GetMemberRole(ctx, groupID, actorID)
		if err != nil {
			return domain.Message{}, err
		}
		if role != domain.RoleOwner && role != domain.RoleAdmin {
			return domain.Message{}, ErrForbidden
		}
	}

	query := `
		WITH updated AS (
			UPDATE messages
			SET deleted_at = now(), text = ''
			WHERE id = $1 AND group_id = $2 AND deleted_at IS NULL
			RETURNING id, group_id, sender_id, text, created_at, edited_at, deleted_at
		)
		SELECT m.id, m.group_id, m.sender_id, u.display_name, m.text, m.created_at, m.edited_at, m.deleted_at
		FROM updated m
		JOIN users u ON u.id = m.sender_id`
	var message domain.Message
	err := r.db.QueryRow(ctx, query, messageID, groupID).Scan(
		&message.ID,
		&message.GroupID,
		&message.SenderID,
		&message.SenderName,
		&message.Text,
		&message.CreatedAt,
		&message.EditedAt,
		&message.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, ErrNotFound
	}
	if err != nil {
		return domain.Message{}, fmt.Errorf("delete message: %w", err)
	}
	return message, nil
}
