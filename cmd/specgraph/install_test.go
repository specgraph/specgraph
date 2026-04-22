// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
)

func withInstallDeps(t *testing.T, deps installDeps) {
	t.Helper()
	orig := installFns
	installFns = deps
	t.Cleanup(func() { installFns = orig })
}

func TestDoInstall_ManualModeErrors(t *testing.T) {
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "manual", Docker: true}}
	err := doInstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out)
	if err == nil {
		t.Fatal("manual mode should error")
	}
	if !strings.Contains(err.Error(), "manual") {
		t.Errorf("error should mention mode, got: %v", err)
	}
}

func TestDoInstall_AlreadyInstalledIsIdempotent(t *testing.T) {
	var calls []string
	withInstallDeps(t, installDeps{
		isInstalled:    func() bool { return true },
		generateDef:    func() (string, error) { calls = append(calls, "generateDef"); return "/plist", nil },
		composeUp:      func(string) error { calls = append(calls, "composeUp"); return nil },
		composeStop:    func(string) error { calls = append(calls, "composeStop"); return nil },
		serviceInstall: func(string) error { calls = append(calls, "serviceInstall"); return nil },
		removeFile:     func(string) error { calls = append(calls, "removeFile"); return nil },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: true}}
	if err := doInstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Idempotent path: ensure docker is up; don't re-register the service.
	want := []string{"composeUp"}
	if len(calls) != 1 || calls[0] != want[0] {
		t.Errorf("got calls %v, want %v", calls, want)
	}
}

func TestDoInstall_FreshInstallOrdersComposeBeforeService(t *testing.T) {
	var calls []string
	withInstallDeps(t, installDeps{
		isInstalled:    func() bool { return false },
		generateDef:    func() (string, error) { calls = append(calls, "generateDef"); return "/plist", nil },
		composeUp:      func(string) error { calls = append(calls, "composeUp"); return nil },
		composeStop:    func(string) error { calls = append(calls, "composeStop"); return nil },
		serviceInstall: func(string) error { calls = append(calls, "serviceInstall"); return nil },
		removeFile:     func(string) error { calls = append(calls, "removeFile"); return nil },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: true}}
	if err := doInstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Order matters: compose must come up before service registration, so
	// launchd doesn't start a binary that can't reach the DB.
	want := []string{"generateDef", "composeUp", "serviceInstall"}
	if len(calls) != 3 || calls[0] != want[0] || calls[1] != want[1] || calls[2] != want[2] {
		t.Errorf("got calls %v, want %v", calls, want)
	}
}

func TestDoInstall_ComposeUpFailureSkipsServiceInstall(t *testing.T) {
	var calls []string
	boom := errors.New("docker down")
	withInstallDeps(t, installDeps{
		isInstalled:    func() bool { return false },
		generateDef:    func() (string, error) { calls = append(calls, "generateDef"); return "/plist", nil },
		composeUp:      func(string) error { calls = append(calls, "composeUp"); return boom },
		composeStop:    func(string) error { calls = append(calls, "composeStop"); return nil },
		serviceInstall: func(string) error { calls = append(calls, "serviceInstall"); return nil },
		removeFile:     func(string) error { calls = append(calls, "removeFile"); return nil },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: true}}
	err := doInstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected compose-up failure to surface, got: %v", err)
	}
	for _, c := range calls {
		if c == "serviceInstall" {
			t.Errorf("serviceInstall must not run after composeUp failure, got: %v", calls)
		}
	}
}

func TestDoInstall_ServiceInstallFailureUnwindsContainerAndFile(t *testing.T) {
	var calls []string
	boom := errors.New("launchctl fail")
	withInstallDeps(t, installDeps{
		isInstalled:    func() bool { return false },
		generateDef:    func() (string, error) { calls = append(calls, "generateDef"); return "/tmp/plist", nil },
		composeUp:      func(string) error { calls = append(calls, "composeUp"); return nil },
		composeStop:    func(string) error { calls = append(calls, "composeStop"); return nil },
		serviceInstall: func(string) error { calls = append(calls, "serviceInstall"); return boom },
		removeFile:     func(string) error { calls = append(calls, "removeFile"); return nil },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: true}}
	err := doInstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected service install failure to surface, got: %v", err)
	}
	// Unwind order: composeStop (halt container we just started) then removeFile
	// (delete the plist we just wrote). Index comparison rather than
	// strings.Contains so that swapping the order fails the test.
	composeStopIdx, removeFileIdx := -1, -1
	for i, c := range calls {
		if c == "composeStop" {
			composeStopIdx = i
		}
		if c == "removeFile" {
			removeFileIdx = i
		}
	}
	if composeStopIdx == -1 || removeFileIdx == -1 {
		t.Errorf("expected composeStop + removeFile on unwind, got: %v", calls)
	}
	if composeStopIdx > removeFileIdx {
		t.Errorf("composeStop must precede removeFile, got: %v", calls)
	}
}

// Regression guard: when service.Install fails AND both cleanup steps fail,
// the user must see all three errors joined, not just the outer one.
func TestDoInstall_UnwindFailuresAreJoinedToOuterError(t *testing.T) {
	installBoom := errors.New("launchctl fail")
	stopBoom := errors.New("dockerd dead")
	rmBoom := errors.New("plist permission denied")
	withInstallDeps(t, installDeps{
		isInstalled:    func() bool { return false },
		generateDef:    func() (string, error) { return "/tmp/plist", nil },
		composeUp:      func(string) error { return nil },
		composeStop:    func(string) error { return stopBoom },
		serviceInstall: func(string) error { return installBoom },
		removeFile:     func(string) error { return rmBoom },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: true}}
	err := doInstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	for _, want := range []error{installBoom, stopBoom, rmBoom} {
		if !errors.Is(err, want) {
			t.Errorf("joined error should wrap %v, got: %v", want, err)
		}
	}
}

// Regression guard: if removeFile fails because the plist is already gone
// (ENOENT), that is expected cleanup success — do NOT surface it.
func TestDoInstall_UnwindIgnoresMissingPlist(t *testing.T) {
	installBoom := errors.New("launchctl fail")
	withInstallDeps(t, installDeps{
		isInstalled:    func() bool { return false },
		generateDef:    func() (string, error) { return "/tmp/plist", nil },
		composeUp:      func(string) error { return nil },
		composeStop:    func(string) error { return nil },
		serviceInstall: func(string) error { return installBoom },
		removeFile:     func(string) error { return fs.ErrNotExist },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: true}}
	err := doInstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out)
	if err == nil || !errors.Is(err, installBoom) {
		t.Fatalf("outer error should still surface, got: %v", err)
	}
	// The ENOENT from removeFile must NOT leak into the joined error.
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ENOENT from removeFile should be swallowed as expected cleanup, got: %v", err)
	}
}

func TestDoInstall_NoDockerSkipsComposeUp(t *testing.T) {
	var calls []string
	withInstallDeps(t, installDeps{
		isInstalled:    func() bool { return false },
		generateDef:    func() (string, error) { calls = append(calls, "generateDef"); return "/plist", nil },
		composeUp:      func(string) error { calls = append(calls, "composeUp"); return nil },
		composeStop:    func(string) error { calls = append(calls, "composeStop"); return nil },
		serviceInstall: func(string) error { calls = append(calls, "serviceInstall"); return nil },
		removeFile:     func(string) error { calls = append(calls, "removeFile"); return nil },
	})
	var out bytes.Buffer
	cfg := &config.GlobalConfig{Server: config.ServerSection{Mode: "service", Docker: false}}
	if err := doInstall(cfg, "/tmp/plist", "/tmp/compose.yaml", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range calls {
		if c == "composeUp" {
			t.Errorf("docker:false should skip composeUp, got: %v", calls)
		}
	}
}
