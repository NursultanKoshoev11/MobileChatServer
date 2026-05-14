package domain

import "time"

type GroupVisibility string

const (
	VisibilityPublic  GroupVisibility = "public"
	VisibilityPrivate GroupVisibility = "private"
)

type GroupRole string

const (
	RoleOwner  GroupRole = "owner"
	RoleAdmin  GroupRole = "admin"
	RoleMember GroupRole = "member"
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type Group struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Visibility  GroupVisibility `json:"visibility"`
	OwnerID     string          `json:"owner_id"`
	InviteCode  string          `json:"invite_code,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	MemberCount int             `json:"member_count"`
	MyRole      *GroupRole      `json:"my_role,omitempty"`
}

type Message struct {
	ID         string    `json:"id"`
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	Text       string    `json:"text"`
	CreatedAt  time.Time `json:"created_at"`
}

type Session struct {
	AccessToken string `json:"access_token"`
	User        User   `json:"user"`
}
