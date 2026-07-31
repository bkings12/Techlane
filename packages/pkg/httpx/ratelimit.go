package httpx

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/techlane/techlane/packages/pkg/apierrors"
)

// IPRateLimiter is a simple fixed-window limiter keyed by client IP. It's not
// as precise as a sliding window / token bucket in Redis, but it's dependency
// free and good enough to blunt brute-force and credential-stuffing traffic
// against a handful of sensitive unauthenticated endpoints (login, refresh,
// password reset, MFA verify).
type IPRateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*windowCount
}

type windowCount struct {
	count int
	start time.Time
}

func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	rl := &IPRateLimiter{limit: limit, window: window, hits: map[string]*windowCount{}}
	go rl.gc()
	return rl
}

func (rl *IPRateLimiter) gc() {
	t := time.NewTicker(rl.window)
	defer t.Stop()
	for range t.C {
		rl.mu.Lock()
		now := time.Now()
		for k, v := range rl.hits {
			if now.Sub(v.start) > rl.window*2 {
				delete(rl.hits, k)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *IPRateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	wc, ok := rl.hits[key]
	if !ok || now.Sub(wc.start) > rl.window {
		rl.hits[key] = &windowCount{count: 1, start: now}
		return true
	}
	if wc.count >= rl.limit {
		return false
	}
	wc.count++
	return true
}

// Middleware rejects requests once an IP exceeds `limit` requests per `window`.
func (rl *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(requestIP(r)) {
			w.Header().Set("Retry-After", "60")
			apierrors.Write(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests, please slow down", CorrelationID(r.Context()))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
