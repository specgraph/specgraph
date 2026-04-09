// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e || e2e_cli || e2e_agent

package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CLIRunner runs the specgraph binary with a given config.
type CLIRunner struct {
	BinaryPath string
	ConfigPath string
	CoverDir   string // GOCOVERDIR for coverage-instrumented binary; empty to skip
}

// CLIResult holds the output of a CLI command.
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// BuildBinary builds the specgraph binary once into a temp dir with coverage
// instrumentation enabled. It returns the binary path, a coverage directory
// path (for GOCOVERDIR), a cleanup function, and any error.
// Callers should invoke cleanup (e.g. via DeferCleanup) after the suite finishes.
func BuildBinary() (string, string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "specgraph-e2e-*")
	if err != nil {
		return "", "", nil, err
	}
	// Use SPECGRAPH_E2E_COVERDIR if set (CI), otherwise create inside tmpDir.
	coverDir := os.Getenv("SPECGRAPH_E2E_COVERDIR")
	if coverDir == "" {
		coverDir = filepath.Join(tmpDir, "coverdata")
	}
	if err := os.MkdirAll(coverDir, 0o750); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", nil, err
	}
	cleanup := func() { os.RemoveAll(tmpDir) }
	binaryPath := filepath.Join(tmpDir, "specgraph")
	cmd := exec.Command("go", "build", "-cover", "-o", binaryPath, "./cmd/specgraph") //nolint:gosec // binary path is constructed, not user input
	root, err := FindProjectRoot()
	if err != nil {
		cleanup()
		return "", "", nil, err
	}
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", "", nil, &BuildError{Output: string(out), Err: err}
	}
	return binaryPath, coverDir, cleanup, nil
}

// NewCLIRunner creates a CLIRunner from an already-built binary path, a config
// file path, and an optional coverage directory (from BuildBinary). Use this in
// BeforeEach after calling BuildBinary once in BeforeSuite.
func NewCLIRunner(binaryPath, configPath, coverDir string) *CLIRunner {
	return &CLIRunner{BinaryPath: binaryPath, ConfigPath: configPath, CoverDir: coverDir}
}

// Run executes the specgraph CLI with the given args.
func (c *CLIRunner) Run(args ...string) CLIResult {
	return c.RunInDir("", args...)
}

// RunInDir executes the specgraph CLI with the given args in the specified directory.
// If dir is empty, uses the current working directory.
func (c *CLIRunner) RunInDir(dir string, args ...string) CLIResult {
	fullArgs := append([]string{"--config", c.ConfigPath}, args...)
	cmd := exec.Command(c.BinaryPath, fullArgs...) //nolint:gosec // binary path from BuildBinary, not user input
	if dir != "" {
		cmd.Dir = dir
	}
	if c.CoverDir != "" {
		cmd.Env = append(os.Environ(), "GOCOVERDIR="+c.CoverDir)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = -1
		stderr.WriteString(err.Error())
	}
	return CLIResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: exitCode}
}

// BuildError wraps a failed go build.
type BuildError struct {
	Output string
	Err    error
}

func (e *BuildError) Error() string {
	return "go build failed: " + e.Output
}

// FindProjectRoot walks up from the working directory to find the go.mod root.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
