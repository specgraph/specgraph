// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/seanb4t/specgraph/e2e/testutil"
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

		configPath := filepath.Join(tmpDir, ".specgraph", "config.yaml")
		cli = testutil.NewCLIRunner(cliBinaryPath, configPath)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("creates config with defaults in non-interactive mode", func() {
		result := cli.Run("init", "--yes")
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)

		// Verify config file was created
		configPath := filepath.Join(tmpDir, ".specgraph", "config.yaml")
		_, err := os.Stat(configPath)
		Expect(err).NotTo(HaveOccurred(), "config file should exist")
	})

	It("generates constitution draft with --scan", func() {
		// Create a go.mod so the scanner detects a Go project.
		goMod := filepath.Join(tmpDir, "go.mod")
		Expect(os.WriteFile(goMod, []byte("module example.com/test\n\ngo 1.25\n"), 0o644)).To(Succeed())

		result := cli.Run("init", "--yes", "--scan")
		Expect(result.ExitCode).To(Equal(0), "stderr: %s", result.Stderr)

		// Verify constitution file was created by the scan.
		constitutionPath := filepath.Join(tmpDir, ".specgraph", "constitution.yaml")
		_, err := os.Stat(constitutionPath)
		Expect(err).NotTo(HaveOccurred(), "constitution file should exist after --scan")
	})

	It("rejects init when config already exists", func() {
		// First init should succeed
		result := cli.Run("init", "--yes")
		Expect(result.ExitCode).To(Equal(0), "first init stderr: %s", result.Stderr)

		// Second init should fail
		result = cli.Run("init", "--yes")
		Expect(result.ExitCode).NotTo(Equal(0), "second init should fail")
	})
})
