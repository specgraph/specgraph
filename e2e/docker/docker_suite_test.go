// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package docker_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var binaryPath string

func TestDocker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docker E2E Suite")
}

var _ = BeforeSuite(func() {
	// Build the specgraph binary once for the whole suite.
	tmpDir, err := os.MkdirTemp("", "specgraph-docker-e2e-*")
	Expect(err).NotTo(HaveOccurred())

	binaryPath = filepath.Join(tmpDir, "specgraph")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/specgraph")
	cmd.Dir = findProjectRoot()
	out, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "go build failed: %s", string(out))

	DeferCleanup(func() {
		os.RemoveAll(tmpDir)
	})
})

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
