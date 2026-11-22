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
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return NewValidationError("group_id is required")
	}
	return s.recordAuditedMutation(ctx, userID, "group_joined_public", "group", groupID, func(repo *storage.Repository) error {
		return repo.JoinPublicGroup(ctx, groupID, userID)
	})
}

func (s *Service) UpdateGroupMemberRole(ctx context.Context, actorID, groupID, targetUserID string, role domain.GroupRole) (domain.GroupMember, error) {
	if groupID == "" || targetUserID == "" {
		return domain.GroupMember{}, NewValidationError("group_id and user_id are required")
	}
	if role != domain.RoleAdmin && role != domain.RoleMember {
		return domain.GroupMember{}, NewValidationError("role must be admin or member")
	}
	eventType := "member_demoted"
	if role == domain.RoleAdmin {
		eventType = "member_promoted"
	}
	if err := s.recordAuditedMutation(ctx, actorID, eventType, "group", groupID, func(repo *storage.Repository) error {
		return repo.SetMemberRole(ctx, groupID, actorID, targetUserID, role)
	}); err != nil {
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
	// Only the owner may change roles (UpdateGroupMemberRole enforces the same),
	// and the phone is resolved only among the group's members.
	user, err := s.resolveGroupMemberByPhone(ctx, actorID, groupID, phone, domain.RoleOwner)
	if err != nil {
		return domain.GroupMember{}, err
	}
	return s.UpdateGroupMemberRole(ctx, actorID, groupID, user.ID, role)
}

// EnsureGroupInviteCode returns the group's invite code (creating one if the
// group has none). Only the group owner and admins may obtain it.
func (s *Service) EnsureGroupInviteCode(ctx context.Context, userID, groupID string) (domain.Group, error) {
	return s.issueGroupInviteCode(ctx, userID, groupID, false)
}

// RotateGroupInviteCode replaces the group's invite code with a new one; the
// old code (and QR passes that carry it) stops working. Owner/admin only.
func (s *Service) RotateGroupInviteCode(ctx context.Context, userID, groupID string) (domain.Group, error) {
	return s.issueGroupInviteCode(ctx, userID, groupID, true)
}

func (s *Service) issueGroupInviteCode(ctx context.Context, userID, groupID string, rotate bool) (domain.Group, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return domain.Group{}, NewValidationError("group_id is required")
	}
	eventType := "group_invite_code_accessed"
	if rotate {
		eventType = "group_invite_code_rotated"
	}
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		var group domain.Group
		err := s.recordAuditedMutation(ctx, userID, eventType, "group", groupID, func(repo *storage.Repository) error {
			var ensureErr error
			group, ensureErr = repo.EnsureGroupInviteCodeInTransaction(ctx, groupID, userID, randomInviteCode(), rotate)
			return ensureErr
		})
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
	if len(inviteCode) > maxInviteCodeInputLen {
		return domain.Group{}, storage.ErrNotFound
	}
	groupID, err := s.repo.FindGroupIDByInviteCode(ctx, inviteCode)
	if err != nil {
		return domain.Group{}, err
	}
	var group domain.Group
	err = s.recordAuditedMutation(ctx, userID, "group_joined_by_invite_code", "group", groupID, func(repo *storage.Repository) error {
		var joinErr error
		group, joinErr = repo.JoinByInviteCode(ctx, userID, inviteCode)
		return joinErr
	})
	if err != nil {
		return domain.Group{}, err
	}
	// The joiner is a plain member, who may not see the invite code.
	group.InviteCode = ""
	return group, nil
}

func (s *Service) InviteUserByID(ctx context.Context, adminID, groupID, targetUserID string) error {
	if groupID == "" || targetUserID == "" {
		return NewValidationError("group_id and target_user_id are required")
	}
	return s.recordAuditedMutation(ctx, adminID, "group_member_added_by_admin", "group", groupID, func(repo *storage.Repository) error {
		return repo.InviteUserByID(ctx, groupID, adminID, targetUserID)
	})
}

func (s *Service) ListMessages(ctx context.Context, userID, groupID string, limit int, before time.Time, beforeIDs ...string) ([]domain.Message, error) {
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	return s.repo.ListMessages(ctx, groupID, userID, limit, before, beforeIDs...)
}

func (s *Service) ListMessagesAfter(ctx context.Context, userID, groupID string, limit int, after time.Time, afterID string) ([]domain.Message, error) {
	if groupID == "" {
		return nil, NewValidationError("group_id is required")
	}
	if after.IsZero() || strings.TrimSpace(afterID) == "" {
		return nil, NewValidationError("after cursor is required")
	}
	return s.repo.ListMessagesAfter(ctx, groupID, userID, limit, after, afterID)
}

func (s *Service) SendMessage(ctx context.Context, senderID, groupID string, input SendMessageInput) (domain.Message, error) {
	return s.createMessage(ctx, senderID, groupID, input, true)
}

func (s *Service) createMessage(ctx context.Context, senderID, groupID string, input SendMessageInput, runModeration bool) (domain.Message, error) {
	text := strings.TrimSpace(input.Text)
	if groupID == "" {
		return domain.Message{}, NewValidationError("group_id is required")
	}
	role, err := s.repo.GetMemberRole(ctx, groupID, senderID)
	if err != nil {
		return domain.Message{}, err
	}
	if !canPublishOfficialMessage(role) {
		return domain.Message{}, storage.ErrForbidden
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
	var created domain.Message
	_, err = s.persistModeratedPublication(ctx, moderationNotice, func(repo *storage.Repository) (string, error) {
		var createErr error
		created, createErr = repo.CreateMessage(ctx, message)
		return created.ID, createErr
	})
	if err != nil {
		return domain.Message{}, err
	}
	return created, nil
}

func canPublishOfficialMessage(role domain.GroupRole) bool {
	return role == domain.RoleOwner || role == domain.RoleAdmin
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

// maxInviteCodeInputLen bounds the normalized invite code accepted for a
// lookup; real codes are 6 (legacy AAA-999) or 10 characters.
const maxInviteCodeInputLen = 32

// normalizeInviteCode uppercases the code and drops separators (dashes,
// spaces and any other non-alphanumeric characters). The result is the
// compact form matched against the dash-stripped, upper-cased expression
// indexed by migration 0026, so both legacy
// AAA-999 codes and new XXXXX-XXXXX codes are found by an index lookup.
func normalizeInviteCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	var compact strings.Builder
	compact.Grow(len(code))
	for _, char := range code {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			compact.WriteRune(char)
		}
	}
	return compact.String()
}

func randomHex(bytesCount int) string {
	buf := make([]byte, bytesCount)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return hex.EncodeToString(buf)
}

// inviteCodeAlphabet omits characters that are easy to confuse when read
// aloud or typed: 0/O, 1/I/L and U/V.
const inviteCodeAlphabet = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

// randomInviteCode returns a new 10-character code (about 49 bits from
// crypto/rand) displayed and stored as XXXXX-XXXXX.
func randomInviteCode() string {
	code := randomChars(inviteCodeAlphabet, 10)
	return code[:5] + "-" + code[5:]
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
