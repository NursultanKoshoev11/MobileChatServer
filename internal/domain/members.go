package domain

import "time"

type GroupMember struct {
	UserID      string    `json:"user_id"`
	DisplayName string    `json:"display_name"`
	Phone       string    `json:"phone,omitempty"`
	Role        GroupRole `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}
