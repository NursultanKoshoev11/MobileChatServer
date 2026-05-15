package domain

import "time"

type GroupCreationRequestStatus string

const (
	GroupCreationRequestPending       GroupCreationRequestStatus = "pending"
	GroupCreationRequestApproved      GroupCreationRequestStatus = "approved"
	GroupCreationRequestRejected      GroupCreationRequestStatus = "rejected"
	GroupCreationRequestNeedsMoreInfo GroupCreationRequestStatus = "needs_more_info"
)

type GroupCreationRequest struct {
	ID               string                     `json:"id"`
	RequesterID      string                     `json:"requester_id"`
	RequesterName    string                     `json:"requester_name,omitempty"`
	RequesterPhone   string                     `json:"requester_phone,omitempty"`
	ApplicantName    string                     `json:"applicant_name"`
	Position         string                     `json:"position"`
	OrganizationName string                     `json:"organization_name"`
	OrganizationType string                     `json:"organization_type"`
	Region           string                     `json:"region"`
	OfficialPhone    string                     `json:"official_phone"`
	OfficialEmail    string                     `json:"official_email"`
	Website          string                     `json:"website"`
	GroupTitle       string                     `json:"group_title"`
	GroupDescription string                     `json:"group_description"`
	Reason           string                     `json:"reason"`
	Documents        string                     `json:"documents"`
	Status           GroupCreationRequestStatus `json:"status"`
	AdminComment     string                     `json:"admin_comment,omitempty"`
	CreatedGroupID   string                     `json:"created_group_id,omitempty"`
	ReviewedBy       string                     `json:"reviewed_by,omitempty"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
	ReviewedAt       *time.Time                 `json:"reviewed_at,omitempty"`
}
