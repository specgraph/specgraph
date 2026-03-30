// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package docker_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/e2e/testutil"
)

var binaryPath string

func TestDocker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docker E2E Suite")
}

var _ = BeforeSuite(func() {
	var cleanup func()
	var err error
	binaryPath, _, cleanup, err = testutil.BuildBinary()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(cleanup)
})
