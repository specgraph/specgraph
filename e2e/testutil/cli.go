// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package testutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
)

// CLIRunner runs the specgraph binary with a given config.
type CLIRunner struct {
	BinaryPath string
	ConfigPath string
}

// CLIResult holds the output of a CLI command.
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// BuildBinary builds the specgraph binary once into a temp dir.
// It returns the binary path, a cleanup function, and any error.
// Callers should invoke cleanup (e.g. via DeferCleanup) after the suite finishes.
func BuildBinary() (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "specgraph-e2e-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(tmpDir) }
	binaryPath := filepath.Join(tmpDir, "specgraph")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/specgraph")
	cmd.Dir = findProjectRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, &BuildError{Output: string(out), Err: err}
	}
	return binaryPath, cleanup, nil
}

// NewCLIRunner creates a CLIRunner from an already-built binary path and a
// config file path. Use this in BeforeEach after calling BuildBinary once in
// BeforeSuite.
func NewCLIRunner(binaryPath, configPath string) *CLIRunner {
	return &CLIRunner{BinaryPath: binaryPath, ConfigPath: configPath}
}

// Run executes the specgraph CLI with the given args.
func (c *CLIRunner) Run(args ...string) CLIResult {
	fullArgs := append([]string{"--config", c.ConfigPath}, args...)
	cmd := exec.Command(c.BinaryPath, fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = -1
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

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
