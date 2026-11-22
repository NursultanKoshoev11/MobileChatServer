package config

import (
	"strings"
	"testing"
)

func setProductionBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1/prod")
	t.Setenv("JWT_SECRET", "q8Zr2LwX5mN0vB7cT4yH9kP1sD6fG3jA")
	t.Setenv("APP_ENV", "production")
	t.Setenv("BCRYPT_COST", "12")
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.18.0.0/16")
	t.Setenv("ALLOWED_ORIGINS", "https://example.org")
	t.Setenv("HF_TOKEN", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("APP_REVIEW_PHONE", "")
	t.Setenv("APP_REVIEW_CODE", "")
	t.Setenv("TEST_AUTH_ENABLED", "")
}

func TestProductionRefusesDisabledModeration(t *testing.T) {
	setProductionBaseEnv(t)
	t.Setenv("CONTENT_MODERATION_ENABLED", "false")
	t.Setenv("CONTENT_MODERATION_PROVIDER", "local")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "CONTENT_MODERATION_ENABLED=false") {
		t.Fatalf("expected production to refuse disabled moderation, got %v", err)
	}
}

// The exact production moderation configuration (local rules, fail closed, no
// Hugging Face token) must keep starting.
func TestProductionAcceptsLocalModerationWithoutToken(t *testing.T) {
	setProductionBaseEnv(t)
	t.Setenv("CONTENT_MODERATION_ENABLED", "true")
	t.Setenv("CONTENT_MODERATION_PROVIDER", "local")
	t.Setenv("MODERATION_FAIL_CLOSED", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("production with local moderation must start: %v", err)
	}
	if !cfg.ContentModerationEnabled || cfg.ContentModerationProvider != "local" {
		t.Fatalf("unexpected moderation config: %+v", cfg)
	}
	if cfg.UploadDailyQuotaBytes != 300<<20 {
		t.Fatalf("default upload quota = %d", cfg.UploadDailyQuotaBytes)
	}
}

func TestDisabledModerationStillAllowedOutsideProduction(t *testing.T) {
	setProductionBaseEnv(t)
	t.Setenv("APP_ENV", "test")
	t.Setenv("CONTENT_MODERATION_ENABLED", "false")
	if _, err := Load(); err != nil {
		t.Fatalf("test environment may disable moderation: %v", err)
	}
}

func TestUploadQuotaConfig(t *testing.T) {
	setProductionBaseEnv(t)
	t.Setenv("APP_ENV", "test")
	t.Setenv("UPLOAD_DAILY_QUOTA_MB", "0")
	if cfg, err := Load(); err != nil || cfg.UploadDailyQuotaBytes != 0 {
		t.Fatalf("quota 0 should disable: %+v %v", cfg.UploadDailyQuotaBytes, err)
	}
	t.Setenv("UPLOAD_DAILY_QUOTA_MB", "-5")
	if _, err := Load(); err == nil {
		t.Fatal("negative quota must be rejected")
	}
}
