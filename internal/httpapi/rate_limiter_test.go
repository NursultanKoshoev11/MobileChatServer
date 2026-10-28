package httpapi

import (
	"net/http"
	"testing"
	"time"
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
