// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// ipRateLimiter is a per-client-IP token-bucket limiter. Buckets are created
// lazily and kept in memory for the process lifetime (bounded by distinct IPs;
// acceptable for the public OIDC start/callback endpoints). Used to bound the
// unauthenticated DB writes performed by /api/auth/oidc/start.
type ipRateLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*rate.Limiter
	r            rate.Limit
	burst        int
	trustedProxy bool
}

// newIPRateLimiter returns a limiter allowing r events/sec with the given
// burst, per client IP. trustedProxy selects X-Forwarded-For extraction.
func newIPRateLimiter(perSec float64, burst int, trustedProxy bool) *ipRateLimiter {
	return &ipRateLimiter{
		buckets:      map[string]*rate.Limiter{},
		r:            rate.Limit(perSec),
		burst:        burst,
		trustedProxy: trustedProxy,
	}
}

func (l *ipRateLimiter) limiterFor(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.buckets[ip]
	if !ok {
		lim = rate.NewLimiter(l.r, l.burst)
		l.buckets[ip] = lim
	}
	return lim
}

// wrap returns middleware that rejects over-limit requests with 429.
func (l *ipRateLimiter) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, l.trustedProxy)
		if !l.limiterFor(ip).Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP returns the client IP. When trustedProxy is true it uses the
// leftmost X-Forwarded-For entry (then X-Real-Ip); otherwise the TCP peer.
func clientIP(r *http.Request, trustedProxy bool) string {
	if trustedProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first, _, ok := strings.Cut(xff, ","); ok {
				return strings.TrimSpace(first)
			}
			return strings.TrimSpace(xff)
		}
		if xr := r.Header.Get("X-Real-Ip"); xr != "" {
			return strings.TrimSpace(xr)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
