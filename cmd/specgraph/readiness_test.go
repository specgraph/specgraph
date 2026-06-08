// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadinessPinger_NotReadyBeforeSet(t *testing.T) {
	p := newReadinessPinger()
	err := p.Ping(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errStoreNotReady)
}

func TestReadinessPinger_DelegatesAfterSet(t *testing.T) {
	p := newReadinessPinger()

	p.set(&stubPinger{}) // stubPinger is defined in serve_probes_test.go (same package)
	assert.NoError(t, p.Ping(context.Background()))

	want := errors.New("db down")
	p.set(&stubPinger{err: want})
	assert.ErrorIs(t, p.Ping(context.Background()), want)
}

func TestNotReadyHandler_Returns503(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)

	notReadyHandler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "5", rec.Header().Get("Retry-After"))
	assert.Contains(t, rec.Body.String(), "storage not ready")
}

func TestAtomicHandler_SwapsBehaviour(t *testing.T) {
	a := newAtomicHandler(notReadyHandler())

	rec1 := httptest.NewRecorder()
	a.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec1.Code)

	a.set(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec2 := httptest.NewRecorder()
	a.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTeapot, rec2.Code)
}
