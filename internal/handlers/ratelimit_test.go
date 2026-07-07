package handlers

import (
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cur := base

	l := newLoginLimiter(3, 10*time.Minute)
	l.now = func() time.Time { return cur }

	key := "1.2.3.4"
	// Three failures are allowed through; the fourth is blocked.
	for i := 0; i < 3; i++ {
		if !l.allow(key) {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
		l.recordFailure(key)
	}
	if l.allow(key) {
		t.Error("attempt after max failures should be blocked")
	}

	// After the window elapses the key is allowed again.
	cur = base.Add(11 * time.Minute)
	if !l.allow(key) {
		t.Error("should be allowed after the window expires")
	}

	// Keys are independent, and reset clears a key.
	cur = base
	l2 := newLoginLimiter(1, time.Minute)
	l2.now = func() time.Time { return cur }
	l2.recordFailure("a")
	if l2.allow("a") {
		t.Error("key a should be blocked")
	}
	if !l2.allow("b") {
		t.Error("key b should be independent and allowed")
	}
	l2.reset("a")
	if !l2.allow("a") {
		t.Error("key a should be allowed after reset")
	}
}
