package main

import (
	"testing"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/config"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/sms"
)

func TestNewSMSSenderUsesTwilioInProduction(t *testing.T) {
	sender, err := newSMSSender(config.Config{
		Environment:               "production",
		SMSProvider:               "twilio",
		TwilioAccountSID:          "ACtest",
		TwilioAuthToken:           "test-token",
		SMSFrom:                   "KOOM",
		TwilioMessagingServiceSID: "",
	}, nil)
	if err != nil {
		t.Fatalf("newSMSSender returned error: %v", err)
	}
	if _, ok := sender.(sms.TwilioSender); !ok {
		t.Fatalf("expected sms.TwilioSender, got %T", sender)
	}
}

func TestNewSMSSenderRejectsIncompleteTwilioConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config config.Config
	}{
		{
			name: "missing account credentials",
			config: config.Config{
				Environment: "production",
				SMSProvider: "twilio",
				SMSFrom:     "KOOM",
			},
		},
		{
			name: "missing sender",
			config: config.Config{
				Environment:      "production",
				SMSProvider:      "twilio",
				TwilioAccountSID: "ACtest",
				TwilioAuthToken:  "test-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := newSMSSender(tt.config, nil); err == nil {
				t.Fatal("expected incomplete Twilio configuration to fail")
			}
		})
	}
}

func TestNewSMSSenderUsesSMSProInProduction(t *testing.T) {
	sender, err := newSMSSender(config.Config{
		Environment:    "production",
		SMSProvider:    "smspro",
		SMSProLogin:    "koom-login",
		SMSProPassword: "secret",
		SMSProSender:   "KOOM",
	}, nil)
	if err != nil {
		t.Fatalf("newSMSSender returned error: %v", err)
	}
	if _, ok := sender.(sms.SMSProSender); !ok {
		t.Fatalf("expected sms.SMSProSender, got %T", sender)
	}
}

func TestNewSMSSenderRejectsIncompleteSMSProConfiguration(t *testing.T) {
	_, err := newSMSSender(config.Config{
		Environment:  "production",
		SMSProvider:  "smspro",
		SMSProLogin:  "koom-login",
		SMSProSender: "KOOM",
	}, nil)
	if err == nil {
		t.Fatal("expected missing SMSPRO_PASSWORD to be rejected")
	}
}

func TestNewSMSSenderRejectsDevelopmentProviderInProduction(t *testing.T) {
	_, err := newSMSSender(config.Config{Environment: "production", SMSProvider: "dev"}, nil)
	if err == nil {
		t.Fatal("expected dev SMS provider to be rejected in production")
	}
}

func TestNewSMSSenderDisabledInProductionNeedsExplicitSwitch(t *testing.T) {
	if _, err := newSMSSender(config.Config{Environment: "production", SMSProvider: "disabled"}, nil); err == nil {
		t.Fatal("expected disabled SMS to be rejected in production without SMS_DISABLED_IN_PRODUCTION")
	}
	sender, err := newSMSSender(config.Config{Environment: "production", SMSProvider: "disabled", SMSDisabledInProduction: true}, nil)
	if err != nil {
		t.Fatalf("expected explicit switch to allow disabled SMS, got %v", err)
	}
	if _, ok := sender.(sms.DisabledSender); !ok {
		t.Fatalf("expected sms.DisabledSender, got %T", sender)
	}
}
