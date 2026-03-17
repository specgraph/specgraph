// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e_cli

package cli_test

import (
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

var _ = Describe("CLI Pipeline", Ordered, func() {
	const slug = "cli-pipeline-test"

	It("creates a spec", func() {
		result := cli.RunInDir(workDir, "create", slug, "--intent", "CLI pipeline test")
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Created: " + slug))
	})

	It("sparks the spec", func() {
		result := cli.RunInDir(workDir, "spark", slug, "--seed", "CLI pipeline E2E seed idea")
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Sparked: " + slug))
	})

	It("shapes the spec", func() {
		result := cli.RunInDir(workDir, "shape", slug, "--json-file", testdataPath("shape-output.json"))
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Shaped: " + slug))
	})

	It("specifies the spec", func() {
		result := cli.RunInDir(workDir, "specify", slug, "--json-file", testdataPath("specify-output.json"))
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Specified: " + slug))
	})

	It("decomposes the spec", func() {
		result := cli.RunInDir(workDir, "decompose", slug, "--json-file", testdataPath("decompose-output.json"))
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Decomposed: " + slug))
	})

	It("approves the spec", func() {
		result := cli.RunInDir(workDir, "approve", slug)
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Approved: " + slug))
	})

	It("claims the spec", func() {
		result := cli.RunInDir(workDir, "claim", slug, "--agent", "cli-e2e-agent")
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Claimed: " + slug))
	})

	It("generates an execution bundle", func() {
		result := cli.RunInDir(workDir, "bundle", slug)
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).NotTo(BeEmpty())
	})

	It("shows execution events (empty)", func() {
		result := cli.RunInDir(workDir, "progress", slug)
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		// No events have been written via CLI, so expect empty or "No execution events found."
		Expect(result.Stdout).To(ContainSubstring("No execution events found"))
	})

	It("shows the spec with correct stage", func() {
		result := cli.RunInDir(workDir, "show", slug)
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring(slug))
		Expect(result.Stdout).To(ContainSubstring("approved"))
	})
})
