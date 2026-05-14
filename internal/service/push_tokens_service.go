package service

import (
	"context"
	"strings"
)

type RegisterPushTokenInput struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (s *Service) RegisterPushToken(ctx context.Context, userID string, input RegisterPushTokenInput) error {
	token := strings.TrimSpace(input.Token)
	platform := strings.TrimSpace(input.Platform)
	if token == "" {
		return NewValidationError("token is required")
	}
	if platform == "" {
		platform = "unknown"
	}
	return s.repo.UpsertPushToken(ctx, userID, token, platform)
}

func (s *Service) DeletePushToken(ctx context.Context, userID string, input RegisterPushTokenInput) error {
	token := strings.TrimSpace(input.Token)
	if token == "" {
		return NewValidationError("token is required")
	}
	return s.repo.DeletePushToken(ctx, userID, token)
}
