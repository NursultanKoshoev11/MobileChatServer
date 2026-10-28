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
	default:
		return nil, fmt.Errorf("SMS_PROVIDER is unsupported; integrate a provider before enabling SMS")
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
