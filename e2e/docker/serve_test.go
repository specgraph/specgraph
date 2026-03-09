// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e && !windows

package docker_test

import (
	"fmt"
	"net"
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
		done       chan error // receives cmd.Wait() result; detects early crashes
	)

	BeforeAll(func() {
		var err error
		projectDir, err = os.MkdirTemp("", "specgraph-docker-project-*")
		Expect(err).NotTo(HaveOccurred())

		// Find a free port dynamically to avoid collisions when tests run in parallel.
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()

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
		if cmd != nil && cmd.Process != nil {
			// Kill the entire process group so child processes (e.g. docker compose
			// down) are also terminated; otherwise cmd.Wait blocks on inherited pipes.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if done != nil {
				select {
				case <-done:
				case <-time.After(15 * time.Second):
				}
			}
		}
		os.RemoveAll(projectDir)
	})

	It("writes the compose file into .specgraph/", func() {
		// Start the serve process — it persists across ordered specs.
		cmd = exec.Command(binaryPath, "--config", configPath, "serve")
		cmd.Dir = projectDir
		cmd.Stdout = GinkgoWriter
		cmd.Stderr = GinkgoWriter
		// Put the process in its own process group so AfterAll can kill child
		// processes (e.g. docker compose down) that inherit stdout pipes.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		err := cmd.Start()
		Expect(err).NotTo(HaveOccurred())

		// Monitor the process in the background so other specs can detect crashes.
		done = make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

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
			// Check if process crashed before attempting health check.
			select {
			case err := <-done:
				return fmt.Errorf("serve process exited unexpectedly: %v", err)
			default:
			}
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

		// Check for early crash before sending signal.
		select {
		case err := <-done:
			Fail(fmt.Sprintf("serve process already exited: %v", err))
		default:
		}

		// Send SIGTERM.
		err := cmd.Process.Signal(syscall.SIGTERM)
		Expect(err).NotTo(HaveOccurred())

		// Wait for the process to exit via the shared done channel.
		var exitErr error
		Eventually(done).WithTimeout(30 * time.Second).Should(Receive(&exitErr))

		// Verify clean exit.
		Expect(cmd.ProcessState).NotTo(BeNil())
		Expect(cmd.ProcessState.ExitCode()).To(Equal(0), "serve should exit cleanly on SIGTERM")
	})
})
