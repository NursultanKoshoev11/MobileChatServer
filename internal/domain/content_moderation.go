package domain

import "time"

type ContentModerationContentType string

const (
	ContentTypeGroupMessage         ContentModerationContentType = "group_message"
	ContentTypePublicRequest        ContentModerationContentType = "public_request"
	ContentTypePublicRequestComment ContentModerationContentType = "public_request_comment"
)

type ContentModerationStatus string

const (
	ContentModerationStatusPending  ContentModerationStatus = "pending"
	ContentModerationStatusApproved ContentModerationStatus = "approved"
	ContentModerationStatusRejected ContentModerationStatus = "rejected"
)

type ContentModerationItem struct {
	ID                  string                       `json:"id"`
	GroupID             string                       `json:"group_id"`
	ContentType         ContentModerationContentType `json:"content_type"`
	AuthorID            string                       `json:"author_id"`
	AuthorName          string                       `json:"author_name,omitempty"`
	TargetID            string                       `json:"target_id,omitempty"`
	Title               string                       `json:"title,omitempty"`
	Body                string                       `json:"body"`
	RequestType         PublicRequestType            `json:"request_type,omitempty"`
	InteractionMode     PublicRequestInteractionMode `json:"interaction_mode,omitempty"`
	Status              ContentModerationStatus      `json:"status"`
	Decision            string                       `json:"decision"`
	Reasons             []string                     `json:"reasons"`
	Provider            string                       `json:"provider"`
	ProviderModel       string                       `json:"provider_model,omitempty"`
	ProviderResponseID  string                       `json:"provider_response_id,omitempty"`
	ProviderScoresJSON  string                       `json:"-"`
	PublishedResourceID string                       `json:"published_resource_id,omitempty"`
	CreatedAt           time.Time                    `json:"created_at"`
	ReviewedAt          *time.Time                   `json:"reviewed_at,omitempty"`
	ReviewedBy          string                       `json:"reviewed_by,omitempty"`
}

type ContentModerationReviewResult struct {
	Item          ContentModerationItem `json:"item"`
	Message       *Message              `json:"message,omitempty"`
	PublicRequest *PublicRequest        `json:"public_request,omitempty"`
	Comment       *PublicRequestComment `json:"comment,omitempty"`
}
