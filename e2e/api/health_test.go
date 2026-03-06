// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Health", func() {
	It("returns healthy when server is running", func() {
		client := specgraphv1connect.NewServerServiceClient(http.DefaultClient, serverInfo.BaseURL)
		resp, err := client.Health(context.Background(), connect.NewRequest(&specv1.HealthRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Status).To(Equal("ok"))
	})
})
