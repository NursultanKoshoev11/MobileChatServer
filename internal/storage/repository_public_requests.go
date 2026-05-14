package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreatePublicRequest(ctx context.Context, request domain.PublicRequest) (domain.PublicRequest, error) {
	isMember, err := r.IsGroupMember(ctx, request.GroupID, request.AuthorID)
	if err != nil {
		return domain.PublicRequest{}, err
	}
	if !isMember {
		return domain.PublicRequest{}, ErrForbidden
	}
	query := `
		INSERT INTO public_requests (id, group_id, author_id, request_type, interaction_mode, title, body, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'new', now(), now())
		RETURNING status, created_at, updated_at`
	if err := r.db.QueryRow(ctx, query, request.ID, request.GroupID, request.AuthorID, request.RequestType, request.InteractionMode, request.Title, request.Body).Scan(&request.Status, &request.CreatedAt, &request.UpdatedAt); err != nil {
		return domain.PublicRequest{}, fmt.Errorf("create public request: %w", err)
	}
	user, err := r.GetUserByID(ctx, request.AuthorID)
	if err != nil {
		return domain.PublicRequest{}, err
	}
	request.AuthorName = user.DisplayName
	return request, nil
}

func (r *Repository) ListPublicRequests(ctx context.Context, groupID, viewerID string, limit int, before time.Time, mineOnly bool) ([]domain.PublicRequest, error) {
	isMember, err := r.IsGroupMember(ctx, groupID, viewerID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var beforePtr *time.Time
	if !before.IsZero() {
		beforePtr = &before
	}
	query := `
		SELECT pr.id, pr.group_id, pr.author_id, u.display_name, pr.request_type, pr.interaction_mode, pr.title, pr.body, pr.status,
		       COALESCE(SUM(CASE WHEN v.vote_type = 'support' THEN 1 ELSE 0 END), 0)::int AS support_count,
		       COALESCE(SUM(CASE WHEN v.vote_type = 'oppose' THEN 1 ELSE 0 END), 0)::int AS oppose_count,
		       (SELECT COUNT(*)::int FROM public_request_comments c WHERE c.request_id = pr.id AND c.deleted_at IS NULL) AS comment_count,
		       COALESCE(myv.vote_type, '') AS my_vote,
		       pr.created_at, pr.updated_at
		FROM public_requests pr
		JOIN users u ON u.id = pr.author_id
		LEFT JOIN public_request_votes v ON v.request_id = pr.id
		LEFT JOIN public_request_votes myv ON myv.request_id = pr.id AND myv.user_id = $2
		WHERE pr.group_id = $1
		  AND pr.hidden_at IS NULL
		  AND ($3::timestamptz IS NULL OR pr.created_at < $3)
		  AND ($4::boolean = false OR pr.author_id = $2)
		GROUP BY pr.id, u.display_name, myv.vote_type
		ORDER BY pr.created_at DESC
		LIMIT $5`
	rows, err := r.db.Query(ctx, query, groupID, viewerID, beforePtr, mineOnly, limit)
	if err != nil {
		return nil, fmt.Errorf("list public requests: %w", err)
	}
	defer rows.Close()

	requests := make([]domain.PublicRequest, 0)
	for rows.Next() {
		var request domain.PublicRequest
		var myVote string
		if err := rows.Scan(
			&request.ID,
			&request.GroupID,
			&request.AuthorID,
			&request.AuthorName,
			&request.RequestType,
			&request.InteractionMode,
			&request.Title,
			&request.Body,
			&request.Status,
			&request.SupportCount,
			&request.OpposeCount,
			&request.CommentCount,
			&myVote,
			&request.CreatedAt,
			&request.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan public request: %w", err)
		}
		if myVote != "" {
			request.MyVote = &myVote
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (r *Repository) VotePublicRequest(ctx context.Context, requestID, userID, voteType string) error {
	groupID, err := r.publicRequestGroupID(ctx, requestID)
	if err != nil {
		return err
	}
	mode, err := r.publicRequestInteractionMode(ctx, requestID)
	if err != nil {
		return err
	}
	if mode == domain.InteractionModeReadOnly {
		return ErrForbidden
	}
	isMember, err := r.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrForbidden
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO public_request_votes (request_id, user_id, vote_type, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (request_id, user_id) DO UPDATE SET vote_type = EXCLUDED.vote_type, updated_at = now()`, requestID, userID, voteType)
	if err != nil {
		return fmt.Errorf("vote public request: %w", err)
	}
	return nil
}

func (r *Repository) ClearPublicRequestVote(ctx context.Context, requestID, userID string) error {
	groupID, err := r.publicRequestGroupID(ctx, requestID)
	if err != nil {
		return err
	}
	mode, err := r.publicRequestInteractionMode(ctx, requestID)
	if err != nil {
		return err
	}
	if mode == domain.InteractionModeReadOnly {
		return ErrForbidden
	}
	isMember, err := r.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrForbidden
	}
	_, err = r.db.Exec(ctx, `DELETE FROM public_request_votes WHERE request_id = $1 AND user_id = $2`, requestID, userID)
	if err != nil {
		return fmt.Errorf("clear public request vote: %w", err)
	}
	return nil
}

func (r *Repository) CreatePublicRequestComment(ctx context.Context, comment domain.PublicRequestComment) (domain.PublicRequestComment, error) {
	groupID, err := r.publicRequestGroupID(ctx, comment.RequestID)
	if err != nil {
		return domain.PublicRequestComment{}, err
	}
	mode, err := r.publicRequestInteractionMode(ctx, comment.RequestID)
	if err != nil {
		return domain.PublicRequestComment{}, err
	}
	if mode != domain.InteractionModeDiscussion {
		return domain.PublicRequestComment{}, ErrForbidden
	}
	isMember, err := r.IsGroupMember(ctx, groupID, comment.AuthorID)
	if err != nil {
		return domain.PublicRequestComment{}, err
	}
	if !isMember {
		return domain.PublicRequestComment{}, ErrForbidden
	}
	if err := r.db.QueryRow(ctx, `
		INSERT INTO public_request_comments (id, request_id, author_id, body, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING created_at`, comment.ID, comment.RequestID, comment.AuthorID, comment.Body).Scan(&comment.CreatedAt); err != nil {
		return domain.PublicRequestComment{}, fmt.Errorf("create public request comment: %w", err)
	}
	user, err := r.GetUserByID(ctx, comment.AuthorID)
	if err != nil {
		return domain.PublicRequestComment{}, err
	}
	comment.AuthorName = user.DisplayName
	return comment, nil
}

func (r *Repository) ListPublicRequestComments(ctx context.Context, requestID, viewerID string) ([]domain.PublicRequestComment, error) {
	groupID, err := r.publicRequestGroupID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	isMember, err := r.IsGroupMember(ctx, groupID, viewerID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrForbidden
	}
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.request_id, c.author_id, u.display_name, c.body, c.created_at, c.deleted_at
		FROM public_request_comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.request_id = $1 AND c.deleted_at IS NULL
		ORDER BY c.created_at ASC`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list public request comments: %w", err)
	}
	defer rows.Close()

	comments := make([]domain.PublicRequestComment, 0)
	for rows.Next() {
		var comment domain.PublicRequestComment
		if err := rows.Scan(&comment.ID, &comment.RequestID, &comment.AuthorID, &comment.AuthorName, &comment.Body, &comment.CreatedAt, &comment.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan public request comment: %w", err)
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (r *Repository) DeletePublicRequestComment(ctx context.Context, commentID, moderatorID string) error {
	var groupID string
	err := r.db.QueryRow(ctx, `
		SELECT pr.group_id
		FROM public_request_comments c
		JOIN public_requests pr ON pr.id = c.request_id
		WHERE c.id = $1 AND c.deleted_at IS NULL AND pr.hidden_at IS NULL`, commentID).Scan(&groupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load comment group: %w", err)
	}
	role, err := r.GetMemberRole(ctx, groupID, moderatorID)
	if err != nil {
		return err
	}
	if role != domain.RoleOwner && role != domain.RoleAdmin {
		return ErrForbidden
	}
	result, err := r.db.Exec(ctx, `UPDATE public_request_comments SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, commentID)
	if err != nil {
		return fmt.Errorf("delete public request comment: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) HidePublicRequest(ctx context.Context, requestID, moderatorID string) error {
	groupID, err := r.publicRequestGroupID(ctx, requestID)
	if err != nil {
		return err
	}
	role, err := r.GetMemberRole(ctx, groupID, moderatorID)
	if err != nil {
		return err
	}
	if role != domain.RoleOwner && role != domain.RoleAdmin {
		return ErrForbidden
	}
	result, err := r.db.Exec(ctx, `UPDATE public_requests SET hidden_at = now(), hidden_by = $1, updated_at = now() WHERE id = $2 AND hidden_at IS NULL`, moderatorID, requestID)
	if err != nil {
		return fmt.Errorf("hide public request: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdatePublicRequestStatus(ctx context.Context, requestID, adminID string, status domain.PublicRequestStatus) error {
	groupID, err := r.publicRequestGroupID(ctx, requestID)
	if err != nil {
		return err
	}
	role, err := r.GetMemberRole(ctx, groupID, adminID)
	if err != nil {
		return err
	}
	if role != domain.RoleOwner && role != domain.RoleAdmin {
		return ErrForbidden
	}
	_, err = r.db.Exec(ctx, `UPDATE public_requests SET status = $1, updated_at = now() WHERE id = $2 AND hidden_at IS NULL`, status, requestID)
	if err != nil {
		return fmt.Errorf("update public request status: %w", err)
	}
	return nil
}

func (r *Repository) publicRequestGroupID(ctx context.Context, requestID string) (string, error) {
	var groupID string
	if err := r.db.QueryRow(ctx, `SELECT group_id FROM public_requests WHERE id = $1 AND hidden_at IS NULL`, requestID).Scan(&groupID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("load public request group: %w", err)
	}
	return groupID, nil
}

func (r *Repository) publicRequestInteractionMode(ctx context.Context, requestID string) (domain.PublicRequestInteractionMode, error) {
	var mode domain.PublicRequestInteractionMode
	if err := r.db.QueryRow(ctx, `SELECT interaction_mode FROM public_requests WHERE id = $1 AND hidden_at IS NULL`, requestID).Scan(&mode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("load public request interaction mode: %w", err)
	}
	return mode, nil
}
