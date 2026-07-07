package handlers

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// loginLimiter throttles failed login attempts per client IP to slow down
// online password guessing. It is a small in-memory fixed-window counter — no
// external dependency, adequate for a self-hosted single-instance app.
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptWindow
	max      int
	window   time.Duration
	now      func() time.Time // injectable for tests
}

type attemptWindow struct {
	count   int
	resetAt time.Time
}

func newLoginLimiter(max int, window time.Duration) *loginLimiter {
	return &loginLimiter{
		attempts: make(map[string]*attemptWindow),
		max:      max,
		window:   window,
		now:      time.Now,
	}
}

// allow reports whether a login attempt from key may proceed. It also opportun-
// istically prunes expired windows so the map doesn't grow without bound.
func (l *loginLimiter) allow(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	for k, w := range l.attempts {
		if now.After(w.resetAt) {
			delete(l.attempts, k)
		}
	}

	w := l.attempts[key]
	if w == nil || now.After(w.resetAt) {
		return true // fresh window; recorded on failure
	}
	return w.count < l.max
}

// recordFailure increments the failure counter for key.
func (l *loginLimiter) recordFailure(key string) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	w := l.attempts[key]
	if w == nil || now.After(w.resetAt) {
		l.attempts[key] = &attemptWindow{count: 1, resetAt: now.Add(l.window)}
		return
	}
	w.count++
}

// reset clears the counter for key after a successful login.
func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// clientIP extracts the client IP from the request for rate-limit keying. The
// RealIP middleware has already resolved r.RemoteAddr from proxy headers.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
