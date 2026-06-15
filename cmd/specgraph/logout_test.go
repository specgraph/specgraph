// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRevokeSession(t *testing.T) {
	t.Parallel()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	if err := revokeSession(t.Context(), srv.URL, "spgr_ws_abc"); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer spgr_ws_abc" {
		t.Fatalf("auth header = %q", gotAuth)
	}
}

func TestRevokeSession_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if err := revokeSession(t.Context(), srv.URL, "spgr_ws_abc"); err == nil {
		t.Fatal("expected error on 500")
	}
}
