// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/e2e/testutil"
)

var _ = Describe("init command", func() {
	var (
		cli    *testutil.CLIRunner
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "specgraph-init-*")
		Expect(err).NotTo(HaveOccurred())

		// Point --config at the running test server so init doesn't try to
		// start its own Docker compose stack.
		cli = testutil.NewCLIRunner(cliBinaryPath, serverInfo.ConfigPath, "")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	})

	It("creates .specgraph.yaml with project slug", func() {
		// Run init from tmpDir so it writes there.
		result := cli.RunInDir(tmpDir, "init", "test-project", "--yes")
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)

		// Verify .specgraph.yaml was created.
		yamlPath := filepath.Join(tmpDir, ".specgraph.yaml")
		_, err := os.Stat(yamlPath)
		Expect(err).NotTo(HaveOccurred(), ".specgraph.yaml should exist")

		// Verify it contains the project slug.
		data, err := os.ReadFile(yamlPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("test-project"))
	})

	It("is idempotent when re-run with the same slug", func() {
		// First init creates .specgraph.yaml.
		result := cli.RunInDir(tmpDir, "init", "test-project", "--yes")
		Expect(result.ExitCode).To(Equal(0), "first init stderr: %s", result.Stderr)

		// Second init with the same slug succeeds as a no-op.
		result = cli.RunInDir(tmpDir, "init", "test-project", "--yes")
		Expect(result.ExitCode).To(Equal(0), "second init stderr: %s", result.Stderr)
	})

	It("rejects init when slug arg conflicts with existing .specgraph.yaml", func() {
		// First init pins the slug.
		result := cli.RunInDir(tmpDir, "init", "test-project", "--yes")
		Expect(result.ExitCode).To(Equal(0), "first init stderr: %s", result.Stderr)

		// Re-running with a different slug must refuse.
		result = cli.RunInDir(tmpDir, "init", "other-project", "--yes")
		Expect(result.ExitCode).NotTo(Equal(0), "conflicting-slug init should fail")
		Expect(result.Stderr).To(ContainSubstring("cannot change project slug"))
	})
})
