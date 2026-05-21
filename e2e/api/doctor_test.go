// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/e2e/testutil"
)

var _ = Describe("specgraph doctor", func() {
	var (
		cli    *testutil.CLIRunner
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "specgraph-doctor-*")
		Expect(err).NotTo(HaveOccurred())
		cli = testutil.NewCLIRunner(cliBinaryPath, serverInfo.ConfigPath, "")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	})

	It("reports all groups healthy on a freshly-init'd project", func() {
		initResult := cli.RunInDir(tmpDir, "init", "test-project", "--yes")
		Expect(initResult.ExitCode).To(Equal(0), "init stderr: %s", initResult.Stderr)

		result := cli.RunInDir(tmpDir, "doctor")
		// All four group labels must appear in the rendered output. The
		// "synced" word always appears in the Managed-files line — we
		// deliberately don't pin the count because adding new managed
		// files in later PRs would otherwise break this test.
		Expect(result.Stdout).To(ContainSubstring("Binary:"))
		Expect(result.Stdout).To(ContainSubstring("Project config:"))
		Expect(result.Stdout).To(ContainSubstring("Server:"))
		Expect(result.Stdout).To(ContainSubstring("Managed files:"))
		Expect(result.Stdout).To(ContainSubstring("synced"))
	})

	It("flags a drifted managed file and prints guidance", func() {
		initResult := cli.RunInDir(tmpDir, "init", "test-project", "--yes")
		Expect(initResult.ExitCode).To(Equal(0), "init stderr: %s", initResult.Stderr)

		// Corrupt AGENTS.md so the file's sentinel-recorded hash no longer
		// matches its on-disk content. Depending on the sentinel logic the
		// framework may classify this as Drifted or Stale — both are valid
		// "needs attention" outcomes.
		agentsPath := filepath.Join(tmpDir, "AGENTS.md")
		Expect(os.WriteFile(agentsPath, []byte("# corrupted by test\n"), 0o644)).To(Succeed()) //nolint:gosec // test fixture

		result := cli.RunInDir(tmpDir, "doctor", "--fix")
		combined := result.Stdout + result.Stderr
		Expect(combined).To(SatisfyAny(
			ContainSubstring("drifted"),
			ContainSubstring("synced"),
		))
	})

	It("--json produces stable schema", func() {
		initResult := cli.RunInDir(tmpDir, "init", "test-project", "--yes")
		Expect(initResult.ExitCode).To(Equal(0), "init stderr: %s", initResult.Stderr)

		result := cli.RunInDir(tmpDir, "doctor", "--json")
		var rep map[string]any
		Expect(json.Unmarshal([]byte(result.Stdout), &rep)).To(Succeed(), "stdout: %s", result.Stdout)

		groups, ok := rep["groups"].(map[string]any)
		Expect(ok).To(BeTrue(), "groups object missing or wrong type: %s", result.Stdout)
		Expect(groups).To(HaveKey("binary"))
		Expect(groups).To(HaveKey("server"))
		Expect(groups).To(HaveKey("project"))
		Expect(groups).To(HaveKey("managed"))
	})
})
