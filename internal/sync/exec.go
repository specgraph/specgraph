// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// ExecRunner implements CommandRunner using os/exec.
type ExecRunner struct{}

// NewExecRunner creates a new ExecRunner.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// Run executes a command and returns its stdout output.
// Stderr is captured separately to prevent it from corrupting stdout parsing.
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return out, fmt.Errorf("exec %s: %w (stderr: %s)", name, err, stderr.String())
		}
		return out, fmt.Errorf("exec %s: %w", name, err)
	}
	return out, nil
}
