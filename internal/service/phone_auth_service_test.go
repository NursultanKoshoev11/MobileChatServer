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

func TestTestAuthIsDisabledInProductionEvenForSpecificPhone(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{
		Environment:     "production",
		TestAuthEnabled: true,
		TestAuthPhone:   "+996555555555",
		TestAuthCode:    "654321",
	}, nil)
	if auth.isTestAuthMobile("+996555555555") {
		t.Fatal("did not expect test auth in production")
	}
}

func TestStagingTestAuthAllowsOnlyConfiguredPhones(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{
		Environment:     "staging",
		TestAuthEnabled: true,
		TestAuthPhone:   "+996700000001,+996700000002,+996700000003",
		TestAuthCode:    "111111",
	}, nil)
	for _, phone := range []string{"+996700000001", "+996700000002", "+996700000003"} {
		if !auth.isTestAuthMobile(phone) {
			t.Fatalf("expected configured staging phone %s to be accepted", phone)
		}
	}
	if auth.isTestAuthMobile("+996700000004") {
		t.Fatal("did not expect an unconfigured staging phone to be accepted")
	}
}

func TestStagingTestAuthRejectsWildcardMoreThanThreePhonesDuplicatesAndInvalidCode(t *testing.T) {
	configs := []PhoneAuthConfig{
		{Environment: "staging", TestAuthEnabled: true, TestAuthPhone: "*", TestAuthCode: "111111"},
		{Environment: "staging", TestAuthEnabled: true, TestAuthPhone: "+996700000001,+996700000002,+996700000003,+996700000004", TestAuthCode: "111111"},
		{Environment: "staging", TestAuthEnabled: true, TestAuthPhone: "+996700000001,+996700000001", TestAuthCode: "111111"},
		{Environment: "staging", TestAuthEnabled: true, TestAuthPhone: "+996700000001,+996700000002", TestAuthCode: "11111x"},
	}
	for _, cfg := range configs {
		auth := NewPhoneAuth(nil, cfg, nil)
		if auth.isTestAuthMobile("+996700000001") {
			t.Fatalf("unexpected test auth for unsafe staging config: %#v", cfg)
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
