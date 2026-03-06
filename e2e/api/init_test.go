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
		cli, err = testutil.NewCLI(configPath)
		Expect(err).NotTo(HaveOccurred())
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

	It("rejects init when config already exists", func() {
		// First init should succeed
		result := cli.Run("init", "--yes")
		Expect(result.ExitCode).To(Equal(0), "first init stderr: %s", result.Stderr)

		// Second init should fail
		result = cli.Run("init", "--yes")
		Expect(result.ExitCode).NotTo(Equal(0), "second init should fail")
	})
})
