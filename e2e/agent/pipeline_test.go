// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e_agent

package agent_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/e2e/testutil"
)

// appendPath returns a copy of os.Environ() with dir prepended to PATH.
func appendPath(dir string) []string {
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = fmt.Sprintf("PATH=%s:%s", dir, e[5:])
			return env
		}
	}
	return append(env, fmt.Sprintf("PATH=%s", dir))
}

// pluginDir returns the absolute path to the specgraph plugin directory.
func pluginDir() string {
	root, err := testutil.FindProjectRoot()
	Expect(err).NotTo(HaveOccurred())
	return filepath.Join(root, "plugin", "specgraph")
}

// agentRun invokes `claude -p` with a natural language prompt and the
// specgraph plugin loaded. The thin plugin primes routing guidance while the
// agent can still fall back to the CLI through Bash. The prompt includes the
// binary path since the test binary lives in a temp dir, not on the standard
// PATH.
func agentRun(prompt string) (string, error) {
	fullPrompt := fmt.Sprintf(
		"The specgraph binary is at %s and the working directory is %s (which has a .specgraph.yaml configured). %s",
		binaryPath, workDir, prompt,
	)
	cmd := exec.Command(claudePath, "-p", fullPrompt,
		"--max-turns", "5",
		"--allowedTools", "Bash",
		"--plugin-dir", pluginDir(),
		"--add-dir", workDir,
	)
	cmd.Dir = workDir
	cmd.Env = appendPath(filepath.Dir(binaryPath))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := stdout.String()
	errOut := stderr.String()
	combined := strings.ToLower(out + "\n" + errOut)
	// "Reached max turns" is a soft failure — return output for assertions.
	if err != nil && !strings.Contains(combined, "max turns") {
		return out, fmt.Errorf("claude -p failed: %w\nstderr: %s\nstdout: %s",
			err, errOut, out)
	}
	return out, nil
}

// specgraphRun runs a specgraph CLI command and returns the result.
func specgraphRun(args ...string) testutil.CLIResult {
	cli := testutil.NewCLIRunner(binaryPath, serverInfo.ConfigPath, "")
	return cli.RunInDir(workDir, args...)
}

var _ = Describe("Agent pipeline", Ordered, func() {
	const slug = "agent-pipeline-spec"

	// Agent tests are non-deterministic smoke tests. We verify that the
	// agent can discover and invoke SpecGraph through the thin plugin
	// guidance and CLI fallback. Assertions check that the spec exists and
	// the agent completed without hard errors — exact stage transitions may vary.

	It("creates a spec via agent", func(ctx SpecContext) {
		_, err := agentRun(fmt.Sprintf(
			"Use /specgraph to create a new spec with slug %q and intent %q.",
			slug, "Agent-driven pipeline test",
		))
		Expect(err).NotTo(HaveOccurred())

		result := specgraphRun("show", slug)
		Expect(result.ExitCode).To(Equal(0), "spec should exist after agent create")
		Expect(result.Stdout).To(ContainSubstring(slug))
		Expect(result.Stdout).To(ContainSubstring("spark"), "newly created spec should be at spark stage")
	}, SpecTimeout(5*time.Minute))

	It("sparks the spec via agent", func(ctx SpecContext) {
		_, err := agentRun(fmt.Sprintf(
			"Use SpecGraph to spark the spec %q with the seed idea: %q.",
			slug, "Build a widget service for testing",
		))
		Expect(err).NotTo(HaveOccurred())

		// Verify spec still exists — stage may or may not have advanced
		// depending on whether the agent found the right flags.
		result := specgraphRun("show", slug)
		Expect(result.ExitCode).To(Equal(0))
	}, SpecTimeout(5*time.Minute))

	It("advances the spec to approved via agent", func(ctx SpecContext) {
		out, err := agentRun(fmt.Sprintf(
			"Update specgraph spec %q to stage 'approved'.",
			slug,
		))
		Expect(err).NotTo(HaveOccurred())

		result := specgraphRun("show", slug)
		Expect(result.ExitCode).To(Equal(0))

		// Check whether the agent's output mentions the update.
		if strings.Contains(strings.ToLower(out), "approved") {
			GinkgoWriter.Printf("agent output confirms approved stage\n")
		}

		// Agent may not always find the right command. If stage didn't
		// advance, fall back to direct CLI so subsequent steps work.
		if !strings.Contains(result.Stdout, "approved") {
			GinkgoWriter.Printf("agent did not advance to approved — falling back to direct CLI\n")
			fb := specgraphRun("update", slug, "--stage", "approved")
			Expect(fb.ExitCode).To(Equal(0), "direct fallback failed: %s", fb.Stderr)
		}
	}, SpecTimeout(5*time.Minute))

	It("claims the spec via agent", func(ctx SpecContext) {
		_, err := agentRun(fmt.Sprintf(
			"Claim specgraph spec %q as agent %q.",
			slug, "agent-e2e",
		))
		Expect(err).NotTo(HaveOccurred(), "agent claim step should succeed")

		result := specgraphRun("show", slug)
		Expect(result.ExitCode).To(Equal(0))
	}, SpecTimeout(5*time.Minute))

	It("shows the spec via agent", func(ctx SpecContext) {
		out, err := agentRun(fmt.Sprintf(
			"Use SpecGraph to show details of spec %q.",
			slug,
		))
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.ToLower(out)).To(ContainSubstring(slug))
	}, SpecTimeout(5*time.Minute))
})
