package httpapi

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTermsPageStatesUGCPolicy(t *testing.T) {
	server := &Server{logger: log.New(io.Discard, "", 0)}
	recorder := httptest.NewRecorder()
	server.termsOfUse(recorder, httptest.NewRequest(http.MethodGet, "/terms", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Fatalf("missing CSP: %q", got)
	}
	if recorder.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing site security headers")
	}
	body := recorder.Body.String()
	for _, want := range []string{
		"нулевая терпимость", "24 часов", "заблокировать", "пожаловаться", // Russian
		"нөлдүк чыдамкайлык", "24 сааттын ичинде", "бөгөттөй", "даттана", // Kyrgyz
		`href="/privacy"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("terms page is missing %q", want)
		}
	}
	if !strings.Contains(privacyPolicyHTML, `href="/terms"`) {
		t.Error("privacy page must link to /terms")
	}
}
