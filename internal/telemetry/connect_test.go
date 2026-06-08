// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"net/http"
	"testing"
)

func TestServerInterceptor_DisabledReturnsNil(t *testing.T) {
	ic, err := ServerInterceptor(false)
	if err != nil {
		t.Fatal(err)
	}
	if ic != nil {
		t.Fatal("disabled ServerInterceptor should be nil")
	}
}

func TestServerInterceptor_EnabledNonNil(t *testing.T) {
	ic, err := ServerInterceptor(true)
	if err != nil {
		t.Fatal(err)
	}
	if ic == nil {
		t.Fatal("enabled ServerInterceptor should be non-nil")
	}
}

func TestClientInterceptor_Gating(t *testing.T) {
	ic, err := ClientInterceptor(false)
	if err != nil {
		t.Fatal(err)
	}
	if ic != nil {
		t.Fatal("disabled ClientInterceptor should be nil")
	}
	ic, err = ClientInterceptor(true)
	if err != nil {
		t.Fatal(err)
	}
	if ic == nil {
		t.Fatal("enabled ClientInterceptor should be non-nil")
	}
}

func TestLoopbackTransport_Gating(t *testing.T) {
	base := http.DefaultTransport
	// Disabled: must return the SAME RoundTripper (identity passthrough) — this
	// is the zero-overhead guarantee.
	if got := LoopbackTransport(false, base); got != base {
		t.Fatal("disabled LoopbackTransport must return base unchanged")
	}
	// Enabled: must wrap (different RoundTripper).
	if got := LoopbackTransport(true, base); got == base {
		t.Fatal("enabled LoopbackTransport must wrap base")
	}
}
