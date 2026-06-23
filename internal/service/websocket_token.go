package service

import (
	"context"
	"errors"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/security"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

const webSocketTokenTTL = 5 * time.Minute

func (s *Service) IssueWebSocketToken(userID string) (string, error) {
	if userID == "" {
		return "", ErrUnauthorized
	}
	return security.SignWebSocketToken(userID, s.cfg.JWTSecret, webSocketTokenTTL)
}

func (s *Service) AuthenticateWebSocket(ctx context.Context, token string) (domain.User, error) {
	claims, err := security.ParseWebSocketToken(token, s.cfg.JWTSecret)
	if err != nil {
		return domain.User{}, ErrUnauthorized
	}
	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.User{}, ErrUnauthorized
		}
		return domain.User{}, err
	}
	return user, nil
}
