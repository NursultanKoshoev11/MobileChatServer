package service

import "testing"

func TestTestAuthMobileSupportsConfiguredPhonesOnlyInLocalEnvironment(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{
		Environment:     "development",
		TestAuthEnabled: true,
		TestAuthPhone:   "+996555555555,+996700000001",
		TestAuthCode:    "654321",
	}, nil)

	if !auth.isTestAuthMobile("+996555555555") || !auth.isTestAuthMobile("+996700000001") {
		t.Fatal("expected configured development test phones to be accepted")
	}
	if auth.isTestAuthMobile("+996700123456") {
		t.Fatal("did not expect unconfigured phone to be accepted")
	}
	if got := auth.expectedTestAuthCode(); got != "654321" {
		t.Fatalf("unexpected test auth code %q", got)
	}
}

func TestTestAuthIsDisabledInSharedEnvironmentsEvenForSpecificPhone(t *testing.T) {
	for _, environment := range []string{"staging", "production"} {
		auth := NewPhoneAuth(nil, PhoneAuthConfig{
			Environment:     environment,
			TestAuthEnabled: true,
			TestAuthPhone:   "+996555555555",
			TestAuthCode:    "654321",
		}, nil)
		if auth.isTestAuthMobile("+996555555555") {
			t.Fatalf("did not expect test auth in %s", environment)
		}
	}
}

func TestWildcardTestAuthMobileIsLocalOnly(t *testing.T) {
	local := NewPhoneAuth(nil, PhoneAuthConfig{Environment: "test", TestAuthEnabled: true, TestAuthPhone: "*", TestAuthCode: "654321"}, nil)
	if !local.isTestAuthMobile("+996555555555") {
		t.Fatal("expected wildcard test auth in test environment")
	}
}

func TestExpectedTestAuthCodeHasNoProductionFallback(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{}, nil)
	if got := auth.expectedTestAuthCode(); got != "" {
		t.Fatalf("expected no fallback test auth code, got %q", got)
	}
}
