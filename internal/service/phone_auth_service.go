package service

import (
	"context"
	"errors"
	"fmt"
	"log"
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

	// SMS spend guards (toll-fraud / SMS-pumping protection).
	phoneCodeDailyWindow            = 24 * time.Hour
	phoneCodeDailyMax               = 8
	phoneCodeGlobalWindow           = time.Hour
	defaultPhoneCodeGlobalHourlyMax = 500
)

// defaultSMSAllowedCountryPrefixes limits where verification SMS may be sent.
// KOOM serves Kyrgyzstan; sending codes to arbitrary countries lets attackers
// pump premium-rate numbers at the project's expense.
var defaultSMSAllowedCountryPrefixes = []string{"+996"}

type PhoneAuthConfig struct {
	JWTSecret           string
	AccessTokenTTL      time.Duration
	Environment         string
	TestAuthEnabled     bool
	TestAuthPhone       string
	TestAuthCode        string
	TestAuthDisplayName string
	AppReviewPhone      string
	AppReviewCode       string
	// SMSAllowedCountryPrefixes overrides defaultSMSAllowedCountryPrefixes when set.
	SMSAllowedCountryPrefixes []string
	// SMSGlobalHourlyMax overrides defaultPhoneCodeGlobalHourlyMax when positive.
	SMSGlobalHourlyMax int
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

	if s.isTestAuthMobile(mobile) {
		return RequestPhoneCodeOutput{Status: "test_code_ready", DevCode: s.expectedTestAuthCode()}, nil
	}
	appReview := s.isAppReviewMobile(mobile)
	if !appReview && !s.smsCountryAllowed(mobile) {
		return RequestPhoneCodeOutput{}, NewValidationError("sms verification is not available for this phone number")
	}

	now := time.Now().UTC()
	// The per-number daily cap also bounds guessing of the fixed review code.
	if err := s.checkSMSBudget(ctx, mobile, now); err != nil {
		return RequestPhoneCodeOutput{}, err
	}

	var code string
	if appReview {
		code = s.cfg.AppReviewCode
	} else {
		code, err = newNumericCode(6)
		if err != nil {
			return RequestPhoneCodeOutput{}, err
		}
	}
	codeID := "PC-" + strings.ToUpper(randomHex(12))
	allowed, err := s.repo.CreatePhoneCodeIfWithinLimit(
		ctx,
		codeID,
		mobile,
		security.HashToken(code),
		now.Add(phoneCodeTTL),
		now.Add(-phoneCodeRateLimitWindow),
		now.Add(-phoneCodeMinRequestGap),
		phoneCodeRateLimitMax,
	)
	if err != nil {
		return RequestPhoneCodeOutput{}, err
	}
	if !allowed {
		return RequestPhoneCodeOutput{}, NewValidationError("too many code requests; try again later")
	}
	if appReview {
		// Store-review account: the code is the configured one, so no SMS is
		// sent. The response is identical to a real send and never reveals it.
		log.Printf("app review login code issued for the configured review account")
		return RequestPhoneCodeOutput{Status: "code_sent"}, nil
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
	return RequestPhoneCodeOutput{Status: "code_sent"}, nil
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
		if code != s.expectedTestAuthCode() {
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
		if err := s.repo.MarkPhoneVerified(ctx, user.ID); err != nil {
			return domain.PhoneSession{}, err
		}
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
		if _, err := s.repo.IncrementPhoneCodeAttempts(ctx, record.ID, phoneCodeMaxAttempts); err != nil {
			return domain.PhoneSession{}, err
		}
		return domain.PhoneSession{}, ErrInvalidCredentials
	}
	if _, err := s.repo.GetPhoneUserByMobile(ctx, mobile); errors.Is(err, storage.ErrNotFound) {
		if err := validateNewPhoneUserDisplayName(input.DisplayName); err != nil {
			return domain.PhoneSession{}, err
		}
	} else if err != nil {
		return domain.PhoneSession{}, err
	}
	consumed, err := s.repo.ConsumePhoneCode(ctx, record.ID, phoneCodeMaxAttempts)
	if err != nil {
		return domain.PhoneSession{}, err
	}
	if !consumed {
		return domain.PhoneSession{}, ErrInvalidCredentials
	}

	user, err := s.getOrCreatePhoneUser(ctx, mobile, input.DisplayName)
	if err != nil {
		return domain.PhoneSession{}, err
	}
	if err := s.repo.MarkPhoneVerified(ctx, user.ID); err != nil {
		return domain.PhoneSession{}, err
	}
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
	if record.RevokedAt != nil {
		// A rotated or revoked token presented again is only logged: a client
		// that lost a refresh response on a flaky network legitimately retries
		// with the old token, so this must not sign the user out everywhere.
		log.Printf("refresh token reuse rejected user_id=%q session_id=%q revoked_at=%s", record.UserID, record.ID, record.RevokedAt.UTC().Format(time.RFC3339))
		return domain.PhoneSession{}, ErrUnauthorized
	}
	if time.Now().UTC().After(record.ExpiresAt) {
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
	newRefreshToken, err := security.NewOpaqueToken(48)
	if err != nil {
		return domain.PhoneSession{}, err
	}
	newRefreshSessionID := "RT-" + strings.ToUpper(randomHex(12))
	if err := s.repo.RotateRefreshSession(ctx, record.ID, newRefreshSessionID, user.ID, security.HashToken(newRefreshToken), time.Now().UTC().Add(refreshTokenTTL)); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.PhoneSession{}, ErrUnauthorized
		}
		return domain.PhoneSession{}, err
	}
	return domain.PhoneSession{AccessToken: accessToken, RefreshToken: newRefreshToken, User: user}, nil
}

// LogoutAll revokes every refresh token of the user (all devices).
func (s *PhoneAuthService) LogoutAll(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrUnauthorized
	}
	_, err := s.repo.RevokeAllRefreshSessionsForUser(ctx, userID)
	return err
}

func (s *PhoneAuthService) Logout(ctx context.Context, input RefreshInput) error {
	_, err := s.LogoutSession(ctx, input)
	return err
}

// LogoutSession revokes the refresh session and returns the user it belonged
// to ("" when the token was unknown or already revoked), so callers can also
// close that user's realtime connections.
func (s *PhoneAuthService) LogoutSession(ctx context.Context, input RefreshInput) (string, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return "", NewValidationError("refresh_token is required")
	}
	record, err := s.repo.GetRefreshSession(ctx, security.HashToken(refreshToken))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	if record.RevokedAt != nil {
		return "", nil
	}
	if err := s.repo.RevokeRefreshSession(ctx, record.ID); err != nil {
		return "", err
	}
	return record.UserID, nil
}

func (s *PhoneAuthService) getOrCreatePhoneUser(ctx context.Context, mobile string, displayNameInput string) (domain.PhoneAuthUser, error) {
	user, err := s.repo.GetPhoneUserByMobile(ctx, mobile)
	if errors.Is(err, storage.ErrNotFound) {
		if err := validateNewPhoneUserDisplayName(displayNameInput); err != nil {
			return domain.PhoneAuthUser{}, err
		}
		return s.repo.CreatePhoneUser(ctx, domain.PhoneAuthUser{
			ID:          "U-" + strings.ToUpper(randomHex(8)),
			Mobile:      mobile,
			DisplayName: strings.TrimSpace(displayNameInput),
			Role:        domain.UserRoleUser,
		})
	}
	if err != nil {
		return domain.PhoneAuthUser{}, err
	}
	return user, nil
}

func validateNewPhoneUserDisplayName(displayNameInput string) error {
	displayName := strings.TrimSpace(displayNameInput)
	if displayName == "" {
		return NewValidationError("display_name is required for new account")
	}
	if len(displayName) < 2 || len(displayName) > maxDisplayNameLen {
		return NewValidationError(fmt.Sprintf("display_name must be between 2 and %d characters", maxDisplayNameLen))
	}
	return nil
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
	if !s.cfg.TestAuthEnabled || !isLocalTestAuthEnvironment(s.cfg.Environment) {
		return false
	}
	for _, part := range strings.Split(s.cfg.TestAuthPhone, ",") {
		testPhone := normalizeTestValue(part)
		if testPhone == "" {
			continue
		}
		if testPhone == "*" || strings.EqualFold(testPhone, "any") || strings.EqualFold(testPhone, "all") {
			return true
		}
		if normalizeTestValue(mobile) == testPhone {
			return true
		}
	}
	return false
}

// isAppReviewMobile reports whether mobile is the dedicated store-review
// account (APP_REVIEW_PHONE). Unlike test auth it works in production, but
// only through the normal code flow: TTL, attempt limits and rate limits apply.
// APP_REVIEW_PHONE may list several comma-separated accounts.
func (s *PhoneAuthService) isAppReviewMobile(mobile string) bool {
	if len(s.cfg.AppReviewCode) != 6 {
		return false
	}
	target := normalizeTestValue(mobile)
	if target == "" {
		return false
	}
	for _, part := range strings.Split(s.cfg.AppReviewPhone, ",") {
		if normalizeTestValue(part) == target {
			return true
		}
	}
	return false
}

func (s *PhoneAuthService) expectedTestAuthCode() string {
	return strings.TrimSpace(s.cfg.TestAuthCode)
}

func (s *PhoneAuthService) testAuthDisplayName(_ string) string {
	name := strings.TrimSpace(s.cfg.TestAuthDisplayName)
	if name == "" {
		return "Test User"
	}
	return name
}

func isLocalTestAuthEnvironment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "local", "dev", "development", "test", "testing":
		return true
	default:
		return false
	}
}

func normalizeTestValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "(", "")
	value = strings.ReplaceAll(value, ")", "")
	return value
}
