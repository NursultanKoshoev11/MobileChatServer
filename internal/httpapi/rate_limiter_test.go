package httpapi

import (
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
