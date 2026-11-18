package sms

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const smsProMessageEndpoint = "https://smspro.nikita.kg/api/message"

const (
	smsProRequestTimeout = 10 * time.Second
	smsProRetryDelay     = 5 * time.Second
	smsProMaxResponse    = 32 * 1024
)

type SMSProSender struct {
	Login      string
	Password   string
	SenderName string
	HTTPClient *http.Client
	endpoint   string
	retryDelay time.Duration
}

type smsProMessage struct {
	XMLName  xml.Name     `xml:"message"`
	Login    string       `xml:"login"`
	Password string       `xml:"pwd"`
	ID       string       `xml:"id"`
	Sender   string       `xml:"sender"`
	Text     string       `xml:"text"`
	Phones   smsProPhones `xml:"phones"`
}

type smsProPhones struct {
	Phone []string `xml:"phone"`
}

type smsProResponse struct {
	XMLName xml.Name `xml:"response"`
	Status  *int     `xml:"status"`
	Message string   `xml:"message"`
}

func (s SMSProSender) Validate() error {
	if strings.TrimSpace(s.Login) == "" || strings.TrimSpace(s.Password) == "" {
		return errors.New("SMS_PROVIDER=smspro requires SMSPRO_LOGIN and SMSPRO_PASSWORD")
	}
	if !validSMSProSenderName(strings.TrimSpace(s.SenderName)) {
		return errors.New("SMS_PROVIDER=smspro requires a valid SMSPRO_SENDER")
	}
	return nil
}

func (s SMSProSender) SendVerificationCode(ctx context.Context, phoneNumber string, code string) error {
	if err := s.Validate(); err != nil {
		return err
	}
	login := strings.TrimSpace(s.Login)
	password := strings.TrimSpace(s.Password)
	senderName := strings.TrimSpace(s.SenderName)

	messageID, err := newSMSProMessageID()
	if err != nil {
		return fmt.Errorf("create smspro message id: %w", err)
	}

	requestXML, err := xml.Marshal(smsProMessage{
		Login:    login,
		Password: password,
		ID:       messageID,
		Sender:   senderName,
		Text:     fmt.Sprintf("Your Koom verification code is %s", code),
		Phones:   smsProPhones{Phone: []string{phoneNumber}},
	})
	if err != nil {
		return fmt.Errorf("encode smspro request: %w", err)
	}
	requestXML = append([]byte(xml.Header), requestXML...)

	endpoint := strings.TrimSpace(s.endpoint)
	if endpoint == "" {
		endpoint = smsProMessageEndpoint
	}

	for attempt := 0; attempt < 2; attempt++ {
		status, err := s.submit(ctx, endpoint, requestXML)
		if err != nil {
			return err
		}
		if status == 0 {
			return nil
		}
		if status != 9 || attempt == 1 {
			return fmt.Errorf("smspro rejected SMS request: status=%d", status)
		}

		delay := s.retryDelay
		if delay <= 0 {
			delay = smsProRetryDelay
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return errors.New("smspro request ended unexpectedly")
}

func (s SMSProSender) submit(ctx context.Context, endpoint string, requestXML []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestXML))
	if err != nil {
		return 0, fmt.Errorf("create smspro request: %w", err)
	}
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Accept", "application/xml")

	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: smsProRequestTimeout}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		// Credentials are in the XML body, so never forward a request after a redirect.
		return http.ErrUseLastResponse
	}

	resp, err := clientCopy.Do(req)
	if err != nil {
		return 0, fmt.Errorf("send smspro request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("smspro returned HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, smsProMaxResponse+1))
	if err != nil {
		return 0, fmt.Errorf("read smspro response: %w", err)
	}
	if len(body) > smsProMaxResponse {
		return 0, errors.New("smspro response is too large")
	}
	var result smsProResponse
	if err := xml.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parse smspro response: %w", err)
	}
	if result.XMLName.Local != "response" {
		return 0, errors.New("smspro response has an unexpected root element")
	}
	if result.Status == nil {
		return 0, errors.New("smspro response has no status")
	}
	return *result.Status, nil
}

func newSMSProMessageID() (string, error) {
	var value [6]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func validSMSProSenderName(value string) bool {
	if len(value) == 14 {
		for _, char := range value {
			if char < '0' || char > '9' {
				return false
			}
		}
		return true
	}
	if len(value) == 0 || len(value) > 11 {
		return false
	}
	for _, char := range value {
		if !((char >= 'A' && char <= 'Z') ||
			(char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '-') {
			return false
		}
	}
	return true
}
