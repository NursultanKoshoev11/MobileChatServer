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
	if !localTestAuthEnvironment("development") || !localTestAuthEnvironment("test") {
		t.Fatal("expected local test auth environments")
	}
	if localTestAuthEnvironment("staging") || localTestAuthEnvironment("production") {
		t.Fatal("did not expect shared environments to allow wildcard test auth")
	}
}
