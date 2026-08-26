package main

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipRateLimiter holds one token-bucket limiter per client IP — real state
// to encapsulate (CLAUDE.md rule 1), unlike the pure-function handlers
// elsewhere in this package. See docs/deployment-hardening-design.md for
// why this is per-IP/in-memory rather than a shared/distributed limiter.
type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPRateLimiter(rps float64, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		visitors: make(map[string]*visitor),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	limiter := v.limiter
	l.mu.Unlock()

	return limiter.Allow()
}

// cleanupLoop evicts visitors that haven't made a request in maxIdle, so a
// map keyed by "every distinct IP ever seen" doesn't grow without bound
// over the life of the process (CLAUDE.md rule 7). Intended to run in its
// own goroutine for the lifetime of the server; stops when ctx is done.
func (l *ipRateLimiter) cleanupLoop(done <-chan struct{}, interval, maxIdle time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			l.mu.Lock()
			for ip, v := range l.visitors {
				if now.Sub(v.lastSeen) > maxIdle {
					delete(l.visitors, ip)
				}
			}
			l.mu.Unlock()
		}
	}
}

// withRateLimit rejects a client past its request budget with 429 before
// the request reaches routing/handlers. Wrapped by withCorrelationID (see
// main.go), so the request's correlation id is already on the context by
// the time this runs — a throttled client's later retry can still be tied
// back to this log line. Also wrapped by withCORS, so a preflight OPTIONS
// request — already answered directly by withCORS — never consumes a
// client's budget.
func withRateLimit(limiter *ipRateLimiter, fallback *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !limiter.allow(ip) {
			loggerFromContext(r.Context(), fallback).Warn("rate limit exceeded", "ip", ip, "path", r.URL.Path)
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// clientIP reads RemoteAddr rather than X-Forwarded-For: there's no
// reverse proxy in front of this service yet (no live deploy target —
// docs/deployment-hardening-design.md), so trusting a client-supplied
// forwarded-for header without one would let anyone bypass the limiter by
// spoofing it.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func rateLimitFromEnv() *ipRateLimiter {
	rps := 20.0
	if raw := os.Getenv("RATE_LIMIT_RPS"); raw != "" {
		if n, err := strconv.ParseFloat(raw, 64); err == nil && n > 0 {
			rps = n
		}
	}
	burst := 40
	if raw := os.Getenv("RATE_LIMIT_BURST"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			burst = n
		}
	}
	return newIPRateLimiter(rps, burst)
}
