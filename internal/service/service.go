package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
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
	repo     *storage.Repository
	cfg      Config
	notifier Notifier
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

	user := domain.User{
		ID:          "U-" + strings.ToUpper(randomHex(8)),
		Email:       email,
		DisplayName: displayName,
	}
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
	if err := s.repo.RevokeRefreshSession(ctx, record.ID); err != nil {
		return domain.Session{}, err
	}
	user, err := s.repo.GetUserByID(ctx, record.UserID)
	if err != nil {
		return domain.Session{}, err
	}
	return s.issueSession(ctx, user)
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
	groups, err := s.repo.ListUserGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	for index := range groups {
		if strings.TrimSpace(groups[index].InviteCode) != "" {
			continue
		}
		group, err := s.EnsureGroupInviteCode(ctx, userID, groups[index].ID)
		if err != nil {
			return nil, err
		}
		groups[index].InviteCode = group.InviteCode
		groups[index].MemberCount = group.MemberCount
		groups[index].MyRole = group.MyRole
	}
	return groups, nil
}

func (s *Service) SearchPublicGroups(ctx context.Context, query string) ([]domain.Group, error) {
	return s.repo.SearchPublicGroups(ctx, query)
}

func (s *Service) CreateGroup(ctx context.Context, ownerID string, input CreateGroupInput) (domain.Group, error) {
	title := strings.TrimSpace(input.Title)
	description := strings.TrimSpace(input.Description)
	if len(title) < 3 || len(title) > maxGroupTitleLen {
		return domain.Group{}, NewValidationError(fmt.Sprintf("title must be between 3 and %d characters", maxGroupTitleLen))
	}
	if len(description) > maxDescriptionLen {
		return domain.Group{}, NewValidationError(fmt.Sprintf("description must be at most %d characters", maxDescriptionLen))
	}
	if input.Visibility != domain.VisibilityPublic && input.Visibility != domain.VisibilityPrivate {
		return domain.Group{}, NewValidationError("visibility must be public or private")
	}
	group := domain.Group{
		ID:          "G-" + strings.ToUpper(randomHex(8)),
		Title:       title,
		Description: description,
		Visibility:  input.Visibility,
		OwnerID:     ownerID,
		InviteCode:  randomInviteCode(),
	}
	return s.repo.CreateGroup(ctx, group)
}

func (s *Service) JoinPublicGroup(ctx context.Context, userID, groupID string) error {
	if groupID == "" {
		return NewValidationError("group_id is required")
	}
	return s.repo.JoinPublicGroup(ctx, groupID, userID)
}

func (s *Service) ListGroupMembers(ctx context.Context, userID, groupID string) ([]domain.GroupMember, error) {
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	return s.repo.ListGroupMembers(ctx, groupID, userID)
}

func (s *Service) UpdateGroupMemberRole(ctx context.Context, actorID, groupID, targetUserID string, role domain.GroupRole) (domain.GroupMember, error) {
	if groupID == "" || targetUserID == "" {
		return domain.GroupMember{}, NewValidationError("group_id and user_id are required")
	}
	if role != domain.RoleAdmin && role != domain.RoleMember {
		return domain.GroupMember{}, NewValidationError("role must be admin or member")
	}
	return s.repo.UpdateGroupMemberRole(ctx, groupID, actorID, targetUserID, role)
}

func (s *Service) UpdateGroupMemberRoleByPhone(ctx context.Context, actorID, groupID, phone string, role domain.GroupRole) (domain.GroupMember, error) {
	phone = normalizePhone(phone)
	if phone == "" {
		return domain.GroupMember{}, NewValidationError("phone is required")
	}
	user, err := s.FindUserByPhone(ctx, phone)
	if err != nil {
		return domain.GroupMember{}, err
	}
	return s.UpdateGroupMemberRole(ctx, actorID, groupID, user.ID, role)
}

func (s *Service) EnsureGroupInviteCode(ctx context.Context, userID, groupID string) (domain.Group, error) {
	if groupID == "" {
		return domain.Group{}, NewValidationError("group_id is required")
	}
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		group, err := s.repo.EnsureGroupInviteCode(ctx, groupID, userID, randomInviteCode())
		if err == nil {
			return group, nil
		}
		lastErr = err
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return domain.Group{}, err
		}
	}
	return domain.Group{}, fmt.Errorf("generate group invite code: %w", lastErr)
}

func (s *Service) JoinByInviteCode(ctx context.Context, userID, inviteCode string) (domain.Group, error) {
	inviteCode = normalizeInviteCode(inviteCode)
	if inviteCode == "" {
		return domain.Group{}, NewValidationError("invite_code is required")
	}
	return s.repo.JoinByInviteCode(ctx, userID, inviteCode)
}

func (s *Service) FindUserByPhone(ctx context.Context, mobile string) (domain.User, error) {
	mobile = normalizePhone(mobile)
	if mobile == "" {
		return domain.User{}, NewValidationError("mobile is required")
	}
	return s.repo.GetUserByPhone(ctx, mobile)
}

func (s *Service) InviteUserByID(ctx context.Context, adminID, groupID, targetUserID string) error {
	if groupID == "" || targetUserID == "" {
		return NewValidationError("group_id and target_user_id are required")
	}
	return s.repo.InviteUserByID(ctx, groupID, adminID, targetUserID)
}

func (s *Service) ListMessages(ctx context.Context, userID, groupID string, limit int, before time.Time) ([]domain.Message, error) {
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	return s.repo.ListMessages(ctx, groupID, userID, limit, before)
}

func (s *Service) SendMessage(ctx context.Context, senderID, groupID string, input SendMessageInput) (domain.Message, error) {
	text := strings.TrimSpace(input.Text)
	if groupID == "" {
		return domain.Message{}, NewValidationError("group_id is required")
	}
	role, err := s.repo.GetMemberRole(ctx, groupID, senderID)
	if err != nil {
		return domain.Message{}, err
	}
	if role != domain.RoleOwner && role != domain.RoleAdmin {
		return domain.Message{}, storage.ErrForbidden
	}
	if text == "" {
		return domain.Message{}, NewValidationError("text is required")
	}
	if len(text) > maxMessageLen {
		return domain.Message{}, NewValidationError(fmt.Sprintf("text must be at most %d characters", maxMessageLen))
	}
	message := domain.Message{
		ID:       "M-" + strings.ToUpper(randomHex(12)),
		GroupID:  groupID,
		SenderID: senderID,
		Text:     text,
	}
	return s.repo.CreateMessage(ctx, message)
}

func (s *Service) issueSession(ctx context.Context, user domain.User) (domain.Session, error) {
	accessToken, err := security.SignAccessToken(user.ID, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		return domain.Session{}, err
	}
	refreshToken, err := security.NewOpaqueToken(48)
	if err != nil {
		return domain.Session{}, err
	}
	refreshSessionID := "RT-" + strings.ToUpper(randomHex(12))
	if err := s.repo.CreateRefreshSession(ctx, refreshSessionID, user.ID, security.HashToken(refreshToken), time.Now().UTC().Add(refreshTokenTTL)); err != nil {
		return domain.Session{}, err
	}
	return domain.Session{AccessToken: accessToken, RefreshToken: refreshToken, User: user}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	return phone
}

func normalizeInviteCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	var compact strings.Builder
	compact.Grow(6)
	for _, char := range code {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			compact.WriteRune(char)
		}
	}
	value := compact.String()
	if len(value) >= 6 {
		return value[:3] + "-" + value[3:6]
	}
	return value
}

func randomHex(bytesCount int) string {
	buf := make([]byte, bytesCount)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return hex.EncodeToString(buf)
}

func randomInviteCode() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const digits = "23456789"
	return randomChars(letters, 3) + "-" + randomChars(digits, 3)
}

func randomChars(alphabet string, count int) string {
	if count <= 0 || alphabet == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(count)
	max := big.NewInt(int64(len(alphabet)))
	for builder.Len() < count {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			fallback := strings.ToUpper(randomHex(8))
			for _, char := range fallback {
				if strings.ContainsRune(alphabet, char) {
					builder.WriteRune(char)
					if builder.Len() == count {
						break
					}
				}
			}
			continue
		}
		builder.WriteByte(alphabet[index.Int64()])
	}
	return builder.String()
}

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func NewValidationError(message string) ValidationError {
	return ValidationError{Message: message}
}

var (
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidCredentials = errors.New("invalid email or password")
)
