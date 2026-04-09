// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package sync

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// DefaultExecTimeout is the maximum time a subprocess is allowed to run
// if the caller's context has no deadline.
const DefaultExecTimeout = 30 * time.Second

// ExecRunner implements CommandRunner using os/exec.
type ExecRunner struct{}

// NewExecRunner creates a new ExecRunner.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// Run executes a command and returns its stdout output.
// Stderr is captured separately to prevent it from corrupting stdout parsing.
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultExecTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			stderrMsg := stderr.String()
			const maxStderr = 512
			if len(stderrMsg) > maxStderr {
				stderrMsg = stderrMsg[:maxStderr] + "... (truncated)"
			}
			return out, fmt.Errorf("exec %s: %w (stderr: %s)", name, err, stderrMsg)
		}
		return out, fmt.Errorf("exec %s: %w", name, err)
	}
	if stderr.Len() > 0 {
		slog.Debug("exec stderr (non-fatal)", "cmd", name, "stderr", stderr.String())
	}
	return out, nil
}
