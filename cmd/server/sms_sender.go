package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/config"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/sms"
)

func newSMSSender(cfg config.Config, logger *log.Logger) sms.Sender {
	switch strings.ToLower(strings.TrimSpace(cfg.SMSProvider)) {
	case "dev", "":
		return sms.DevSender{Logger: logger}
	case "twilio":
		return sms.TwilioSender{
			AccountSID:          os.Getenv("TWILIO_ACCOUNT_SID"),
			AuthToken:           os.Getenv("TWILIO_AUTH_TOKEN"),
			From:                cfg.SMSFrom,
			MessagingServiceSID: os.Getenv("TWILIO_MESSAGING_SERVICE_SID"),
			HTTPClient:          &http.Client{Timeout: 10 * time.Second},
		}
	default:
		return sms.DisabledSender{}
	}
}
