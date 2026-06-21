package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TwilioSender struct {
	AccountSID         string
	AuthToken          string
	From               string
	MessagingServiceSID string
	HTTPClient         *http.Client
}

func (s TwilioSender) SendVerificationCode(ctx context.Context, phoneNumber string, code string) error {
	accountSID := strings.TrimSpace(s.AccountSID)
	authToken := strings.TrimSpace(s.AuthToken)
	if accountSID == "" || authToken == "" {
		return fmt.Errorf("twilio sms sender is not configured")
	}
	values := url.Values{}
	values.Set("To", phoneNumber)
	values.Set("Body", fmt.Sprintf("Your MobileChat verification code is %s", code))
	if serviceSID := strings.TrimSpace(s.MessagingServiceSID); serviceSID != "" {
		values.Set("MessagingServiceSid", serviceSID)
	} else if from := strings.TrimSpace(s.From); from != "" {
		values.Set("From", from)
	} else {
		return fmt.Errorf("twilio sender requires SMS_FROM or TWILIO_MESSAGING_SERVICE_SID")
	}

	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", url.PathEscape(accountSID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(accountSID, authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send twilio sms: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var payload struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	if payload.Message != "" {
		return fmt.Errorf("twilio sms failed: status=%d code=%d message=%s", resp.StatusCode, payload.Code, payload.Message)
	}
	return fmt.Errorf("twilio sms failed: status=%d", resp.StatusCode)
}
