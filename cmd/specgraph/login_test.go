// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuardHTTPS(t *testing.T) {
	t.Parallel()
	if err := guardHTTPS("http://127.0.0.1:9090"); err != nil {
		t.Errorf("loopback http should pass: %v", err)
	}
	if err := guardHTTPS("https://api.example.com"); err != nil {
		t.Errorf("https should pass: %v", err)
	}
	if err := guardHTTPS("http://api.example.com"); err == nil {
		t.Error("remote http should fail")
	}
}

func TestLoopbackHandler_StateMismatch(t *testing.T) {
	t.Parallel()
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	h := loopbackHandler(5000, "secret", codeCh, errCh)
	req := httptest.NewRequest(http.MethodGet, "/callback?cli_state=wrong&code=abc", nil)
	req.Host = "127.0.0.1:5000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	select {
	case <-errCh:
	default:
		t.Fatal("expected an error on the channel")
	}
}

func TestLoopbackHandler_Success(t *testing.T) {
	t.Parallel()
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	h := loopbackHandler(5000, "secret", codeCh, errCh)
	req := httptest.NewRequest(http.MethodGet, "/callback?cli_state=secret&code=thecode", nil)
	req.Host = "127.0.0.1:5000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := <-codeCh; got != "thecode" {
		t.Fatalf("code = %q", got)
	}
}

func TestLoopbackHandler_IgnoresOtherPaths(t *testing.T) {
	t.Parallel()
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	h := loopbackHandler(5000, "secret", codeCh, errCh)
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	req.Host = "127.0.0.1:5000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	select {
	case <-errCh:
		t.Fatal("a non-callback request must not abort the login")
	default:
	}
}

func TestGuardRemote_SSHRejected(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "10.0.0.1 22 10.0.0.2 22")
	if err := guardRemote(); err == nil {
		t.Fatal("expected guardRemote to reject an SSH session")
	}
}

func TestPickProvider(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"providers":[{"id":"entra","display_name":"Entra"}]}`))
	}))
	defer srv.Close()
	id, err := pickProvider(t.Context(), srv.URL, "")
	if err != nil || id != "entra" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

func TestExchangeCode_BadRequest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	_, _, err := exchangeCode(t.Context(), srv.URL, "c", "v")
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("want expired error, got %v", err)
	}
}
