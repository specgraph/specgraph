// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/e2e/testutil"
)

var (
	serverInfo    *testutil.ServerInfo
	cleanupServer func()
	cleanupDB     func()
	cliBinaryPath string
	pgConnURL     string
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API E2E Suite")
}

var _ = BeforeSuite(func() {
	ctx := context.Background()

	var err error

	var cleanupBinary func()
	cliBinaryPath, _, cleanupBinary, err = testutil.BuildBinary()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(cleanupBinary)

	pgConnURL, cleanupDB, err = testutil.StartPostgres(ctx)
	Expect(err).NotTo(HaveOccurred())

	serverInfo, cleanupServer, err = testutil.StartServer(ctx, pgConnURL)
	Expect(err).NotTo(HaveOccurred())

	// Clear any leftover data from previous test runs.
	Expect(serverInfo.Store.ClearAll(ctx)).To(Succeed())
})

var _ = AfterSuite(func() {
	if cleanupServer != nil {
		cleanupServer()
	}
	if cleanupDB != nil {
		cleanupDB()
	}
})
