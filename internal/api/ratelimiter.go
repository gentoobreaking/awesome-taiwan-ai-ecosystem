package api

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter implements a simple per-IP rate limiter using a sliding window.
// Based on spec §67: 100 req/min per IP.
type rateLimiter struct {
	maxRequests int
	window      time.Duration
	visitors    map[string]*visitor
	mu          *sync.Mutex
	corsOrigins []string
}

type visitor struct {
	requests  int
	resetTime time.Time
}

func newRateLimiter(perMin int) *rateLimiter {
	rl := &rateLimiter{
		maxRequests: perMin,
		window:      time.Minute,
		visitors:    make(map[string]*visitor),
		mu:          &sync.Mutex{},
		corsOrigins: []string{"*"},
	}

	// Cleanup old visitors every minute
	go func() {
		for {
			time.Sleep(time.Minute)
			rl.cleanup()
		}
	}()

	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()

	if !exists || now.After(v.resetTime) {
		rl.visitors[ip] = &visitor{
			requests:  1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	if v.requests >= rl.maxRequests {
		return false
	}

	v.requests++
	return true
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, v := range rl.visitors {
		if now.After(v.resetTime) {
			delete(rl.visitors, ip)
		}
	}
}

func (rl *rateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}
