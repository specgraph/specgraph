// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"testing"

	"github.com/spf13/pflag"
)

func newFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(fs)
	return fs
}

func TestResolveConfig_DefaultsOff(t *testing.T) {
	fs := newFlagSet()
	got := ResolveConfig(fs, func(string) string { return "" })
	if got.Enabled {
		t.Fatalf("Enabled = true, want false by default")
	}
	if got.SampleRatio != 1.0 {
		t.Fatalf("SampleRatio = %v, want 1.0", got.SampleRatio)
	}
	if !got.LogsExport {
		t.Fatalf("LogsExport = false, want true default")
	}
}

func TestResolveConfig_FlagBeatsEnv(t *testing.T) {
	fs := newFlagSet()
	if err := fs.Parse([]string{"--otel", "--otel-sample-ratio", "0.25"}); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"SPECGRAPH_OTEL_ENABLED":      "false",
		"SPECGRAPH_OTEL_SAMPLE_RATIO": "0.9",
	}
	got := ResolveConfig(fs, func(k string) string { return env[k] })
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true (flag set beats env)")
	}
	if got.SampleRatio != 0.25 {
		t.Fatalf("SampleRatio = %v, want 0.25 (flag wins)", got.SampleRatio)
	}
}

func TestResolveConfig_EnvWhenFlagUnset(t *testing.T) {
	fs := newFlagSet()
	env := map[string]string{
		"SPECGRAPH_OTEL_ENABLED": "true",
		"SPECGRAPH_OTEL_LOGS":    "false",
	}
	got := ResolveConfig(fs, func(k string) string { return env[k] })
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true from env")
	}
	if got.LogsExport {
		t.Fatalf("LogsExport = true, want false from env")
	}
}
