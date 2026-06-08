// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func decode(t *testing.T, line []byte) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("unmarshal %q: %v", line, err)
	}
	return m
}

func TestEnrichHandler_AddsTraceAndProject(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	h := newEnrichHandler(base, func(context.Context) (string, bool) { return "acme", true }, nil)
	logger := slog.New(h)

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x01},
		SpanID:  trace.SpanID{0x02},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	logger.InfoContext(ctx, "hello")

	m := decode(t, bytes.TrimSpace(buf.Bytes()))
	if m["trace_id"] == nil || m["span_id"] == nil {
		t.Fatalf("missing trace_id/span_id: %v", m)
	}
	if m["project"] != "acme" {
		t.Fatalf("project = %v, want acme", m["project"])
	}
}

func TestEnrichHandler_BareContextOmits(t *testing.T) {
	var buf bytes.Buffer
	h := newEnrichHandler(slog.NewJSONHandler(&buf, nil), nil, nil)
	slog.New(h).InfoContext(context.Background(), "hi")
	m := decode(t, bytes.TrimSpace(buf.Bytes()))
	if _, ok := m["trace_id"]; ok {
		t.Fatalf("trace_id present on bare ctx: %v", m)
	}
}

func TestEnrichHandler_WithAttrsAndGroupSurvive(t *testing.T) {
	var buf bytes.Buffer
	h := newEnrichHandler(slog.NewJSONHandler(&buf, nil), nil, nil)
	logger := slog.New(h).With("svc", "x").WithGroup("g").With("k", "v")
	logger.InfoContext(context.Background(), "msg")
	out := buf.String()
	if !strings.Contains(out, `"svc":"x"`) {
		t.Fatalf("WithAttrs dropped: %s", out)
	}
	if !strings.Contains(out, `"g":{"k":"v"}`) {
		t.Fatalf("WithGroup dropped: %s", out)
	}
}

func TestEnrichHandler_AddsIdentity(t *testing.T) {
	var buf bytes.Buffer
	h := newEnrichHandler(slog.NewJSONHandler(&buf, nil), nil,
		func(context.Context) (string, bool) { return "apikey:abc", true })
	slog.New(h).InfoContext(context.Background(), "hi")
	m := decode(t, bytes.TrimSpace(buf.Bytes()))
	if m["identity"] != "apikey:abc" {
		t.Fatalf("identity = %v, want apikey:abc", m["identity"])
	}
}

func TestEnrichHandler_NegativeAccessorOmits(t *testing.T) {
	var buf bytes.Buffer
	h := newEnrichHandler(slog.NewJSONHandler(&buf, nil),
		func(context.Context) (string, bool) { return "", false },
		func(context.Context) (string, bool) { return "x", false })
	slog.New(h).InfoContext(context.Background(), "hi")
	m := decode(t, bytes.TrimSpace(buf.Bytes()))
	if _, ok := m["project"]; ok {
		t.Fatalf("project present despite ok=false: %v", m)
	}
	if _, ok := m["identity"]; ok {
		t.Fatalf("identity present despite ok=false: %v", m)
	}
}

func TestEnrichHandler_TraceWithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := newEnrichHandler(slog.NewJSONHandler(&buf, nil), nil, nil)
	logger := slog.New(h).WithGroup("g")

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x01},
		SpanID:  trace.SpanID{0x02},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	logger.InfoContext(ctx, "msg")

	out := buf.String()
	if !strings.Contains(out, "trace_id") {
		t.Fatalf("trace_id missing with group: %s", out)
	}
}

// enabledStub is a minimal slog.Handler whose Enabled returns a configurable
// bool, used to verify enrichHandler delegates Enabled to the downstream.
type enabledStub struct{ enabled bool }

func (s enabledStub) Enabled(context.Context, slog.Level) bool  { return s.enabled }
func (s enabledStub) Handle(context.Context, slog.Record) error { return nil }
func (s enabledStub) WithAttrs([]slog.Attr) slog.Handler        { return s }
func (s enabledStub) WithGroup(string) slog.Handler             { return s }

func TestEnrichHandler_EnabledDelegates(t *testing.T) {
	h := newEnrichHandler(enabledStub{enabled: false}, nil, nil)
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Enabled = true, want false (should delegate to downstream)")
	}
}

func TestBuildLogger_FanoutPreservesAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	cfg := Config{} // no accessors, no lp
	logger := buildLogger(&cfg, base, nil).With("svc", "x").WithGroup("g").With("k", "v")
	logger.InfoContext(context.Background(), "msg")
	out := buf.String()
	if !strings.Contains(out, `"svc":"x"`) {
		t.Fatalf("WithAttrs dropped through fanout: %s", out)
	}
	if !strings.Contains(out, `"g":{"k":"v"}`) {
		t.Fatalf("WithGroup dropped through fanout: %s", out)
	}
}
