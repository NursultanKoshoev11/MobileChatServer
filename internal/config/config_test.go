package config

import (
	"strings"
	"testing"
)

func TestOriginListContainsWildcard(t *testing.T) {
	if !originListContainsWildcard("https://example.com, *") {
		t.Fatal("expected wildcard")
	}
	if originListContainsWildcard("https://example.com,https://admin.example.com") {
		t.Fatal("unexpected wildcard")
	}
}

func TestTestAuthIsLimitedToLocalEnvironments(t *testing.T) {
	for _, environment := range []string{"local", "dev", "development", "test", "testing"} {
		if !localTestAuthEnvironment(environment) {
			t.Errorf("expected %q to allow local test auth", environment)
		}
	}
	for _, environment := range []string{"staging", "production"} {
		if localTestAuthEnvironment(environment) {
			t.Errorf("shared environment %q must not allow test auth", environment)
		}
	}

	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1/test")
	t.Setenv("JWT_SECRET", "12345678901234567890123456789012")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("TEST_AUTH_ENABLED", "true")
	t.Setenv("TEST_AUTH_PHONE", "+996700000001")
	t.Setenv("TEST_AUTH_CODE", "111111")
	t.Setenv("BCRYPT_COST", "12")
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.18.0.0/16")
	if _, err := Load(); err == nil {
		t.Fatal("expected staging test auth configuration to be rejected")
	}
}

func TestValidTrustedProxyCIDRs(t *testing.T) {
	for _, value := range []string{"172.18.0.0/16", "127.0.0.1", "192.0.2.0/24, 192.0.2.10", "2001:db8::/32"} {
		if !validTrustedProxyCIDRs(value) {
			t.Errorf("expected %q to be accepted", value)
		}
	}
	for _, value := range []string{"", "not-an-ip", "172.18.0.0/16, invalid", "172.16.0.0/12", "0.0.0.0/0", "::/0"} {
		if validTrustedProxyCIDRs(value) {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}

func TestAppendUniquePhoneIgnoresBlankAndDuplicates(t *testing.T) {
	phones := appendUniquePhone([]string{"+996700000001"}, "")
	if len(phones) != 1 {
		t.Fatalf("blank phone should be ignored: %#v", phones)
	}
	phones = appendUniquePhone(phones, "+996700000001")
	if len(phones) != 1 {
		t.Fatalf("duplicate phone should be ignored: %#v", phones)
	}
}

func setBaseConfigEnv(t *testing.T, environment string) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1/test")
	t.Setenv("JWT_SECRET", "q7Vx2LmN9pR4sT8wYz3bC6dF1gH5jK0n")
	t.Setenv("APP_ENV", environment)
	t.Setenv("BCRYPT_COST", "12")
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.18.0.0/16")
	t.Setenv("ALLOWED_ORIGINS", "https://koom.example")
	t.Setenv("TEST_AUTH_ENABLED", "")
	t.Setenv("SMS_ALLOWED_COUNTRY_PREFIXES", "")
	t.Setenv("SMS_GLOBAL_HOURLY_MAX", "")
}

func TestAppEnvMustBeKnown(t *testing.T) {
	for _, environment := range []string{"local", "dev", "development", "test", "testing", "staging", "production", "Production"} {
		setBaseConfigEnv(t, environment)
		if _, err := Load(); err != nil {
			t.Errorf("APP_ENV=%q rejected: %v", environment, err)
		}
	}
	for _, environment := range []string{"prod", "stage", "qa", "productoin"} {
		setBaseConfigEnv(t, environment)
		if _, err := Load(); err == nil {
			t.Errorf("APP_ENV=%q must be rejected", environment)
		}
	}
}

func TestPlaceholderJWTSecretRejectedInSharedEnvironments(t *testing.T) {
	placeholders := []string{
		"change_this_jwt_secret_to_a_long_random_value_32_chars_min",
		"replace_with_at_least_32_random_characters_prefer_64_plus",
		"EXAMPLE-0123456789abcdef0123456789abcdef",
		"my-jwt-secret-here-0123456789abcdefghij",
	}
	for _, secret := range placeholders {
		setBaseConfigEnv(t, "production")
		t.Setenv("JWT_SECRET", secret)
		if _, err := Load(); err == nil {
			t.Errorf("placeholder secret %q accepted in production", secret)
		}
		setBaseConfigEnv(t, "development")
		t.Setenv("JWT_SECRET", secret)
		if _, err := Load(); err != nil {
			t.Errorf("placeholder secret should still work locally: %v", err)
		}
	}
	setBaseConfigEnv(t, "staging")
	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Error("short secret accepted")
	}
}

func TestSMSGuardConfig(t *testing.T) {
	setBaseConfigEnv(t, "production")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.SMSAllowedCountryPrefixes) != 1 || cfg.SMSAllowedCountryPrefixes[0] != "+996" || cfg.SMSGlobalHourlyMax != 500 {
		t.Fatalf("defaults: prefixes=%v hourly=%d", cfg.SMSAllowedCountryPrefixes, cfg.SMSGlobalHourlyMax)
	}

	t.Setenv("SMS_ALLOWED_COUNTRY_PREFIXES", " +996, +7 ,+996")
	t.Setenv("SMS_GLOBAL_HOURLY_MAX", "120")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.SMSAllowedCountryPrefixes) != 2 || cfg.SMSAllowedCountryPrefixes[1] != "+7" || cfg.SMSGlobalHourlyMax != 120 {
		t.Fatalf("custom: prefixes=%v hourly=%d", cfg.SMSAllowedCountryPrefixes, cfg.SMSGlobalHourlyMax)
	}

	for _, bad := range []string{"996", "+99x", ","} {
		t.Setenv("SMS_ALLOWED_COUNTRY_PREFIXES", bad)
		if _, err := Load(); err == nil {
			t.Errorf("SMS_ALLOWED_COUNTRY_PREFIXES=%q accepted", bad)
		}
	}
	t.Setenv("SMS_ALLOWED_COUNTRY_PREFIXES", "")
	for _, bad := range []string{"0", "-5", "many"} {
		t.Setenv("SMS_GLOBAL_HOURLY_MAX", bad)
		if _, err := Load(); err == nil {
			t.Errorf("SMS_GLOBAL_HOURLY_MAX=%q accepted", bad)
		}
	}
}

func TestAppReviewLoginValidation(t *testing.T) {
	base := func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1/test")
		t.Setenv("JWT_SECRET", "k8Zq2Lr7Vx4Np9Tg3Hw6Jy1Bc5Mf0Ds8Qa")
		t.Setenv("APP_ENV", "production")
		t.Setenv("SUPER_ADMIN_PHONES", "+996700111222")
		t.Setenv("PROJECT_OWNER_PHONE", "")
		t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.220/32")
	}
	cases := []struct {
		name, phone, code string
		ok                bool
	}{
		{"not configured", "", "", true},
		{"valid", "+996312000001", "482915", true},
		{"valid with spaces", "+996 312 000 001", "482915", true},
		{"phone without code", "+996312000001", "", false},
		{"code without phone", "", "482915", false},
		{"bad phone", "0312000001", "482915", false},
		{"short code", "+996312000001", "48291", false},
		{"letters", "+996312000001", "48a915", false},
		{"repeated digits", "+996312000001", "111111", false},
		{"two digits only", "+996312000001", "121212", false},
		{"ascending", "+996312000001", "123456", false},
		{"descending", "+996312000001", "987654", false},
		{"admin phone", "+996700111222", "482915", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base(t)
			t.Setenv("APP_REVIEW_PHONE", tc.phone)
			t.Setenv("APP_REVIEW_CODE", tc.code)
			_, err := Load()
			if tc.ok && err != nil {
				t.Fatalf("expected valid config, got %v", err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "APP_REVIEW")) {
				t.Fatalf("expected APP_REVIEW error, got %v", err)
			}
		})
	}
}
