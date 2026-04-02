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
		dataDir    string
		cmd        *exec.Cmd
		port       int
		done       chan error // receives cmd.Wait() result; detects early crashes
	)

	BeforeAll(func() {
		var err error
		projectDir, err = os.MkdirTemp("", "specgraph-docker-project-*")
		Expect(err).NotTo(HaveOccurred())

		dataDir, err = os.MkdirTemp("", "specgraph-docker-data-*")
		Expect(err).NotTo(HaveOccurred())

		// Find a free port dynamically to avoid collisions when tests run in parallel.
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()

		// Write a GlobalConfig-format config file.
		// serve uses config.LoadGlobal which reads from XDG_CONFIG_HOME/specgraph/config.yaml.
		configDir := filepath.Join(projectDir, "config", "specgraph")
		Expect(os.MkdirAll(configDir, 0o750)).To(Succeed())
		configPath := filepath.Join(configDir, "config.yaml")
		// Use a random high port for Postgres to avoid conflicts with
		// any Postgres instance already running on the CI runner (e.g.
		// from the integration test step that uses testcontainers).
		pgPort := port + 1000
		configContent := fmt.Sprintf(`server:
  listen: "127.0.0.1:%d"
  mode: docker
  docker: true
  backend: postgres
  postgres:
    url: postgres://specgraph:specgraph@localhost:%d/specgraph?sslmode=disable
auth:
  mode: local
  api_keys:
    - id: docker-e2e
      key: spgr_sk_dkr00000000000000000000000000000
      name: Docker E2E
      role: admin
`, port, pgPort)
		// Set POSTGRES_PORT so the compose template uses our random port.
		os.Setenv("POSTGRES_PORT", fmt.Sprintf("%d", pgPort)) //nolint:tenv // test intentionally sets global env for child process
		Expect(os.WriteFile(configPath, []byte(configContent), 0o600)).To(Succeed())
	})

	AfterAll(func() {
		// Dump container logs for debugging CI failures.
		logCmd := exec.Command("docker", "logs", "specgraph-postgres-1") //nolint:gosec // static container name
		logCmd.Stdout = GinkgoWriter
		logCmd.Stderr = GinkgoWriter
		_ = logCmd.Run()

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
		os.RemoveAll(dataDir)
	})

	It("writes the compose file into the data directory", func() {
		// Start the serve process — it persists across ordered specs.
		// Set XDG env vars so serve reads our config and writes compose files
		// to our temp dirs instead of the real user paths.
		cmd = exec.Command(binaryPath, "serve")
		cmd.Dir = projectDir
		cmd.Env = append(os.Environ(),
			"XDG_CONFIG_HOME="+filepath.Join(projectDir, "config"),
			"XDG_DATA_HOME="+dataDir,
		)
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
		// EnsureComposeFile writes to xdg.DataHome()/docker-compose.yaml.
		composePath := filepath.Join(dataDir, "specgraph", "docker-compose.yaml")
		Eventually(func() bool {
			_, err := os.Stat(composePath)
			return err == nil
		}).WithTimeout(30*time.Second).WithPolling(500*time.Millisecond).Should(BeTrue(),
			"expected compose file at %s", composePath)

		// Read the compose file to verify it's valid.
		content, err := os.ReadFile(composePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("postgres"))
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
