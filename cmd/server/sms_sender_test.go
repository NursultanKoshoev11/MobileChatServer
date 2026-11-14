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

func TestNewSMSSenderRejectsDevelopmentProviderInProduction(t *testing.T) {
	_, err := newSMSSender(config.Config{Environment: "production", SMSProvider: "dev"}, nil)
	if err == nil {
		t.Fatal("expected dev SMS provider to be rejected in production")
	}
}
