package sms

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTwilioSenderSendsVerificationCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("request method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/2010-04-01/Accounts/ACtest/Messages.json" {
			t.Errorf("request path = %s", r.URL.Path)
		}
		username, password, ok := r.BasicAuth()
		if !ok || username != "ACtest" || password != "test-token" {
			t.Errorf("unexpected Basic auth credentials")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("parse request body: %v", err)
			return
		}
		if values.Get("To") != "+996700000001" || values.Get("From") != "KOOM" {
			t.Errorf("unexpected message addressing: To=%q From=%q", values.Get("To"), values.Get("From"))
		}
		if !strings.Contains(values.Get("Body"), "123456") {
			t.Errorf("verification code missing from body")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sender := TwilioSender{
		AccountSID: "ACtest",
		AuthToken:  "test-token",
		From:       "KOOM",
		endpoint:   server.URL + "/2010-04-01/",
		HTTPClient: server.Client(),
	}
	if err := sender.SendVerificationCode(context.Background(), "+996700000001", "123456"); err != nil {
		t.Fatalf("SendVerificationCode returned error: %v", err)
	}
}
