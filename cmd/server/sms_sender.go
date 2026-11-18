package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/config"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/sms"
)

func newSMSSender(cfg config.Config, logger *log.Logger) (sms.Sender, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.SMSProvider))
	switch provider {
	case "dev":
		if !isLocalSMSEnvironment(cfg.Environment) {
			return nil, fmt.Errorf("SMS_PROVIDER=dev is allowed only in local, development, or test environments")
		}
		return sms.DevSender{Logger: logger}, nil
	case "disabled":
		if strings.EqualFold(strings.TrimSpace(cfg.Environment), "production") {
			return nil, fmt.Errorf("SMS_PROVIDER=disabled is not allowed in production")
		}
		return sms.DisabledSender{}, nil
	case "smspro":
		sender := sms.SMSProSender{
			Login:      cfg.SMSProLogin,
			Password:   cfg.SMSProPassword,
			SenderName: cfg.SMSProSender,
		}
		if err := sender.Validate(); err != nil {
			return nil, err
		}
		return sender, nil
	case "twilio":
		if strings.TrimSpace(cfg.TwilioAccountSID) == "" || strings.TrimSpace(cfg.TwilioAuthToken) == "" {
			return nil, fmt.Errorf("SMS_PROVIDER=twilio requires TWILIO_ACCOUNT_SID and TWILIO_AUTH_TOKEN")
		}
		if strings.TrimSpace(cfg.SMSFrom) == "" && strings.TrimSpace(cfg.TwilioMessagingServiceSID) == "" {
			return nil, fmt.Errorf("SMS_PROVIDER=twilio requires SMS_FROM or TWILIO_MESSAGING_SERVICE_SID")
		}
		return sms.TwilioSender{
			AccountSID:          cfg.TwilioAccountSID,
			AuthToken:           cfg.TwilioAuthToken,
			From:                cfg.SMSFrom,
			MessagingServiceSID: cfg.TwilioMessagingServiceSID,
		}, nil
	default:
		return nil, fmt.Errorf("SMS_PROVIDER is unsupported; use smspro, twilio, or a local development provider")
	}
}

func isLocalSMSEnvironment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "local", "dev", "development", "test", "testing":
		return true
	default:
		return false
	}
}
