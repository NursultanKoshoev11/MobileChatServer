package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

type SetGroupCommentMuteInput struct {
	Phone           string `json:"phone"`
	Mobile          string `json:"mobile"`
	DurationMinutes int    `json:"duration_minutes"`
	Reason          string `json:"reason"`
}

type ClearGroupCommentMuteInput struct {
	Phone  string `json:"phone"`
	Mobile string `json:"mobile"`
}

func (s *Service) SetGroupCommentMute(ctx context.Context, actorID, groupID, targetUserID string, input SetGroupCommentMuteInput) (domain.GroupCommentMute, error) {
	groupID = strings.TrimSpace(groupID)
	targetUserID = strings.TrimSpace(targetUserID)
	if groupID == "" {
		return domain.GroupCommentMute{}, NewValidationError("group_id is required")
	}
	if targetUserID == "" {
		return domain.GroupCommentMute{}, NewValidationError("user_id is required")
	}
	if input.DurationMinutes < 0 {
		return domain.GroupCommentMute{}, NewValidationError("duration_minutes must be zero or positive")
	}
	var mutedUntil *time.Time
	if input.DurationMinutes > 0 {
		until := time.Now().UTC().Add(time.Duration(input.DurationMinutes) * time.Minute)
		mutedUntil = &until
	}
	mute, err := s.repo.SetGroupCommentMute(ctx, groupID, actorID, targetUserID, mutedUntil, strings.TrimSpace(input.Reason))
	if err != nil {
		return domain.GroupCommentMute{}, err
	}
	s.RecordEvent(ctx, actorID, "group_comment_muted", "group", groupID)
	return mute, nil
}

func (s *Service) ClearGroupCommentMute(ctx context.Context, actorID, groupID, targetUserID string) error {
	groupID = strings.TrimSpace(groupID)
	targetUserID = strings.TrimSpace(targetUserID)
	if groupID == "" {
		return NewValidationError("group_id is required")
	}
	if targetUserID == "" {
		return NewValidationError("user_id is required")
	}
	if err := s.repo.ClearGroupCommentMute(ctx, groupID, actorID, targetUserID); err != nil {
		return err
	}
	s.RecordEvent(ctx, actorID, "group_comment_unmuted", "group", groupID)
	return nil
}

func (s *Service) SetGroupCommentMuteByPhone(ctx context.Context, actorID, groupID string, input SetGroupCommentMuteInput) (domain.GroupCommentMute, error) {
	groupID = strings.TrimSpace(groupID)
	phone := firstNonEmptyService(input.Phone, input.Mobile)
	phone = normalizePhone(phone)
	if groupID == "" {
		return domain.GroupCommentMute{}, NewValidationError("group_id is required")
	}
	if phone == "" {
		return domain.GroupCommentMute{}, NewValidationError("phone is required")
	}
	if input.DurationMinutes < 0 {
		return domain.GroupCommentMute{}, NewValidationError("duration_minutes must be zero or positive")
	}
	user, err := s.FindUserByPhone(ctx, phone)
	if err != nil {
		return domain.GroupCommentMute{}, err
	}
	var mutedUntil *time.Time
	if input.DurationMinutes > 0 {
		until := time.Now().UTC().Add(time.Duration(input.DurationMinutes) * time.Minute)
		mutedUntil = &until
	}
	mute, err := s.repo.SetGroupCommentMute(ctx, groupID, actorID, user.ID, mutedUntil, strings.TrimSpace(input.Reason))
	if err != nil {
		return domain.GroupCommentMute{}, err
	}
	s.RecordEvent(ctx, actorID, "group_comment_muted", "group", groupID)
	return mute, nil
}

func (s *Service) ClearGroupCommentMuteByPhone(ctx context.Context, actorID, groupID string, input ClearGroupCommentMuteInput) error {
	groupID = strings.TrimSpace(groupID)
	phone := firstNonEmptyService(input.Phone, input.Mobile)
	phone = normalizePhone(phone)
	if groupID == "" {
		return NewValidationError("group_id is required")
	}
	if phone == "" {
		return NewValidationError("phone is required")
	}
	user, err := s.FindUserByPhone(ctx, phone)
	if err != nil {
		return err
	}
	if err := s.repo.ClearGroupCommentMute(ctx, groupID, actorID, user.ID); err != nil {
		return err
	}
	s.RecordEvent(ctx, actorID, "group_comment_unmuted", "group", groupID)
	return nil
}

func (s *Service) ensureCanCommentPublicRequest(ctx context.Context, userID, requestID string) error {
	mute, active, err := s.repo.GetActiveCommentMuteForRequest(ctx, requestID, userID)
	if err != nil {
		return err
	}
	if !active {
		return nil
	}
	if mute.MutedUntil != nil {
		return NewValidationError(fmt.Sprintf("comments are blocked until %s", mute.MutedUntil.Format(time.RFC3339)))
	}
	return NewValidationError("comments are blocked")
}

func firstNonEmptyService(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
