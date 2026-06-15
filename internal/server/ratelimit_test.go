// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPerIPRateLimiter(t *testing.T) {
	rl := newIPRateLimiter(1, 1, false) // 1/sec, burst 1
	h := rl.wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request should pass, got %d", rec1.Code)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request should be 429, got %d", rec2.Code)
	}
	if rec2.Header().Get("Retry-After") == "" {
		t.Fatal("429 must set Retry-After")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.RemoteAddr = "10.0.0.2:1234"
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req2)
	if rec3.Code != http.StatusOK {
		t.Fatalf("new IP should pass, got %d", rec3.Code)
	}
}

func TestClientIP_TrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.9:5555"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.9")

	if ip := clientIP(req, false); ip != "10.0.0.9" {
		t.Fatalf("untrusted: want RemoteAddr host, got %q", ip)
	}
	if ip := clientIP(req, true); ip != "203.0.113.7" {
		t.Fatalf("trusted: want leftmost XFF, got %q", ip)
	}
}
