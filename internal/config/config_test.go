package config

import "testing"

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
