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

type UserRole string

const (
	UserRoleUser          UserRole = "user"
	UserRolePlatformAdmin UserRole = "platform_admin"
	UserRoleSuperAdmin    UserRole = "super_admin"
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone,omitempty"`
	DisplayName string    `json:"display_name"`
	Role        UserRole  `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

type Group struct {
	ID                       string          `json:"id"`
	Title                    string          `json:"title"`
	Description              string          `json:"description"`
	Visibility               GroupVisibility `json:"visibility"`
	OwnerID                  string          `json:"owner_id"`
	InviteCode               string          `json:"invite_code,omitempty"`
	QRPass                   string          `json:"qr_pass,omitempty"`
	CreatedAt                time.Time       `json:"created_at"`
	MemberCount              int             `json:"member_count"`
	UnreadPublicRequestCount int             `json:"unread_public_request_count"`
	MyRole                   *GroupRole      `json:"my_role,omitempty"`
}

type MediaType string

const (
	MediaImage MediaType = "image"
	MediaFile  MediaType = "file"
	MediaAudio MediaType = "audio"
)

type Message struct {
	ID               string     `json:"id"`
	GroupID          string     `json:"group_id"`
	SenderID         string     `json:"sender_id"`
	SenderName       string     `json:"sender_name"`
	Text             string     `json:"text"`
	CreatedAt        time.Time  `json:"created_at"`
	EditedAt         *time.Time `json:"edited_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	ReplyToMessageID *string    `json:"reply_to_message_id,omitempty"`
	MediaType        *MediaType `json:"media_type,omitempty"`
	MediaURL         *string    `json:"media_url,omitempty"`
	MediaName        *string    `json:"media_name,omitempty"`
	MediaSizeBytes   *int64     `json:"media_size_bytes,omitempty"`
}

type InviteRequest struct {
	ID           string     `json:"id"`
	GroupID      string     `json:"group_id"`
	GroupTitle   string     `json:"group_title,omitempty"`
	InviterID    string     `json:"inviter_id"`
	InviterName  string     `json:"inviter_name,omitempty"`
	TargetUserID string     `json:"target_user_id"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	RespondedAt  *time.Time `json:"responded_at,omitempty"`
}

type DeviceToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
	Token     string    `json:"token"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Session struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}
