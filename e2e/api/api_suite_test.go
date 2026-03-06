// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/seanb4t/specgraph/e2e/testutil"
)

var (
	serverInfo    *testutil.ServerInfo
	cleanupServer func()
	cleanupMG     func()
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API E2E Suite")
}

var _ = BeforeSuite(func() {
	ctx := context.Background()

	var err error
	var boltURI string
	boltURI, cleanupMG, err = testutil.StartMemgraph(ctx)
	Expect(err).NotTo(HaveOccurred())

	serverInfo, cleanupServer, err = testutil.StartServer(ctx, boltURI)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if cleanupServer != nil {
		cleanupServer()
	}
	if cleanupMG != nil {
		cleanupMG()
	}
})
