package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/security"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

func (s *Service) JoinPublicGroup(ctx context.Context, userID, groupID string) error {
	if groupID == "" {
		return NewValidationError("group_id is required")
	}
	return s.repo.JoinPublicGroup(ctx, groupID, userID)
}

func (s *Service) UpdateGroupMemberRole(ctx context.Context, actorID, groupID, targetUserID string, role domain.GroupRole) (domain.GroupMember, error) {
	if groupID == "" || targetUserID == "" {
		return domain.GroupMember{}, NewValidationError("group_id and user_id are required")
	}
	if role != domain.RoleAdmin && role != domain.RoleMember {
		return domain.GroupMember{}, NewValidationError("role must be admin or member")
	}
	if err := s.repo.SetMemberRole(ctx, groupID, actorID, targetUserID, role); err != nil {
		return domain.GroupMember{}, err
	}
	members, err := s.repo.ListGroupMembers(ctx, groupID, actorID)
	if err != nil {
		return domain.GroupMember{}, err
	}
	for _, member := range members {
		if member.UserID == targetUserID {
			return member, nil
		}
	}
	return domain.GroupMember{}, storage.ErrNotFound
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
			if pass, passErr := security.MakeQREnv(group.ID, group.InviteCode, 24*time.Hour); passErr == nil {
				group.QRPass = pass
			}
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
	inviteCode = strings.TrimSpace(inviteCode)
	if strings.HasPrefix(inviteCode, security.SignedInvitePrefix+".") {
		claims, err := security.ReadQR(inviteCode, s.cfg.JWTSecret)
		if err != nil {
			return domain.Group{}, err
		}
		inviteCode = claims.Code
	}
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
	return s.createMessage(ctx, senderID, groupID, input, true)
}

func (s *Service) createMessage(ctx context.Context, senderID, groupID string, input SendMessageInput, runModeration bool) (domain.Message, error) {
	text := strings.TrimSpace(input.Text)
	if groupID == "" {
		return domain.Message{}, NewValidationError("group_id is required")
	}
	if _, err := s.repo.GetMemberRole(ctx, groupID, senderID); err != nil {
		return domain.Message{}, err
	}
	if text == "" {
		return domain.Message{}, NewValidationError("text is required")
	}
	if len(text) > maxMessageLen {
		return domain.Message{}, NewValidationError(fmt.Sprintf("text must be at most %d characters", maxMessageLen))
	}
	var moderationNotice *ContentModerationReviewNotice
	if runModeration {
		notice, err := s.moderateContent(ctx, domain.ContentModerationItem{GroupID: groupID, ContentType: domain.ContentTypeGroupMessage, AuthorID: senderID, Body: text})
		if err != nil {
			return domain.Message{}, err
		}
		moderationNotice = notice
	}
	message := domain.Message{ID: "M-" + strings.ToUpper(randomHex(12)), GroupID: groupID, SenderID: senderID, Text: text}
	created, err := s.repo.CreateMessage(ctx, message)
	if err != nil {
		return domain.Message{}, err
	}
	s.publishModerationReviewNotice(ctx, moderationNotice, created.ID)
	return created, nil
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

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	if phone == "" {
		return ""
	}
	if strings.HasPrefix(phone, "00") && len(phone) > 2 {
		phone = "+" + strings.TrimPrefix(phone, "00")
	}
	if strings.HasPrefix(phone, "+") {
		return phone
	}
	if strings.HasPrefix(phone, "996") {
		return "+" + phone
	}
	if strings.HasPrefix(phone, "0") && len(phone) == 10 {
		return "+996" + strings.TrimPrefix(phone, "0")
	}
	if len(phone) == 9 {
		return "+996" + phone
	}
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

type ValidationError struct{ Message string }

func (e ValidationError) Error() string                 { return e.Message }
func NewValidationError(message string) ValidationError { return ValidationError{Message: message} }

var (
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidCredentials = errors.New("invalid email or password")
)
