package config

import "testing"

func TestOriginListContainsWildcard(t *testing.T) {
	if !originListContainsWildcard("https://example.com, *") {
		t.Fatal("expected wildcard to be detected")
	}
	if originListContainsWildcard("https://example.com,https://admin.example.com") {
		t.Fatal("did not expect wildcard")
	}
}

func TestWildcardTestAuthPhoneRequiresLocalEnvironment(t *testing.T) {
	if !wildcardTestAuthPhone("*") || !wildcardTestAuthPhone("any") {
		t.Fatal("expected wildcard values to be detected")
	}
	if !wildcardTestAuthPhone("+996555555555,*") {
		t.Fatal("expected wildcard values inside comma-separated lists to be detected")
	}
	if !localTestAuthEnvironment("development") || !localTestAuthEnvironment("test") {
		t.Fatal("expected local test auth environments")
	}
	if localTestAuthEnvironment("staging") || localTestAuthEnvironment("production") {
		t.Fatal("did not expect shared environments to allow wildcard test auth")
	}
}

func TestAppendUniquePhoneKeepsProjectOwnerOnce(t *testing.T) {
	phones := appendUniquePhone([]string{"+996700000001"}, defaultProjectOwnerPhone)
	if len(phones) != 2 || phones[1] != defaultProjectOwnerPhone {
		t.Fatalf("expected project owner phone to be appended once, got %#v", phones)
	}

	phones = appendUniquePhone(phones, defaultProjectOwnerPhone)
	if len(phones) != 2 {
		t.Fatalf("expected duplicate project owner phone to be ignored, got %#v", phones)
	}
}
