package api

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter manages rate limiters for individual IP addresses.
type IPRateLimiter struct {
	ips     map[string]*rateLimiterEntry
	mu      sync.RWMutex
	r       rate.Limit
	b       int
	cleanup time.Duration
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter creates a new IP-based rate limiter.
// r is the rate (requests per second), b is the burst size.
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	limiter := &IPRateLimiter{
		ips:     make(map[string]*rateLimiterEntry),
		r:       r,
		b:       b,
		cleanup: 10 * time.Minute,
	}

	// Start cleanup goroutine to prevent memory leaks from stale entries
	go limiter.cleanupLoop()

	return limiter
}

// GetLimiter returns the rate limiter for the given IP address.
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	entry, exists := i.ips[ip]
	if !exists {
		limiter := rate.NewLimiter(i.r, i.b)
		i.ips[ip] = &rateLimiterEntry{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	entry.lastSeen = time.Now()
	return entry.limiter
}

// cleanupLoop periodically removes stale rate limiter entries.
func (i *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(i.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		i.mu.Lock()
		cutoff := time.Now().Add(-i.cleanup)
		for ip, entry := range i.ips {
			if entry.lastSeen.Before(cutoff) {
				delete(i.ips, ip)
			}
		}
		i.mu.Unlock()
	}
}

// extractIP extracts the client IP from a request, handling proxied requests.
func extractIP(r *http.Request) string {
	// chi's RealIP middleware sets RemoteAddr to the real IP
	// but we need to extract just the IP without the port
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr might not have a port
		return r.RemoteAddr
	}
	return ip
}

// RateLimitMiddleware returns middleware that rate limits requests by IP.
func RateLimitMiddleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			if !limiter.GetLimiter(ip).Allow() {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LoginRateLimiter is a stricter rate limiter specifically for login attempts.
// It tracks failed attempts by both IP and username to prevent distributed attacks.
type LoginRateLimiter struct {
	ipAttempts       map[string]*loginAttempt
	usernameAttempts map[string]*loginAttempt
	mu               sync.RWMutex
}

type loginAttempt struct {
	failures int
	lastFail time.Time
	blocked  time.Time
}

// NewLoginRateLimiter creates a new login-specific rate limiter.
func NewLoginRateLimiter() *LoginRateLimiter {
	limiter := &LoginRateLimiter{
		ipAttempts:       make(map[string]*loginAttempt),
		usernameAttempts: make(map[string]*loginAttempt),
	}

	// Cleanup goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			limiter.cleanup()
		}
	}()

	return limiter
}

// Allow checks if a login attempt should be allowed for the given IP.
func (l *LoginRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, exists := l.ipAttempts[ip]
	if !exists {
		return true
	}

	// Check if still blocked
	if time.Now().Before(attempt.blocked) {
		return false
	}

	return true
}

// AllowUsername checks if a login attempt should be allowed for the given username.
// This prevents distributed brute force attacks against a single account.
func (l *LoginRateLimiter) AllowUsername(username string) bool {
	if username == "" {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, exists := l.usernameAttempts[username]
	if !exists {
		return true
	}

	// Check if still blocked
	if time.Now().Before(attempt.blocked) {
		return false
	}

	return true
}

// RecordFailure records a failed login attempt and calculates the next block period.
func (l *LoginRateLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, exists := l.ipAttempts[ip]
	if !exists {
		attempt = &loginAttempt{}
		l.ipAttempts[ip] = attempt
	}

	attempt.failures++
	attempt.lastFail = time.Now()

	// Exponential backoff: 1s, 2s, 4s, 8s, 16s, max 60s
	// Shift is bounded to [0, 6] by max/min, safe for uint conversion
	shift := max(0, min(attempt.failures-1, 6))
	backoff := time.Duration(1<<uint(shift)) * time.Second // #nosec G115 -- shift is bounded to [0, 6]
	if backoff > 60*time.Second {
		backoff = 60 * time.Second
	}
	attempt.blocked = time.Now().Add(backoff)
}

// RecordUsernameFailure records a failed login attempt for a specific username.
// Uses longer backoff since distributed attacks can use many IPs.
func (l *LoginRateLimiter) RecordUsernameFailure(username string) {
	if username == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, exists := l.usernameAttempts[username]
	if !exists {
		attempt = &loginAttempt{}
		l.usernameAttempts[username] = attempt
	}

	attempt.failures++
	attempt.lastFail = time.Now()

	// Longer backoff for username: 2s, 4s, 8s, 16s, 32s, max 300s (5 min)
	// Shift is bounded to [0, 8] by max/min, safe for uint conversion
	shift := max(0, min(attempt.failures, 8))
	backoff := time.Duration(1<<uint(shift)) * time.Second // #nosec G115 -- shift is bounded to [0, 8]
	if backoff > 300*time.Second {
		backoff = 300 * time.Second
	}
	attempt.blocked = time.Now().Add(backoff)
}

// RecordSuccess clears the failure count for an IP after successful login.
func (l *LoginRateLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.ipAttempts, ip)
}

// RecordUsernameSuccess clears the failure count for a username after successful login.
func (l *LoginRateLimiter) RecordUsernameSuccess(username string) {
	if username == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.usernameAttempts, username)
}

// BlockDuration returns how long the IP is blocked, or 0 if not blocked.
func (l *LoginRateLimiter) BlockDuration(ip string) time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.ipAttempts[ip]
	if !exists {
		return 0
	}

	remaining := time.Until(attempt.blocked)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// UsernameBlockDuration returns how long the username is blocked, or 0 if not blocked.
func (l *LoginRateLimiter) UsernameBlockDuration(username string) time.Duration {
	if username == "" {
		return 0
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.usernameAttempts[username]
	if !exists {
		return 0
	}

	remaining := time.Until(attempt.blocked)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (l *LoginRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for ip, attempt := range l.ipAttempts {
		if attempt.lastFail.Before(cutoff) {
			delete(l.ipAttempts, ip)
		}
	}
	for username, attempt := range l.usernameAttempts {
		if attempt.lastFail.Before(cutoff) {
			delete(l.usernameAttempts, username)
		}
	}
}
