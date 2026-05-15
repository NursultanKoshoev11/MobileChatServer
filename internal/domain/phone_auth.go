package domain

import "time"

type PhoneAuthUser struct {
	ID          string    `json:"id"`
	Mobile      string    `json:"mobile"`
	DisplayName string    `json:"display_name"`
	Role        UserRole  `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

type PhoneSession struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	User         PhoneAuthUser `json:"user"`
}
