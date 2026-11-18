package sms

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSMSProSenderSendsXMLAndAcceptsQueuedMessage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/message" {
			t.Errorf("request = %s %s, want POST /api/message", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/xml; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		var request smsProMessage
		if err := xml.Unmarshal(body, &request); err != nil {
			t.Errorf("decode request XML: %v", err)
			return
		}
		if request.Login != "koom-login" || request.Password != "pass&word" {
			t.Errorf("request credentials were not encoded correctly")
		}
		if len(request.ID) != 12 {
			t.Errorf("message id length = %d, want 12", len(request.ID))
		}
		if request.Sender != "KOOM" || len(request.Phones.Phone) != 1 || request.Phones.Phone[0] != "+996700000001" {
			t.Errorf("unexpected sender or recipient in request: sender=%q phones=%v", request.Sender, request.Phones.Phone)
		}
		if !strings.Contains(request.Text, "123456") {
			t.Errorf("verification code missing from SMS text")
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, `<response><id>`+request.ID+`</id><status>0</status><phones>1</phones><smscnt>1</smscnt></response>`)
	}))
	defer server.Close()

	sender := SMSProSender{
		Login:      "koom-login",
		Password:   "pass&word",
		SenderName: "KOOM",
		HTTPClient: server.Client(),
		endpoint:   server.URL + "/api/message",
	}
	if err := sender.SendVerificationCode(context.Background(), "+996700000001", "123456"); err != nil {
		t.Fatalf("SendVerificationCode returned error: %v", err)
	}
}

func TestSMSProSenderRetriesStatusNineWithSameMessageID(t *testing.T) {
	var messageIDs []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		var request smsProMessage
		if err := xml.Unmarshal(body, &request); err != nil {
			t.Errorf("decode request XML: %v", err)
			return
		}
		messageIDs = append(messageIDs, request.ID)
		if len(messageIDs) == 1 {
			_, _ = io.WriteString(w, `<response><status>9</status></response>`)
			return
		}
		_, _ = io.WriteString(w, `<response><status>0</status></response>`)
	}))
	defer server.Close()

	sender := SMSProSender{
		Login:      "koom-login",
		Password:   "secret",
		SenderName: "KOOM",
		HTTPClient: server.Client(),
		endpoint:   server.URL,
		retryDelay: time.Millisecond,
	}
	if err := sender.SendVerificationCode(context.Background(), "996700000001", "123456"); err != nil {
		t.Fatalf("SendVerificationCode returned error: %v", err)
	}
	if len(messageIDs) != 2 || messageIDs[0] == "" || messageIDs[0] != messageIDs[1] {
		t.Fatalf("retry message IDs = %v, want the same non-empty ID", messageIDs)
	}
}

func TestSMSProSenderRejectsProviderErrorWithoutReturningProviderMessage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<response><status>2</status><message>credentials are invalid</message></response>`)
	}))
	defer server.Close()

	sender := SMSProSender{
		Login:      "koom-login",
		Password:   "secret",
		SenderName: "KOOM",
		HTTPClient: server.Client(),
		endpoint:   server.URL,
	}
	err := sender.SendVerificationCode(context.Background(), "996700000001", "123456")
	if err == nil || !strings.Contains(err.Error(), "status=2") {
		t.Fatalf("expected sanitized status error, got %v", err)
	}
	if strings.Contains(err.Error(), "credentials are invalid") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("provider response or credential leaked in error: %v", err)
	}
}

func TestValidSMSProSenderName(t *testing.T) {
	for _, value := range []string{"KOOM", "KOOM-1", "12345678901234"} {
		if !validSMSProSenderName(value) {
			t.Errorf("valid sender name %q was rejected", value)
		}
	}
	for _, value := range []string{"", "слишкомдлинное", "KOOM_sender", "1234567890123"} {
		if validSMSProSenderName(value) {
			t.Errorf("invalid sender name %q was accepted", value)
		}
	}
}
