// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e_cli

package cli_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/e2e/testutil"
)

var (
	cli        *testutil.CLIRunner
	serverInfo *testutil.ServerInfo
	workDir    string // temp dir with .specgraph.yaml for project context
)

func TestCLI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI E2E Suite")
}

var _ = BeforeSuite(func() {
	ctx := context.Background()

	binaryPath, coverDir, cleanupBinary, err := testutil.BuildBinary()
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

	// Create a temp working directory with .specgraph.yaml so the CLI
	// sends the X-Specgraph-Project header on every request.
	workDir, err = os.MkdirTemp("", "specgraph-cli-e2e-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { os.RemoveAll(workDir) })

	specgraphYAML := fmt.Sprintf("project: cli-e2e-test\nserver: %s\n", serverInfo.BaseURL)
	Expect(os.WriteFile(filepath.Join(workDir, ".specgraph.yaml"), []byte(specgraphYAML), 0o644)).To(Succeed()) //nolint:gosec // test fixture

	cli = testutil.NewCLIRunner(binaryPath, serverInfo.ConfigPath, coverDir)
})
