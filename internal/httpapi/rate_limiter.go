package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

type visitor struct {
	count     int
	resetTime time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	limiter := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}
	go limiter.cleanupLoop()
	return limiter
}

func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.visitors[key]
	if !ok || now.After(entry.resetTime) {
		l.visitors[key] = &visitor{count: 1, resetTime: now.Add(l.window)}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	return true
}

func (l *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		l.mu.Lock()
		for key, entry := range l.visitors {
			if now.After(entry.resetTime) {
				delete(l.visitors, key)
			}
		}
		l.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(parts[i])
			if candidate == "" {
				continue
			}
			if ip := net.ParseIP(candidate); ip != nil {
				return ip.String()
			}
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		if ip := net.ParseIP(realIP); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
		return host
	}
	return r.RemoteAddr
}
