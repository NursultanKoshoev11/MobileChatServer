package domain

import "time"

type GroupCommentMute struct {
	GroupID    string     `json:"group_id"`
	UserID     string     `json:"user_id"`
	MutedBy    string     `json:"muted_by,omitempty"`
	MutedUntil *time.Time `json:"muted_until,omitempty"`
	Reason     string     `json:"reason,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
