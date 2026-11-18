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

func TestTestAuthIsDisabledOutsideLocalEnvironments(t *testing.T) {
	for _, environment := range []string{"staging", "production"} {
		auth := NewPhoneAuth(nil, PhoneAuthConfig{
			Environment:     environment,
			TestAuthEnabled: true,
			TestAuthPhone:   "+996700000001",
			TestAuthCode:    "111111",
		}, nil)
		if auth.isTestAuthMobile("+996700000001") {
			t.Errorf("test auth must be disabled in %s", environment)
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

func TestAppReviewMobileMatchesOnlyConfiguredNumberInAnyEnvironment(t *testing.T) {
	for _, environment := range []string{"development", "staging", "production"} {
		auth := NewPhoneAuth(nil, PhoneAuthConfig{
			Environment:    environment,
			AppReviewPhone: "+996 312 000 001",
			AppReviewCode:  "482915",
		}, nil)
		if !auth.isAppReviewMobile("+996312000001") {
			t.Errorf("%s: expected configured review phone to match", environment)
		}
		if auth.isAppReviewMobile("+996312000002") {
			t.Errorf("%s: unexpected match for another phone", environment)
		}
		if auth.isTestAuthMobile("+996312000001") {
			t.Errorf("%s: review phone must not enable test auth (which reveals codes)", environment)
		}
	}
}

func TestAppReviewMobileDisabledWithoutValidCode(t *testing.T) {
	for _, code := range []string{"", "12345"} {
		auth := NewPhoneAuth(nil, PhoneAuthConfig{AppReviewPhone: "+996312000001", AppReviewCode: code}, nil)
		if auth.isAppReviewMobile("+996312000001") {
			t.Errorf("review login must be off with code %q", code)
		}
	}
	if NewPhoneAuth(nil, PhoneAuthConfig{}, nil).isAppReviewMobile("+996312000001") {
		t.Error("review login must be off when not configured")
	}
}
