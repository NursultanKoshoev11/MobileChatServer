package config

import "testing"

func TestOriginListContainsWildcard(t *testing.T) {
	if !originListContainsWildcard("https://example.com, *") { t.Fatal("expected wildcard") }
	if originListContainsWildcard("https://example.com,https://admin.example.com") { t.Fatal("unexpected wildcard") }
}

func TestWildcardTestAuthPhoneRequiresLocalEnvironment(t *testing.T) {
	if !wildcardTestAuthPhone("*") || !wildcardTestAuthPhone("any") { t.Fatal("expected wildcard values") }
	if !localTestAuthEnvironment("development") || !localTestAuthEnvironment("test") { t.Fatal("expected local environments") }
	if localTestAuthEnvironment("staging") || localTestAuthEnvironment("production") { t.Fatal("shared environments must not allow test auth") }
}

func TestStagingTestAuthAllowsUpToThreeExactPhones(t *testing.T) {
	if !stagingTestAuthEnvironment(" staging ") || !validStagingTestAuthConfig("+996 700-000-001", "111111") {
		t.Fatal("expected one exact staging phone and a six-digit code to be accepted")
	}
	if !validStagingTestAuthConfig("+996700000001, +996700000002, +996700000003", "111111") {
		t.Fatal("expected three exact staging phones to be accepted")
	}
	for _, tc := range []struct{ phone, code string }{
		{"*", "111111"},
		{"+996700000001,+996700000002,+996700000003,+996700000004", "111111"},
		{"+996700000001,+996700000001", "111111"},
		{"996700000001", "111111"},
		{"+996700000001", "11111x"},
		{"+996700000001", "11111"},
	} {
		if validStagingTestAuthConfig(tc.phone, tc.code) {
			t.Fatalf("unexpectedly accepted staging test auth config phone=%q", tc.phone)
		}
	}
	if stagingTestAuthEnvironment("production") || stagingTestAuthEnvironment("development") {
		t.Fatal("expected staging test auth to be limited to staging")
	}
}

func TestAppendUniquePhoneIgnoresBlankAndDuplicates(t *testing.T) {
	phones := appendUniquePhone([]string{"+996700000001"}, "")
	if len(phones) != 1 { t.Fatalf("blank phone should be ignored: %#v", phones) }
	phones = appendUniquePhone(phones, "+996700000001")
	if len(phones) != 1 { t.Fatalf("duplicate phone should be ignored: %#v", phones) }
}

func TestNormalizePhoneForValidation(t *testing.T) {
	if got := normalizePhoneForValidation("+996 (000) 000-000"); got != "+996000000000" {
		t.Fatalf("unexpected normalized phone %q", got)
	}
}
