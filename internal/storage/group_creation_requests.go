package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func (r *Repository) CreateGroupCreationRequest(ctx context.Context, request domain.GroupCreationRequest) (domain.GroupCreationRequest, error) {
	_, err := r.db.Exec(ctx, `INSERT INTO group_creation_requests (id, requester_id, applicant_name, position, organization_name, organization_type, region, official_phone, official_email, website, group_title, group_description, reason, documents) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, request.ID, request.RequesterID, request.ApplicantName, request.Position, request.OrganizationName, request.OrganizationType, request.Region, request.OfficialPhone, request.OfficialEmail, request.Website, request.GroupTitle, request.GroupDescription, request.Reason, request.Documents)
	if err != nil {
		return domain.GroupCreationRequest{}, fmt.Errorf("create group creation request: %w", err)
	}
	return r.GetGroupCreationRequestByID(ctx, request.ID)
}

func (r *Repository) GetGroupCreationRequestByID(ctx context.Context, id string) (domain.GroupCreationRequest, error) {
	var req domain.GroupCreationRequest
	err := r.db.QueryRow(ctx, `SELECT id, requester_id, applicant_name, position, organization_name, organization_type, region, official_phone, official_email, website, group_title, group_description, reason, documents, status, admin_comment, COALESCE(created_group_id,''), COALESCE(reviewed_by,''), created_at, updated_at, reviewed_at FROM group_creation_requests WHERE id=$1`, strings.TrimSpace(id)).Scan(&req.ID, &req.RequesterID, &req.ApplicantName, &req.Position, &req.OrganizationName, &req.OrganizationType, &req.Region, &req.OfficialPhone, &req.OfficialEmail, &req.Website, &req.GroupTitle, &req.GroupDescription, &req.Reason, &req.Documents, &req.Status, &req.AdminComment, &req.CreatedGroupID, &req.ReviewedBy, &req.CreatedAt, &req.UpdatedAt, &req.ReviewedAt)
	if err != nil {
		return domain.GroupCreationRequest{}, err
	}
	return req, nil
}

func (r *Repository) ListMyGroupCreationRequests(ctx context.Context, requesterID string) ([]domain.GroupCreationRequest, error) {
	rows, err := r.db.Query(ctx, `SELECT id, requester_id, applicant_name, position, organization_name, organization_type, region, official_phone, official_email, website, group_title, group_description, reason, documents, status, admin_comment, COALESCE(created_group_id,''), COALESCE(reviewed_by,''), created_at, updated_at, reviewed_at FROM group_creation_requests WHERE requester_id=$1 ORDER BY created_at DESC LIMIT 100`, requesterID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []domain.GroupCreationRequest{}
	for rows.Next() { var req domain.GroupCreationRequest; if err := rows.Scan(&req.ID,&req.RequesterID,&req.ApplicantName,&req.Position,&req.OrganizationName,&req.OrganizationType,&req.Region,&req.OfficialPhone,&req.OfficialEmail,&req.Website,&req.GroupTitle,&req.GroupDescription,&req.Reason,&req.Documents,&req.Status,&req.AdminComment,&req.CreatedGroupID,&req.ReviewedBy,&req.CreatedAt,&req.UpdatedAt,&req.ReviewedAt); err != nil { return nil, err }; out = append(out, req) }
	return out, rows.Err()
}

func (r *Repository) ListGroupCreationRequestsForAdmin(ctx context.Context, status string, limit int) ([]domain.GroupCreationRequest, error) {
	if limit <= 0 || limit > 200 { limit = 100 }
	rows, err := r.db.Query(ctx, `SELECT id, requester_id, applicant_name, position, organization_name, organization_type, region, official_phone, official_email, website, group_title, group_description, reason, documents, status, admin_comment, COALESCE(created_group_id,''), COALESCE(reviewed_by,''), created_at, updated_at, reviewed_at FROM group_creation_requests WHERE ($1='' OR status=$1) ORDER BY created_at DESC LIMIT $2`, strings.TrimSpace(status), limit)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []domain.GroupCreationRequest{}
	for rows.Next() { var req domain.GroupCreationRequest; if err := rows.Scan(&req.ID,&req.RequesterID,&req.ApplicantName,&req.Position,&req.OrganizationName,&req.OrganizationType,&req.Region,&req.OfficialPhone,&req.OfficialEmail,&req.Website,&req.GroupTitle,&req.GroupDescription,&req.Reason,&req.Documents,&req.Status,&req.AdminComment,&req.CreatedGroupID,&req.ReviewedBy,&req.CreatedAt,&req.UpdatedAt,&req.ReviewedAt); err != nil { return nil, err }; out = append(out, req) }
	return out, rows.Err()
}
