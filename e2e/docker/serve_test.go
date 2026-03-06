// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package docker_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Docker mode serve", Ordered, func() {
	var (
		projectDir string
		configPath string
		cmd        *exec.Cmd
		port       int
	)

	BeforeAll(func() {
		var err error
		projectDir, err = os.MkdirTemp("", "specgraph-docker-project-*")
		Expect(err).NotTo(HaveOccurred())

		// Pick a port unlikely to collide.
		port = 19090

		// Write a docker-mode config file.
		configPath = filepath.Join(projectDir, "specgraph.yaml")
		configContent := fmt.Sprintf(`server:
  mode: docker
  host: 127.0.0.1
  port: %d
storage:
  backend: memgraph
  memgraph:
    bolt_uri: bolt://localhost:7687
`, port)
		err = os.WriteFile(configPath, []byte(configContent), 0o600)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		os.RemoveAll(projectDir)
	})

	It("writes the compose file into .specgraph/", func() {
		// Run serve in a goroutine-like fashion: start the process,
		// verify it writes the compose file, then kill it.
		cmd = exec.Command(binaryPath, "--config", configPath, "serve")
		cmd.Dir = projectDir
		cmd.Stdout = GinkgoWriter
		cmd.Stderr = GinkgoWriter

		err := cmd.Start()
		Expect(err).NotTo(HaveOccurred())

		// Wait for the compose file to appear (docker compose up may take a while).
		composePath := filepath.Join(projectDir, ".specgraph", "docker-compose.yaml")
		Eventually(func() bool {
			_, err := os.Stat(composePath)
			return err == nil
		}).WithTimeout(30*time.Second).WithPolling(500*time.Millisecond).Should(BeTrue(),
			"expected compose file at %s", composePath)

		// Read the compose file to verify it's valid.
		content, err := os.ReadFile(composePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("memgraph"))
		Expect(string(content)).To(ContainSubstring("services:"))
	})

	It("responds to health checks once running", func() {
		// Wait for the HTTP server to be ready.
		healthURL := fmt.Sprintf("http://127.0.0.1:%d/specgraph.v1.ServerService/Health", port)
		Eventually(func() error {
			resp, err := http.Post(healthURL, "application/json", nil)
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode >= 500 {
				return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
			}
			return nil
		}).WithTimeout(60 * time.Second).WithPolling(1 * time.Second).Should(Succeed())
	})

	It("shuts down cleanly on SIGTERM", func() {
		Expect(cmd).NotTo(BeNil(), "serve process should be running")
		Expect(cmd.Process).NotTo(BeNil())

		// Send SIGTERM.
		err := cmd.Process.Signal(syscall.SIGTERM)
		Expect(err).NotTo(HaveOccurred())

		// Wait for the process to exit.
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		Eventually(done).WithTimeout(30 * time.Second).Should(Receive())
	})
})
