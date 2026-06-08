// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestInit_DisabledIsNoOp(t *testing.T) {
	// The default global TracerProvider in otel is an unset delegate
	// (*global.tracerProvider) that forwards to no-op until SetTracerProvider
	// is called. Capture it before Init so we can assert the disabled path
	// leaves it untouched (i.e. installs no SDK provider).
	before := otel.GetTracerProvider()

	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	tel, err := Init(context.Background(), Config{
		Enabled:    false,
		Role:       RoleCLI,
		LogHandler: base,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if tel.Logger == nil {
		t.Fatal("Logger is nil")
	}
	// Disabled Init must not set/replace the global TracerProvider.
	if otel.GetTracerProvider() != before {
		t.Fatalf("disabled Init changed the global TracerProvider: %T", otel.GetTracerProvider())
	}
	if err := tel.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestInit_NilLogHandlerRejected(t *testing.T) {
	_, err := Init(context.Background(), Config{Enabled: false, Role: RoleCLI, LogHandler: nil})
	if err == nil {
		t.Fatal("expected error for nil LogHandler")
	}
}
