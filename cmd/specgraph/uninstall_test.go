// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
)

func withUninstallDeps(t *testing.T, deps uninstallDeps) {
	t.Helper()
	orig := uninstallFns
	uninstallFns = deps
	t.Cleanup(func() { uninstallFns = orig })
}

func TestDoUninstall_ManualModeErrors(t *testing.T) {
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "manual"}}
	err := doUninstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out)
	if err == nil {
		t.Fatal("manual mode should error")
	}
}

func TestDoUninstall_NotInstalledIsIdempotent(t *testing.T) {
	var calls []string
	withUninstallDeps(t, uninstallDeps{
		isInstalled:      func() bool { return false },
		serviceUninstall: func(string) error { calls = append(calls, "serviceUninstall"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: true}}
	if err := doUninstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("not-installed path should make no calls, got: %v", calls)
	}
	if !strings.Contains(out.String(), "not installed") {
		t.Errorf("should print informative notice, got: %q", out.String())
	}
}

func TestDoUninstall_HappyPathRemovesServiceAndStopsContainer(t *testing.T) {
	var calls []string
	withUninstallDeps(t, uninstallDeps{
		isInstalled:      func() bool { return true },
		serviceUninstall: func(string) error { calls = append(calls, "serviceUninstall"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: true}}
	if err := doUninstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Order: unregister the service first, then halt the container.
	want := []string{"serviceUninstall", "composeStop"}
	if len(calls) != 2 || calls[0] != want[0] || calls[1] != want[1] {
		t.Errorf("got calls %v, want %v", calls, want)
	}
	// The output should reassure the user that data is preserved.
	if !strings.Contains(out.String(), "data preserved") {
		t.Errorf("message should reassure about data, got: %q", out.String())
	}
}

func TestDoUninstall_NoDockerSkipsComposeStop(t *testing.T) {
	var calls []string
	withUninstallDeps(t, uninstallDeps{
		isInstalled:      func() bool { return true },
		serviceUninstall: func(string) error { calls = append(calls, "serviceUninstall"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: false}}
	if err := doUninstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range calls {
		if c == "composeStop" {
			t.Errorf("docker:false should skip composeStop, got: %v", calls)
		}
	}
}

func TestDoUninstall_MissingComposeFileSkipsStop(t *testing.T) {
	var calls []string
	withUninstallDeps(t, uninstallDeps{
		isInstalled:      func() bool { return true },
		serviceUninstall: func(string) error { calls = append(calls, "serviceUninstall"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeFileExist: func(string) bool { return false },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: true}}
	if err := doUninstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range calls {
		if c == "composeStop" {
			t.Errorf("missing compose file should skip composeStop, got: %v", calls)
		}
	}
}
