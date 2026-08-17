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
