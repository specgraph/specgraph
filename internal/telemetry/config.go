// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"strconv"

	"github.com/spf13/pflag"
)

// Flag names registered on the root persistent flag set.
const (
	flagEnabled     = "otel"
	flagSampleRatio = "otel-sample-ratio"
	flagLogs        = "otel-logs"
)

// Env names checked when the corresponding flag is not explicitly set.
const (
	envEnabled     = "SPECGRAPH_OTEL_ENABLED"
	envSampleRatio = "SPECGRAPH_OTEL_SAMPLE_RATIO"
	envLogs        = "SPECGRAPH_OTEL_LOGS"
)

// RegisterFlags adds the three SpecGraph-native telemetry knobs to fs.
// Call on rootCmd.PersistentFlags() so every command accepts them.
func RegisterFlags(fs *pflag.FlagSet) {
	fs.Bool(flagEnabled, false, "Enable OpenTelemetry export (env: "+envEnabled+")")
	fs.Float64(flagSampleRatio, 1.0, "Trace sample ratio 0.0-1.0 (env: "+envSampleRatio+")")
	fs.Bool(flagLogs, true, "Export logs over OTLP when telemetry is enabled (env: "+envLogs+")")
}

// ResolveConfig builds a partial Config from flags + env with precedence
// flag(if Changed) > env > default. getenv is injected for testability
// (pass os.Getenv in production). Role, LogHandler, Version, ServiceName,
// and the context accessors are filled in by the caller.
func ResolveConfig(fs *pflag.FlagSet, getenv func(string) string) Config {
	return Config{
		Enabled:     resolveBool(fs, flagEnabled, envEnabled, getenv, false),
		SampleRatio: resolveFloat(fs, flagSampleRatio, envSampleRatio, getenv, 1.0),
		LogsExport:  resolveBool(fs, flagLogs, envLogs, getenv, true),
	}
}

func resolveBool(fs *pflag.FlagSet, flag, env string, getenv func(string) string, def bool) bool {
	if fs.Changed(flag) {
		if v, err := fs.GetBool(flag); err == nil {
			return v
		}
	}
	if s := getenv(env); s != "" {
		if v, err := strconv.ParseBool(s); err == nil {
			return v
		}
	}
	return def
}

func resolveFloat(fs *pflag.FlagSet, flag, env string, getenv func(string) string, def float64) float64 {
	if fs.Changed(flag) {
		if v, err := fs.GetFloat64(flag); err == nil {
			return v
		}
	}
	if s := getenv(env); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	}
	return def
}
