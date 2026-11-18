package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func TestRateLimiterAllowsWithinLimit(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	if !limiter.Allow("client") {
		t.Fatal("first request should be allowed")
	}
	if !limiter.Allow("client") {
		t.Fatal("second request should be allowed")
	}
}

func TestRateLimiterBlocksOverLimit(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	if !limiter.Allow("client") {
		t.Fatal("first request should be allowed")
	}
	if limiter.Allow("client") {
		t.Fatal("second request should be blocked")
	}
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	limiter := NewRateLimiter(1, 10*time.Millisecond)
	if !limiter.Allow("client") {
		t.Fatal("first request should be allowed")
	}
	if limiter.Allow("client") {
		t.Fatal("second request should be blocked")
	}
	time.Sleep(20 * time.Millisecond)
	if !limiter.Allow("client") {
		t.Fatal("request after reset window should be allowed")
	}
}

func TestClientIPIgnoresForwardedHeadersWithoutTrustedProxy(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/api/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = "198.51.100.10:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.20")
	req.Header.Set("X-Real-IP", "203.0.113.21")
	server := &Server{}

	if got := server.clientIP(req); got != "198.51.100.10" {
		t.Fatalf("expected remote addr IP, got %q", got)
	}
}

func TestClientIPUsesNearestUntrustedForwardedIP(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/api/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = "10.0.0.2:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.20, 198.51.100.7, 10.0.0.2")
	server := &Server{trustedProxies: parseTrustedProxyCIDRs("10.0.0.0/8")}

	if got := server.clientIP(req); got != "198.51.100.7" {
		t.Fatalf("expected nearest untrusted client IP, got %q", got)
	}
}

func TestRateLimiterFullTableEvictsInsteadOfDenyingNewKeys(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute, 100)
	for i := 0; i < 100; i++ {
		if !limiter.Allow(fmt.Sprintf("flood-%d", i)) {
			t.Fatalf("flood key %d denied", i)
		}
	}
	// A legitimate new client must still get through when the table is full.
	if !limiter.Allow("legit") {
		t.Fatal("new key denied when the table is full")
	}
	if got := limiter.size(); got > 100 {
		t.Fatalf("table grew past its cap: %d", got)
	}
	// The newest key is still tracked and limited.
	if limiter.Allow("legit") {
		t.Fatal("legit key must still be limited after insertion")
	}
}

func TestRateLimiterFullTableEvictsExpiredFirst(t *testing.T) {
	limiter := newRateLimiter(1, 20*time.Millisecond, 10)
	for i := 0; i < 5; i++ {
		limiter.Allow(fmt.Sprintf("old-%d", i))
	}
	time.Sleep(30 * time.Millisecond)
	limiter.window = time.Minute
	for i := 0; i < 5; i++ {
		limiter.Allow(fmt.Sprintf("live-%d", i))
	}
	if !limiter.Allow("new") {
		t.Fatal("new key denied")
	}
	if got := limiter.size(); got != 6 {
		t.Fatalf("expected expired keys to be evicted first, size=%d", got)
	}
	for i := 0; i < 5; i++ {
		if limiter.Allow(fmt.Sprintf("live-%d", i)) {
			t.Fatalf("live key %d lost its counter", i)
		}
	}
}

func TestRateLimitKeyForIPGroupsIPv6By64(t *testing.T) {
	a := rateLimitKeyForIP("2001:db8:1:2:aaaa::1")
	b := rateLimitKeyForIP("2001:db8:1:2:ffff:ffff:ffff:ffff")
	c := rateLimitKeyForIP("2001:db8:1:3::1")
	if a != b {
		t.Fatalf("same /64 got different keys: %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("different /64 got the same key %q", a)
	}
	if got := rateLimitKeyForIP("203.0.113.7"); got != "203.0.113.7" {
		t.Fatalf("IPv4 key changed: %q", got)
	}
	if got := rateLimitKeyForIP("::ffff:203.0.113.7"); got != "::ffff:203.0.113.7" {
		t.Fatalf("IPv4-mapped key changed: %q", got)
	}
}

func TestUserRateLimitAppliesPerAuthenticatedUser(t *testing.T) {
	server := &Server{userLimiter: newRateLimiter(2, time.Minute, 100)}
	handler := server.userRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := func(userID string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey{}, domain.User{ID: userID}))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder.Code
	}
	if request("U-1") != http.StatusNoContent || request("U-1") != http.StatusNoContent {
		t.Fatal("requests within the limit must pass")
	}
	if code := request("U-1"); code != http.StatusTooManyRequests {
		t.Fatalf("third request status = %d, want 429", code)
	}
	if code := request("U-2"); code != http.StatusNoContent {
		t.Fatalf("other users must not be affected, got %d", code)
	}
}
