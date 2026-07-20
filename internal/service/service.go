package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/moderation"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/security"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

const (
	maxDisplayNameLen = 40
	maxGroupTitleLen  = 80
	maxDescriptionLen = 240
	maxMessageLen     = 2000
	minPasswordLen    = 8
	refreshTokenTTL   = 30 * 24 * time.Hour
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type Config struct {
	JWTSecret      string
	AccessTokenTTL time.Duration
	BCryptCost     int
}

type Service struct {
	repo             *storage.Repository
	cfg              Config
	notifier         Notifier
	contentModerator moderation.Moderator
}

func New(repo *storage.Repository, cfg Config, notifier ...Notifier) *Service {
	selectedNotifier := Notifier(NoopNotifier{})
	if len(notifier) > 0 && notifier[0] != nil {
		selectedNotifier = notifier[0]
	}
	return &Service{repo: repo, cfg: cfg, notifier: selectedNotifier}
}

type RegisterInput struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

type CreateGroupInput struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Visibility  domain.GroupVisibility `json:"visibility"`
}

type SendMessageInput struct {
	Text string `json:"text"`
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (domain.Session, error) {
	email := normalizeEmail(input.Email)
	displayName := strings.TrimSpace(input.DisplayName)
	password := input.Password

	if !emailPattern.MatchString(email) {
		return domain.Session{}, NewValidationError("valid email is required")
	}
	if len(displayName) < 2 || len(displayName) > maxDisplayNameLen {
		return domain.Session{}, NewValidationError(fmt.Sprintf("display_name must be between 2 and %d characters", maxDisplayNameLen))
	}
	if len(password) < minPasswordLen {
		return domain.Session{}, NewValidationError(fmt.Sprintf("password must be at least %d characters", minPasswordLen))
	}

	hash, err := security.HashPassword(password, s.cfg.BCryptCost)
	if err != nil {
		return domain.Session{}, fmt.Errorf("hash password: %w", err)
	}

	user := domain.User{ID: "U-" + strings.ToUpper(randomHex(8)), Email: email, DisplayName: displayName}
	user, err = s.repo.CreateUser(ctx, user, hash)
	if err != nil {
		return domain.Session{}, err
	}
	return s.issueSession(ctx, user)
}

func (s *Service) Login(ctx context.Context, input LoginInput) (domain.Session, error) {
	email := normalizeEmail(input.Email)
	if !emailPattern.MatchString(email) || input.Password == "" {
		return domain.Session{}, NewValidationError("email and password are required")
	}
	record, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.Session{}, ErrInvalidCredentials
		}
		return domain.Session{}, err
	}
	if !security.CheckPassword(record.PasswordHash, input.Password) {
		return domain.Session{}, ErrInvalidCredentials
	}
	return s.issueSession(ctx, record.User)
}

func (s *Service) RefreshSession(ctx context.Context, input RefreshInput) (domain.Session, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return domain.Session{}, NewValidationError("refresh_token is required")
	}
	record, err := s.repo.GetRefreshSession(ctx, security.HashToken(refreshToken))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.Session{}, ErrUnauthorized
		}
		return domain.Session{}, err
	}
	if record.RevokedAt != nil || time.Now().UTC().After(record.ExpiresAt) {
		return domain.Session{}, ErrUnauthorized
	}
	user, err := s.repo.GetUserByID(ctx, record.UserID)
	if err != nil {
		return domain.Session{}, err
	}
	accessToken, err := security.SignAccessToken(user.ID, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		return domain.Session{}, err
	}
	newRefreshToken, err := security.NewOpaqueToken(48)
	if err != nil {
		return domain.Session{}, err
	}
	newRefreshSessionID := "RT-" + strings.ToUpper(randomHex(12))
	if err := s.repo.RotateRefreshSession(ctx, record.ID, newRefreshSessionID, user.ID, security.HashToken(newRefreshToken), time.Now().UTC().Add(refreshTokenTTL)); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.Session{}, ErrUnauthorized
		}
		return domain.Session{}, err
	}
	return domain.Session{AccessToken: accessToken, RefreshToken: newRefreshToken, User: user}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (domain.User, error) {
	claims, err := security.ParseAccessToken(token, s.cfg.JWTSecret)
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

func (s *Service) ListUserGroups(ctx context.Context, userID string) ([]domain.Group, error) {
	return s.repo.ListUserGroups(ctx, userID)
}

func (s *Service) SearchPublicGroups(ctx context.Context, query string) ([]domain.Group, error) {
	return s.repo.SearchPublicGroups(ctx, query)
}

func (s *Service) CreateGroup(ctx context.Context, ownerID string, input CreateGroupInput) (domain.Group, error) {
	owner, err := s.repo.GetUserByID(ctx, strings.TrimSpace(ownerID))
	if err != nil {
		return domain.Group{}, err
	}
	if !owner.Role.CanManageAllGroups() {
		return domain.Group{}, storage.ErrForbidden
	}

	title := strings.TrimSpace(input.Title)
	description := strings.TrimSpace(input.Description)
	if len(title) < 3 || len(title) > maxGroupTitleLen {
		return domain.Group{}, NewValidationError(fmt.Sprintf("title must be between 3 and %d characters", maxGroupTitleLen))
	}
	if len(description) > maxDescriptionLen {
		return domain.Group{}, NewValidationError(fmt.Sprintf("description must be at most %d characters", maxDescriptionLen))
	}
	visibility := input.Visibility
	if visibility == "" {
		visibility = domain.VisibilityPrivate
	}
	if visibility != domain.VisibilityPrivate && visibility != domain.VisibilityPublic {
		return domain.Group{}, NewValidationError("visibility must be private or public")
	}
	return s.repo.CreateGroup(ctx, domain.Group{ID: "G-" + strings.ToUpper(randomHex(8)), OwnerID: ownerID, Title: title, Description: description, Visibility: visibility})
}
