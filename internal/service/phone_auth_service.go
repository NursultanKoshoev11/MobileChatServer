package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/security"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/sms"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

const (
	phoneCodeTTL             = 5 * time.Minute
	phoneCodeMaxAttempts     = 5
	phoneCodeRateLimitWindow = 10 * time.Minute
	phoneCodeRateLimitMax    = 3
	phoneCodeMinRequestGap   = 30 * time.Second
	publicDemoAuthCode       = "123"
	publicDemoDisplayName    = "Koom Test User"
)

type PhoneAuthConfig struct {
	JWTSecret           string
	AccessTokenTTL      time.Duration
	Environment         string
	TestAuthEnabled     bool
	TestAuthPhone       string
	TestAuthCode        string
	TestAuthDisplayName string
}

type PhoneAuthService struct {
	repo   *storage.Repository
	cfg    PhoneAuthConfig
	sender sms.Sender
}

func NewPhoneAuth(repo *storage.Repository, cfg PhoneAuthConfig, sender sms.Sender) *PhoneAuthService {
	return &PhoneAuthService{repo: repo, cfg: cfg, sender: sender}
}

func (s *PhoneAuthService) RequestCode(ctx context.Context, input RequestPhoneCodeInput) (RequestPhoneCodeOutput, error) {
	mobile, err := normalizeMobile(input.Mobile)
	if err != nil {
		return RequestPhoneCodeOutput{}, err
	}

	accountExists := true
	if _, err := s.repo.GetPhoneUserByMobile(ctx, mobile); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			accountExists = false
		} else {
			return RequestPhoneCodeOutput{}, err
		}
	}

	if s.isTestAuthMobile(mobile) {
		return RequestPhoneCodeOutput{Status: "test_code_ready", AccountExists: accountExists}, nil
	}

	if err := s.enforcePhoneCodeRateLimit(ctx, mobile); err != nil {
		return RequestPhoneCodeOutput{}, err
	}

	code, err := newNumericCode(6)
	if err != nil {
		return RequestPhoneCodeOutput{}, err
	}
	codeID := "PC-" + strings.ToUpper(randomHex(12))
	if err := s.repo.CreatePhoneCode(ctx, codeID, mobile, security.HashToken(code), time.Now().UTC().Add(phoneCodeTTL)); err != nil {
		return RequestPhoneCodeOutput{}, err
	}
	if s.sender == nil {
		return RequestPhoneCodeOutput{}, fmt.Errorf("sms sender is not configured")
	}
	if err := s.sender.SendVerificationCode(ctx, mobile, code); err != nil {
		if errors.Is(err, sms.ErrDisabled) {
			return RequestPhoneCodeOutput{}, NewServiceUnavailableError("sms verification is temporarily unavailable")
		}
		return RequestPhoneCodeOutput{}, err
	}
	return RequestPhoneCodeOutput{Status: "code_sent", AccountExists: accountExists}, nil
}

func (s *PhoneAuthService) enforcePhoneCodeRateLimit(ctx context.Context, mobile string) error {
	now := time.Now().UTC()
	count, err := s.repo.CountPhoneCodesSince(ctx, mobile, now.Add(-phoneCodeRateLimitWindow))
	if err != nil {
		return err
	}
	if count >= phoneCodeRateLimitMax {
		return NewValidationError("too many code requests; try again later")
	}
	latest, err := s.repo.LatestPhoneCodeCreatedAt(ctx, mobile)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	if err == nil && now.Sub(latest) < phoneCodeMinRequestGap {
		return NewValidationError("please wait before requesting another code")
	}
	return nil
}

func (s *PhoneAuthService) VerifyCode(ctx context.Context, input VerifyPhoneCodeInput) (domain.PhoneSession, error) {
	mobile, err := normalizeMobile(input.Mobile)
	if err != nil {
		return domain.PhoneSession{}, err
	}
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return domain.PhoneSession{}, NewValidationError("code is required")
	}

	if s.isTestAuthMobile(mobile) {
		if code != s.expectedTestAuthCode(mobile) {
			return domain.PhoneSession{}, ErrInvalidCredentials
		}
		displayName := strings.TrimSpace(input.DisplayName)
		if displayName == "" {
			displayName = s.testAuthDisplayName(mobile)
		}
		user, err := s.getOrCreatePhoneUser(ctx, mobile, displayName)
		if err != nil {
			return domain.PhoneSession{}, err
		}
		_ = s.repo.MarkPhoneVerified(ctx, user.ID)
		if err := s.repo.UpsertUserRoleFromAllowlist(ctx, user.ID, mobile); err != nil {
			return domain.PhoneSession{}, err
		}
		return s.issuePhoneSession(ctx, user)
	}

	if len(code) != 6 {
		return domain.PhoneSession{}, NewValidationError("code must contain 6 digits")
	}
	record, err := s.repo.GetLatestPhoneCode(ctx, mobile)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.PhoneSession{}, ErrInvalidCredentials
		}
		return domain.PhoneSession{}, err
	}
	if record.ConsumedAt != nil || time.Now().UTC().After(record.ExpiresAt) || record.Attempts >= phoneCodeMaxAttempts {
		return domain.PhoneSession{}, ErrInvalidCredentials
	}
	if security.HashToken(code) != record.CodeHash {
		_ = s.repo.IncrementPhoneCodeAttempts(ctx, record.ID)
		return domain.PhoneSession{}, ErrInvalidCredentials
	}
	if err := s.repo.ConsumePhoneCode(ctx, record.ID); err != nil {
		return domain.PhoneSession{}, err
	}

	user, err := s.getOrCreatePhoneUser(ctx, mobile, input.DisplayName)
	if err != nil {
		return domain.PhoneSession{}, err
	}
	_ = s.repo.MarkPhoneVerified(ctx, user.ID)
	if err := s.repo.UpsertUserRoleFromAllowlist(ctx, user.ID, mobile); err != nil {
		return domain.PhoneSession{}, err
	}
	return s.issuePhoneSession(ctx, user)
}

func (s *PhoneAuthService) Refresh(ctx context.Context, input RefreshInput) (domain.PhoneSession, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return domain.PhoneSession{}, NewValidationError("refresh_token is required")
	}
	record, err := s.repo.GetRefreshSession(ctx, security.HashToken(refreshToken))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.PhoneSession{}, ErrUnauthorized
		}
		return domain.PhoneSession{}, err
	}
	if record.RevokedAt != nil || time.Now().UTC().After(record.ExpiresAt) {
		return domain.PhoneSession{}, ErrUnauthorized
	}
	user, err := s.repo.GetAuthPhoneUserByID(ctx, record.UserID)
	if err != nil {
		return domain.PhoneSession{}, err
	}
	accessToken, err := security.SignAccessToken(user.ID, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		return domain.PhoneSession{}, err
	}
	return domain.PhoneSession{AccessToken: accessToken, RefreshToken: refreshToken, User: user}, nil
}

func (s *PhoneAuthService) Logout(ctx context.Context, input RefreshInput) error {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return NewValidationError("refresh_token is required")
	}
	record, err := s.repo.GetRefreshSession(ctx, security.HashToken(refreshToken))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return err
	}
	if record.RevokedAt != nil {
		return nil
	}
	return s.repo.RevokeRefreshSession(ctx, record.ID)
}

func (s *PhoneAuthService) getOrCreatePhoneUser(ctx context.Context, mobile string, displayNameInput string) (domain.PhoneAuthUser, error) {
	user, err := s.repo.GetPhoneUserByMobile(ctx, mobile)
	if errors.Is(err, storage.ErrNotFound) {
		displayName := strings.TrimSpace(displayNameInput)
		if displayName == "" {
			return domain.PhoneAuthUser{}, NewValidationError("display_name is required for new account")
		}
		if len(displayName) < 2 || len(displayName) > maxDisplayNameLen {
			return domain.PhoneAuthUser{}, NewValidationError(fmt.Sprintf("display_name must be between 2 and %d characters", maxDisplayNameLen))
		}
		return s.repo.CreatePhoneUser(ctx, domain.PhoneAuthUser{
			ID:          "U-" + strings.ToUpper(randomHex(8)),
			Mobile:      mobile,
			DisplayName: displayName,
			Role:        domain.UserRoleUser,
		})
	}
	if err != nil {
		return domain.PhoneAuthUser{}, err
	}
	return user, nil
}

func (s *PhoneAuthService) issuePhoneSession(ctx context.Context, user domain.PhoneAuthUser) (domain.PhoneSession, error) {
	if storedUser, err := s.repo.GetUserByID(ctx, user.ID); err == nil {
		user.Role = storedUser.Role
		if user.DisplayName == "" {
			user.DisplayName = storedUser.DisplayName
		}
		if user.Mobile == "" {
			user.Mobile = storedUser.Phone
		}
	}
	if user.Role == "" {
		user.Role = domain.UserRoleUser
	}
	accessToken, err := security.SignAccessToken(user.ID, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		return domain.PhoneSession{}, err
	}
	refreshToken, err := security.NewOpaqueToken(48)
	if err != nil {
		return domain.PhoneSession{}, err
	}
	refreshSessionID := "RT-" + strings.ToUpper(randomHex(12))
	if err := s.repo.CreateRefreshSession(ctx, refreshSessionID, user.ID, security.HashToken(refreshToken), time.Now().UTC().Add(refreshTokenTTL)); err != nil {
		return domain.PhoneSession{}, err
	}
	return domain.PhoneSession{AccessToken: accessToken, RefreshToken: refreshToken, User: user}, nil
}

func (s *PhoneAuthService) isTestAuthMobile(mobile string) bool {
	if s.cfg.Environment != "production" {
		return true
	}
	if !s.cfg.TestAuthEnabled {
		return false
	}
	testPhone := normalizeTestValue(s.cfg.TestAuthPhone)
	if testPhone == "*" || strings.EqualFold(testPhone, "any") || strings.EqualFold(testPhone, "all") {
		return true
	}
	return normalizeTestValue(mobile) == testPhone
}

func (s *PhoneAuthService) expectedTestAuthCode(mobile string) string {
	if s.cfg.Environment != "production" {
		return publicDemoAuthCode
	}
	return strings.TrimSpace(s.cfg.TestAuthCode)
}

func (s *PhoneAuthService) testAuthDisplayName(mobile string) string {
	if s.cfg.Environment != "production" {
		return publicDemoDisplayName
	}
	return strings.TrimSpace(s.cfg.TestAuthDisplayName)
}


func normalizeTestValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "(", "")
	value = strings.ReplaceAll(value, ")", "")
	return value
}
