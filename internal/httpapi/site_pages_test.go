package httpapi

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// The app links straight to /privacy from the profile screen, so the policy
// must be readable in Russian and Kyrgyz, with the English text kept as well.
func TestPrivacyPageIsTrilingual(t *testing.T) {
	server := &Server{logger: log.New(io.Discard, "", 0)}
	recorder := httptest.NewRecorder()
	server.privacyPolicy(recorder, httptest.NewRequest(http.MethodGet, "/privacy", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Fatalf("missing CSP: %q", got)
	}
	body := recorder.Body.String()
	for _, want := range []string{
		`<article class="card" id="ru" lang="ru">`,
		`<article class="card" id="ky" lang="ky">`,
		`<article class="card" id="en" lang="en">`,
		"Политика конфиденциальности KOOM", // Russian
		"Какую информацию мы собираем",     // Russian section 1
		"KOOM купуялык саясаты",            // Kyrgyz
		"Биз кандай маалымат чогултабыз",   // Kyrgyz section 1
		"Koom Privacy Policy",              // English
		"Information we collect",           // English section 1
		`href="/terms"`,
		`href="/account-deletion"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("privacy page is missing %q", want)
		}
	}
	// Every numbered section must survive in all three languages.
	for i := 1; i <= 11; i++ {
		heading := ">" + strconv.Itoa(i) + ". "
		if got := strings.Count(body, heading); got != 3 {
			t.Errorf("section %d appears %d times, want 3 (ru/ky/en)", i, got)
		}
	}
}

func TestHomePagesShareNavigationAndLegalLinks(t *testing.T) {
	server := &Server{logger: log.New(io.Discard, "", 0)}
	for _, path := range []string{"/", "/kg"} {
		recorder := httptest.NewRecorder()
		server.homePage(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", path, recorder.Code)
		}
		body := recorder.Body.String()
		for _, want := range []string{
			`href="/privacy"`,
			`href="/terms"`,
			`id="preview"`, // clickable product preview
			`id="faq"`,     // FAQ accordion
			`class="skip-link"`,
			`id="screen-feed"`,
			`id="screen-groups"`,
			`id="screen-alerts"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s is missing %q", path, want)
			}
		}
		// The pilot card must not reuse the big-number style any more.
		if strings.Contains(body, `research-stat-text"><strong>`) {
			t.Errorf("%s still renders the pilot heading with <strong>", path)
		}
		if got := strings.Count(body, `class="faq-item"`); got != 5 {
			t.Errorf("%s has %d FAQ items, want 5", path, got)
		}
	}
}
