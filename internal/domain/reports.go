package domain

import "time"

type ReportTargetType string

const (
	ReportTargetPublicRequest ReportTargetType = "public_request"
	ReportTargetComment       ReportTargetType = "comment"
	ReportTargetMessage       ReportTargetType = "message"
	ReportTargetUser          ReportTargetType = "user"
)

func (t ReportTargetType) Valid() bool {
	switch t {
	case ReportTargetPublicRequest, ReportTargetComment, ReportTargetMessage, ReportTargetUser:
		return true
	default:
		return false
	}
}

type ReportReason string

const (
	ReportReasonSpam    ReportReason = "spam"
	ReportReasonAbuse   ReportReason = "abuse"
	ReportReasonIllegal ReportReason = "illegal"
	ReportReasonOther   ReportReason = "other"
)

func (r ReportReason) Valid() bool {
	switch r {
	case ReportReasonSpam, ReportReasonAbuse, ReportReasonIllegal, ReportReasonOther:
		return true
	default:
		return false
	}
}

type ReportStatus string

const (
	ReportStatusOpen     ReportStatus = "open"
	ReportStatusResolved ReportStatus = "resolved"
)

type ReportResolution string

const (
	ReportResolutionDismissed      ReportResolution = "dismissed"
	ReportResolutionContentRemoved ReportResolution = "content_removed"
)

func (r ReportResolution) Valid() bool {
	return r == ReportResolutionDismissed || r == ReportResolutionContentRemoved
}

// ContentReport is the moderator-facing view of a report. It deliberately
// carries no phone numbers.
type ContentReport struct {
	ID                  string           `json:"id"`
	TargetType          ReportTargetType `json:"target_type"`
	TargetID            string           `json:"target_id"`
	GroupID             string           `json:"group_id,omitempty"`
	Reason              ReportReason     `json:"reason"`
	Details             string           `json:"details"`
	Status              ReportStatus     `json:"status"`
	Resolution          string           `json:"resolution,omitempty"`
	ReporterID          string           `json:"-"`
	ReporterDisplayName string           `json:"reporter_display_name"`
	CreatedAt           time.Time        `json:"created_at"`
	ResolvedAt          *time.Time       `json:"resolved_at,omitempty"`
	// TargetUserID is the author of the reported content (or the reported user).
	TargetUserID string `json:"target_user_id,omitempty"`
}

type UserBlock struct {
	UserID      string    `json:"user_id"`
	DisplayName string    `json:"display_name"`
	BlockedAt   time.Time `json:"blocked_at"`
}
