package service

import (
	"context"
	"strings"
)

const maxPushTokenLen = 4096

type RegisterPushTokenInput struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (s *Service) RegisterPushToken(ctx context.Context, userID string, input RegisterPushTokenInput) error {
	token := strings.TrimSpace(input.Token)
	platform := normalizePushPlatform(input.Platform)
	if token == "" {
		return NewValidationError("token is required")
	}
	if len(token) > maxPushTokenLen {
		return NewValidationError("token is too long")
	}
	return s.repo.UpsertPushToken(ctx, userID, token, platform)
}

func (s *Service) DeletePushToken(ctx context.Context, userID string, input RegisterPushTokenInput) error {
	token := strings.TrimSpace(input.Token)
	if token == "" {
		return NewValidationError("token is required")
	}
	if len(token) > maxPushTokenLen {
		return NewValidationError("token is too long")
	}
	return s.repo.DeletePushToken(ctx, userID, token)
}

func normalizePushPlatform(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "android":
		return "android"
	case "ios":
		return "ios"
	default:
		return "unknown"
	}
}
