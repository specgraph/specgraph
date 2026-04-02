// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e_agent

package agent_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/e2e/testutil"
)

var (
	serverInfo *testutil.ServerInfo
	binaryPath string
	workDir    string // temp dir with .specgraph.yaml
	claudePath string
)

func TestAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent E2E Suite")
}

var _ = BeforeSuite(func() {
	// Check prerequisites — skip entire suite if not met.
	var err error
	claudePath, err = exec.LookPath("claude")
	if err != nil {
		Skip("claude CLI not found on PATH — skipping agent E2E tests")
	}

	// Verify claude can produce output by running a trivial prompt.
	// This catches cases where claude is on PATH but can't authenticate,
	// or is running inside a nested Claude Code session (sandbox restriction).
	authCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	authCheck := exec.CommandContext(authCtx, claudePath, "-p", "respond with exactly: ok", "--max-turns", "1")
	authOut, authErr := authCheck.CombinedOutput()
	if errors.Is(authCtx.Err(), context.DeadlineExceeded) {
		Skip("claude CLI auth check timed out")
	}
	authStr := strings.TrimSpace(string(authOut))
	if authErr != nil {
		Skip(fmt.Sprintf("claude CLI auth check failed: %v\nOutput: %s", authErr, authStr))
	}
	if len(authStr) < 2 {
		Skip(fmt.Sprintf("claude CLI produced no meaningful output (got %q) — may be running inside a nested Claude Code session", authStr))
	}

	ctx := context.Background()

	var cleanupBinary func()
	binaryPath, _, cleanupBinary, err = testutil.BuildBinary()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(cleanupBinary)

	connURL, cleanupDB, err := testutil.StartPostgres(ctx)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(cleanupDB)

	var cleanupServer func()
	serverInfo, cleanupServer, err = testutil.StartServer(ctx, connURL)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(cleanupServer)

	Expect(serverInfo.Store.ClearAll(ctx)).To(Succeed())

	// Create a temp working directory with .specgraph.yaml.
	workDir, err = os.MkdirTemp("", "specgraph-agent-e2e-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { os.RemoveAll(workDir) })

	specgraphYAML := fmt.Sprintf("project: agent-e2e-test\nserver: %s\n", serverInfo.BaseURL)
	Expect(os.WriteFile(filepath.Join(workDir, ".specgraph.yaml"), []byte(specgraphYAML), 0o644)).To(Succeed()) //nolint:gosec // test fixture
})
