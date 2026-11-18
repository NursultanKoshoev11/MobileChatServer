package service

import (
	"context"
	"testing"
)

func TestSMSCountryAllowlistDefaultsToKyrgyzstan(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{}, nil)
	if !auth.smsCountryAllowed("+996700123456") {
		t.Fatal("expected Kyrgyz numbers to be allowed")
	}
	for _, mobile := range []string{"+79001234567", "+12025550123", "+4420712345678"} {
		if auth.smsCountryAllowed(mobile) {
			t.Fatalf("did not expect %s to be allowed by default", mobile)
		}
	}
}

func TestSMSCountryAllowlistCanBeOverridden(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{SMSAllowedCountryPrefixes: []string{"+996", " +7 "}}, nil)
	if !auth.smsCountryAllowed("+79001234567") {
		t.Fatal("expected configured prefix to be allowed")
	}
	if auth.smsCountryAllowed("+12025550123") {
		t.Fatal("did not expect unconfigured country to be allowed")
	}
}

// A foreign number must be refused before any database or SMS work happens.
func TestRequestCodeRejectsForeignNumbersBeforeSending(t *testing.T) {
	auth := NewPhoneAuth(nil, PhoneAuthConfig{}, nil)
	_, err := auth.RequestCode(context.Background(), RequestPhoneCodeInput{Mobile: "+44 7911 123456"})
	if err == nil {
		t.Fatal("expected foreign number to be rejected")
	}
	if _, ok := err.(ValidationError); !ok {
		t.Fatalf("expected validation error, got %T: %v", err, err)
	}
}
