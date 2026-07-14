package domain

import "time"

type PublicRequestType string

const (
	PublicRequestAnnouncement PublicRequestType = "announcement"
	PublicRequestSuggestion   PublicRequestType = "suggestion"
	PublicRequestComplaint    PublicRequestType = "complaint"
	PublicRequestRequirement  PublicRequestType = "requirement"
	PublicRequestProblem      PublicRequestType = "problem"
	PublicRequestIdea         PublicRequestType = "idea"
)

type PublicRequestInteractionMode string

const (
	InteractionModeReadOnly   PublicRequestInteractionMode = "read_only"
	InteractionModeVoteOnly   PublicRequestInteractionMode = "vote_only"
	InteractionModeDiscussion PublicRequestInteractionMode = "discussion"
)

type PublicRequestStatus string

const (
	PublicRequestStatusNew         PublicRequestStatus = "new"
	PublicRequestStatusUnderReview PublicRequestStatus = "under_review"
	PublicRequestStatusAccepted    PublicRequestStatus = "accepted"
	PublicRequestStatusRejected    PublicRequestStatus = "rejected"
	PublicRequestStatusResolved    PublicRequestStatus = "resolved"
)

type PublicRequest struct {
	ID              string                       `json:"id"`
	GroupID         string                       `json:"group_id"`
	AuthorID        string                       `json:"author_id"`
	AuthorName      string                       `json:"author_name"`
	RequestType     PublicRequestType            `json:"request_type"`
	InteractionMode PublicRequestInteractionMode `json:"interaction_mode"`
	Title           string                       `json:"title"`
	Body            string                       `json:"body"`
	Status          PublicRequestStatus          `json:"status"`
	SupportCount    int                          `json:"support_count"`
	OpposeCount     int                          `json:"oppose_count"`
	CommentCount    int                          `json:"comment_count"`
	MyVote          *string                      `json:"my_vote,omitempty"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
}

type PublicRequestVoteUpdate struct {
	RequestID    string  `json:"request_id"`
	SupportCount int     `json:"support_count"`
	OpposeCount  int     `json:"oppose_count"`
	VoterID      string  `json:"voter_id"`
	VoteType     *string `json:"vote_type"`
}

type PublicRequestComment struct {
	ID         string     `json:"id"`
	RequestID  string     `json:"request_id"`
	AuthorID   string     `json:"author_id"`
	AuthorName string     `json:"author_name"`
	Body       string     `json:"body"`
	CreatedAt  time.Time  `json:"created_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}
