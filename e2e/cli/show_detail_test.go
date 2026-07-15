// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e_cli

package cli_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Show Stage Detail", Ordered, func() {
	const slug = "show-detail-test"

	It("sparks the spec", func() {
		result := cli.RunInDir(workDir, "spark", slug, "--seed", "Show-detail E2E seed idea for caching layer")
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Sparked: " + slug))
	})

	It("show renders spark detail", func() {
		result := cli.RunInDir(workDir, "show", slug)
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("## Spark"))
		Expect(result.Stdout).To(ContainSubstring("Show-detail E2E seed idea for caching layer"))
	})

	It("shapes the spec", func() {
		result := cli.RunInDir(workDir, "shape", slug, "--json-file", testdataPath("shape-output.json"), "--conversation", testdataPath("conversation-input.json"))
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Shaped: " + slug))
	})

	It("show renders shape detail", func() {
		result := cli.RunInDir(workDir, "show", slug)
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("## Shape"))
		Expect(result.Stdout).To(ContainSubstring("Scope In"))
		Expect(result.Stdout).To(ContainSubstring("Approaches"))
	})
})

var _ = Describe("Conversation Commands", Ordered, func() {
	const slug = "conversation-test"

	It("sparks the spec for conversation tests", func() {
		result := cli.RunInDir(workDir, "spark", slug, "--seed", "Conversation E2E seed")
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Sparked: " + slug))
	})

	It("records conversation exchanges", func() {
		result := cli.RunInDir(workDir,
			"conversation", "record", slug,
			"--stage", "spark",
			"--json-file", testdataPath("conversation-exchanges.json"),
		)
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Recorded conversation"))
	})

	It("lists conversation logs", func() {
		result := cli.RunInDir(workDir, "conversation", "list", slug)
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)
		Expect(result.Stdout).To(ContainSubstring("Probe:"))
		Expect(result.Stdout).To(ContainSubstring("User:"))
	})
})
