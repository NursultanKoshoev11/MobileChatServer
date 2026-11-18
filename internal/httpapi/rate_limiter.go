package httpapi

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxTrackedRateLimitKeys = 100_000

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
	maxKeys  int
}

type visitor struct {
	count     int
	resetTime time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	limiter := newRateLimiter(limit, window, maxTrackedRateLimitKeys)
	go limiter.cleanupLoop()
	return limiter
}

func newRateLimiter(limit int, window time.Duration, maxKeys int) *RateLimiter {
	if maxKeys <= 0 {
		maxKeys = maxTrackedRateLimitKeys
	}
	return &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
		maxKeys:  maxKeys,
	}
}

func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.visitors[key]
	if !ok {
		if len(l.visitors) >= l.maxKeys {
			// Refusing every new key once the table is full would let anyone
			// with many source addresses lock out all other clients. Make room
			// instead: drop expired windows, then the windows closest to reset.
			l.makeRoomLocked(now)
		}
		l.visitors[key] = &visitor{count: 1, resetTime: now.Add(l.window)}
		return true
	}
	if now.After(entry.resetTime) {
		entry.count = 1
		entry.resetTime = now.Add(l.window)
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	return true
}

// makeRoomLocked frees space in a full table. Expired entries go first; if
// that is not enough, about 1% of the table with the soonest reset times is
// evicted in one batch so the O(n log n) work is amortized over many inserts.
// Evicting a key only resets that client's counter early, which is the least
// harmful outcome under key-flooding.
func (l *RateLimiter) makeRoomLocked(now time.Time) {
	for key, entry := range l.visitors {
		if now.After(entry.resetTime) {
			delete(l.visitors, key)
		}
	}
	if len(l.visitors) < l.maxKeys {
		return
	}
	type candidate struct {
		key       string
		resetTime time.Time
	}
	candidates := make([]candidate, 0, len(l.visitors))
	for key, entry := range l.visitors {
		candidates = append(candidates, candidate{key: key, resetTime: entry.resetTime})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].resetTime.Before(candidates[j].resetTime) })
	evict := l.maxKeys / 100
	if evict < 1 {
		evict = 1
	}
	if excess := len(l.visitors) - l.maxKeys + 1; evict < excess {
		evict = excess
	}
	for i := 0; i < evict && i < len(candidates); i++ {
		delete(l.visitors, candidates[i].key)
	}
}

func (l *RateLimiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.visitors)
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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
		return host
	}
	return r.RemoteAddr
}

func (s *Server) clientIP(r *http.Request) string {
	remote := clientIP(r)
	remoteIP := net.ParseIP(remote)
	if remoteIP == nil || !ipInNetworks(remoteIP, s.trustedProxies) {
		return remote
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(parts[i])
			if candidate == "" {
				continue
			}
			if ip := net.ParseIP(candidate); ip != nil {
				if ipInNetworks(ip, s.trustedProxies) {
					continue
				}
				return ip.String()
			}
		}
	}
	return remote
}

func ipInNetworks(ip net.IP, networks []net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// Invite-code joins per account. Legitimate users join a handful of groups;
// this keeps code guessing to at most ~40 attempts per hour per account.
const (
	joinCodeAttemptsPerAccount = 10
	joinCodeAttemptWindow      = 15 * time.Minute
)

// rateLimitKeyForIP groups IPv6 clients by their /64: a single subscriber is
// usually assigned a whole /64, so per-address keys would let one client
// rotate through billions of addresses to evade the limit.
func rateLimitKeyForIP(raw string) string {
	ip := net.ParseIP(raw)
	if ip == nil || ip.To4() != nil {
		return raw
	}
	network := ip.Mask(net.CIDRMask(64, 128))
	return network.String() + "/64"
}

// Authenticated requests are also limited per account, after authentication.
// The pre-auth limiter only sees IP addresses.
const (
	perUserRequestsPerWindow = 300
	perUserRequestWindow     = time.Minute
)

func (s *Server) userRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)
		if user.ID != "" && s.userLimiter != nil && !s.userLimiter.Allow("user:"+user.ID) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
