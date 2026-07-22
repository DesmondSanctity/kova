package api

import (
	"net/http"
	"sync"
	"time"
)

// rateLimiter is a tiny in-memory fixed-window limiter keyed by client IP.
// Good enough to blunt abuse on public endpoints; swap for Redis in a cluster.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: map[string][]time.Time{}, max: max, window: window}
}

func (rl *rateLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-rl.window)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.max {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, now)
	return true
}

// rateLimit wraps a handler, limiting requests per client IP.
func (s *Server) rateLimit(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.limiter != nil && !s.limiter.allow(clientIP(r)) {
			writeErr(w, http.StatusTooManyRequests, "too many requests, please slow down")
			return
		}
		h(w, r)
	}
}
