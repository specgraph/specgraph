// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
)

// withDownDeps swaps down.go's package-level function vars for the duration
// of a single test and restores them on cleanup.
func withDownDeps(t *testing.T, deps downDeps) {
	t.Helper()
	orig := downFns
	downFns = deps
	t.Cleanup(func() { downFns = orig })
}

func serviceDockerCfg() *config.GlobalConfig {
	return &config.GlobalConfig{
		Server: config.ServerSection{Mode: "service", Docker: true},
	}
}

func manualDockerCfg() *config.GlobalConfig {
	return &config.GlobalConfig{
		Server: config.ServerSection{Mode: "manual", Docker: true},
	}
}

func serviceNoDockerCfg() *config.GlobalConfig {
	return &config.GlobalConfig{
		Server: config.ServerSection{Mode: "service", Docker: false},
	}
}

// Retirement error fires before any side effects.
func TestDoDown_RMFlagErrorsWithoutSideEffects(t *testing.T) {
	var calls []string
	withDownDeps(t, downDeps{
		stopService:      func() error { calls = append(calls, "stopService"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeDownVols:  func(string) error { calls = append(calls, "composeDownVols"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	err := doDown(serviceDockerCfg(), downFlags{rmRetired: true}, strings.NewReader(""), &out, false)
	if err == nil {
		t.Fatal("expected retirement error, got nil")
	}
	if !strings.Contains(err.Error(), "retired") {
		t.Errorf("error should mention --rm retirement, got: %v", err)
	}
	if !strings.Contains(err.Error(), "uninstall") || !strings.Contains(err.Error(), "--purge") {
		t.Errorf("error should point at uninstall + --purge, got: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("no side effects expected, got: %v", calls)
	}
}

// Default `specgraph down`: service.Stop + ComposeStop, never ComposeDownVolumes.
func TestDoDown_DefaultStopsServiceAndContainer(t *testing.T) {
	var calls []string
	withDownDeps(t, downDeps{
		stopService:      func() error { calls = append(calls, "stopService"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeDownVols:  func(string) error { calls = append(calls, "composeDownVols"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	if err := doDown(serviceDockerCfg(), downFlags{}, strings.NewReader(""), &out, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"stopService", "composeStop"}
	if len(calls) != len(want) || calls[0] != want[0] || calls[1] != want[1] {
		t.Errorf("got calls %v, want %v", calls, want)
	}
}

// Manual mode: skip service.Stop but still ComposeStop.
func TestDoDown_ManualModeSkipsServiceStop(t *testing.T) {
	var calls []string
	withDownDeps(t, downDeps{
		stopService:      func() error { calls = append(calls, "stopService"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeDownVols:  func(string) error { calls = append(calls, "composeDownVols"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	if err := doDown(manualDockerCfg(), downFlags{}, strings.NewReader(""), &out, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range calls {
		if c == "stopService" {
			t.Errorf("manual mode should not call stopService, got: %v", calls)
		}
	}
}

// No-docker config: skip compose calls entirely.
func TestDoDown_NoDockerSkipsComposeCalls(t *testing.T) {
	var calls []string
	withDownDeps(t, downDeps{
		stopService:      func() error { calls = append(calls, "stopService"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeDownVols:  func(string) error { calls = append(calls, "composeDownVols"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	if err := doDown(serviceNoDockerCfg(), downFlags{}, strings.NewReader(""), &out, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range calls {
		if c == "composeStop" || c == "composeDownVols" {
			t.Errorf("no-docker mode should skip compose, got: %v", calls)
		}
	}
}

// --purge without --yes off a TTY must error without destroying.
func TestDoDown_PurgeNonTTYWithoutYesErrors(t *testing.T) {
	var calls []string
	withDownDeps(t, downDeps{
		stopService:      func() error { calls = append(calls, "stopService"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeDownVols:  func(string) error { calls = append(calls, "composeDownVols"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	err := doDown(serviceDockerCfg(), downFlags{purge: true}, strings.NewReader(""), &out, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	for _, c := range calls {
		if c == "composeDownVols" {
			t.Errorf("destructive call made despite error: %v", calls)
		}
	}
}

// --purge --yes off a TTY: service.Stop + ComposeDownWithVolumes (NOT ComposeStop).
func TestDoDown_PurgeYesCallsDownWithVolumes(t *testing.T) {
	var calls []string
	withDownDeps(t, downDeps{
		stopService:      func() error { calls = append(calls, "stopService"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeDownVols:  func(string) error { calls = append(calls, "composeDownVols"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	if err := doDown(serviceDockerCfg(), downFlags{purge: true, yes: true}, strings.NewReader(""), &out, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.Join(calls, ",")
	if !strings.Contains(got, "composeDownVols") {
		t.Errorf("expected composeDownVols call, got: %v", calls)
	}
	if strings.Contains(got, "composeStop") {
		t.Errorf("should not call composeStop when purging, got: %v", calls)
	}
}

// --purge on TTY with 'y' input proceeds.
func TestDoDown_PurgeTTYAcceptsY(t *testing.T) {
	var calls []string
	withDownDeps(t, downDeps{
		stopService:      func() error { calls = append(calls, "stopService"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeDownVols:  func(string) error { calls = append(calls, "composeDownVols"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	err := doDown(serviceDockerCfg(), downFlags{purge: true}, strings.NewReader("y\n"), &out, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.Join(calls, ","), "composeDownVols") {
		t.Errorf("expected composeDownVols call, got: %v", calls)
	}
	if !strings.Contains(out.String(), "specgraph-data") {
		t.Errorf("prompt should mention volume name, got: %q", out.String())
	}
}

// Compose file missing: skip compose entirely, don't error.
func TestDoDown_MissingComposeFileSkipsCompose(t *testing.T) {
	var calls []string
	withDownDeps(t, downDeps{
		stopService:      func() error { calls = append(calls, "stopService"); return nil },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeDownVols:  func(string) error { calls = append(calls, "composeDownVols"); return nil },
		composeFileExist: func(string) bool { return false },
	})
	var out bytes.Buffer
	if err := doDown(serviceDockerCfg(), downFlags{}, strings.NewReader(""), &out, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range calls {
		if c == "composeStop" || c == "composeDownVols" {
			t.Errorf("missing compose file should skip compose, got: %v", calls)
		}
	}
}

// Regression guard: a compose failure must NOT mask an already-captured
// service-stop error. Both errors must appear in the joined output so the
// user doesn't miss the service-stop problem.
func TestDoDown_ComposeFailureDoesNotMaskServiceStopError(t *testing.T) {
	stopBoom := errors.New("service stop boom")
	composeBoom := errors.New("compose stop boom")
	withDownDeps(t, downDeps{
		stopService:      func() error { return stopBoom },
		composeStop:      func(string) error { return composeBoom },
		composeDownVols:  func(string) error { return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	err := doDown(serviceDockerCfg(), downFlags{}, strings.NewReader(""), &out, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, stopBoom) {
		t.Errorf("service stop error must be surfaced, got: %v", err)
	}
	if !errors.Is(err, composeBoom) {
		t.Errorf("compose error must be surfaced, got: %v", err)
	}
}

// Service stop error surfaces but doesn't prevent compose stop.
func TestDoDown_ServiceStopErrorSurfacesAfterComposeStop(t *testing.T) {
	var calls []string
	sentinel := errors.New("service stop boom")
	withDownDeps(t, downDeps{
		stopService:      func() error { calls = append(calls, "stopService"); return sentinel },
		composeStop:      func(string) error { calls = append(calls, "composeStop"); return nil },
		composeDownVols:  func(string) error { calls = append(calls, "composeDownVols"); return nil },
		composeFileExist: func(string) bool { return true },
	})
	var out bytes.Buffer
	err := doDown(serviceDockerCfg(), downFlags{}, strings.NewReader(""), &out, false)
	if err == nil || !errors.Is(err, sentinel) {
		t.Errorf("expected service stop error surfaced, got: %v", err)
	}
	if len(calls) != 2 || calls[0] != "stopService" || calls[1] != "composeStop" {
		t.Errorf("compose should still run before surfacing stop error, got: %v", calls)
	}
}
