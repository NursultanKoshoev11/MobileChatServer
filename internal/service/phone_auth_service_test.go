package service

import "testing"

func TestTestAuthMobileSupportsCommaSeparatedPhones(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{
		Environment:     "staging",
		TestAuthEnabled: true,
		TestAuthPhone:   "+996555555555, +996700000001, +996700000002",
	}, nil)

	if !auth.isTestAuthMobile("+996555555555") {
		t.Fatal("expected first configured test phone to be accepted")
	}
	if !auth.isTestAuthMobile("+996700000001") {
		t.Fatal("expected second configured test phone to be accepted")
	}
	if !auth.isTestAuthMobile("+996700000002") {
		t.Fatal("expected third configured test phone to be accepted")
	}
	if auth.isTestAuthMobile("+996700123456") {
		t.Fatal("did not expect unconfigured phone to be accepted")
	}
}

func TestWildcardTestAuthMobileIsLocalOnly(t *testing.T) {
	staging := NewPhoneAuth(nil, PhoneAuthConfig{
		Environment:     "staging",
		TestAuthEnabled: true,
		TestAuthPhone:   "*",
	}, nil)
	if staging.isTestAuthMobile("+996555555555") {
		t.Fatal("did not expect wildcard test auth in shared environments")
	}

	local := NewPhoneAuth(nil, PhoneAuthConfig{
		Environment:     "development",
		TestAuthEnabled: true,
		TestAuthPhone:   "*",
	}, nil)
	if !local.isTestAuthMobile("+996555555555") {
		t.Fatal("expected wildcard test auth in local environments")
	}
}

func TestExpectedTestAuthCodeDefaultsToSixOnes(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{}, nil)

	if got := auth.expectedTestAuthCode("+996700000001"); got != "111111" {
		t.Fatalf("expected fallback test auth code 111111, got %q", got)
	}
}

func TestPublicDemoAuthMobileDoesNotRequireTestAuthFlag(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{
		Environment:     "production",
		TestAuthEnabled: false,
	}, nil)

	for _, phone := range []string{"+996555555555", "+996700000001", "+996700000002"} {
		if !auth.isDemoAuthMobile(phone) {
			t.Fatalf("expected public demo auth phone %s to be accepted", phone)
		}
		if got := auth.expectedTestAuthCode(phone); got != "111111" {
			t.Fatalf("expected public demo auth code 111111 for %s, got %q", phone, got)
		}
		if got := auth.testAuthDisplayName(phone); got != "Koom Demo User" {
			t.Fatalf("expected public demo display name for %s, got %q", phone, got)
		}
	}
	if auth.isDemoAuthMobile("+996700123456") {
		t.Fatal("did not expect other phones to use public demo auth")
	}
}
