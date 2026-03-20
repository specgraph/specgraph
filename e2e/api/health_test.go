// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

var _ = Describe("Health", func() {
	It("returns healthy when server is running", func() {
		client := newServerClient()
		resp, err := client.Health(context.Background(), connect.NewRequest(&specv1.HealthRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Status).To(Equal("ok"))
	})
})
