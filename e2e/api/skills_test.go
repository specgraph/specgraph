// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mcppkg "github.com/specgraph/specgraph/internal/mcp"
)

// skillsMCPClient spins up a real in-process MCP server (the full
// internal/mcp.Server backed by the e2e Postgres instance) and returns a
// connected mcp-go client together with a teardown function.
//
// Pattern mirrors read_mcp_resource_test.go's newStubMCPHandler: stand up
// the server in an httptest.Server, wrap it, then hand back a client.  The
// difference here is that we use the real mcp.NewServer (with embedded
// skills) instead of a stub, giving us true server-path coverage for every
// It block below.
//
// No auth middleware is applied to the httptest endpoint — the e2e suite
// owns the running server and has no API key configured in testutil.
func skillsMCPClient(ctx context.Context) (*client.Client, func()) {
	mcpClient := mcppkg.NewClient(http.DefaultClient, serverInfo.BaseURL)
	srv := mcppkg.NewServer(mcpClient)

	httpSrv := httptest.NewServer(http.StripPrefix("/mcp", srv.HTTPHandler()))
	mcpURL := httpSrv.URL + "/mcp/"

	c, err := client.NewStreamableHttpClient(mcpURL, transport.WithHTTPBasicClient(httpSrv.Client()))
	Expect(err).NotTo(HaveOccurred())

	Expect(c.Start(ctx)).To(Succeed())
	Expect(c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "specgraph-e2e", Version: "0.0.0"},
		},
	})).Error().NotTo(HaveOccurred())

	cleanup := func() {
		_ = c.Close()
		httpSrv.Close()
	}
	return c, cleanup
}

// toolText extracts the concatenated text from a successful CallToolResult.
// Fails the spec if the result is nil, is marked as an error, or has no
// text content.
func toolText(res *mcp.CallToolResult) string {
	Expect(res).NotTo(BeNil())
	Expect(res.IsError).To(BeFalse(), "tool returned an error result: %v", res.Content)
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "")
}

var _ = Describe("Skills via MCP", func() {
	var (
		ctx     context.Context
		cancel  context.CancelFunc
		mcpCli  *client.Client
		cleanup func()
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		DeferCleanup(cancel)
		mcpCli, cleanup = skillsMCPClient(ctx)
		DeferCleanup(cleanup)
	})

	It("exposes specgraph://skills/specgraph-authoring with the SKILL.md body", func() {
		resp, err := mcpCli.ReadResource(ctx, mcp.ReadResourceRequest{
			Params: mcp.ReadResourceParams{URI: "specgraph://skills/specgraph-authoring"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Contents).NotTo(BeEmpty())
		text := ""
		for _, content := range resp.Contents {
			if tc, ok := content.(mcp.TextResourceContents); ok {
				text += tc.Text
			}
		}
		Expect(text).To(ContainSubstring("name: specgraph-authoring"))
		Expect(text).To(ContainSubstring("summary:"))
	})

	It("lists seven skills via specgraph_skills_list", func() {
		res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{Name: "specgraph_skills_list"},
		})
		Expect(err).NotTo(HaveOccurred())
		out := toolText(res)
		for _, name := range []string{
			"specgraph-authoring",
			"specgraph-constitution",
			"specgraph-graph-query",
			"specgraph-analytical-passes",
			"specgraph-drift",
			"specgraph-conventions",
			"specgraph-troubleshooting",
		} {
			Expect(out).To(ContainSubstring(name))
		}
	})

	It("fetches specgraph-authoring body via specgraph_skills_get", func() {
		res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "specgraph_skills_get",
				Arguments: map[string]any{"name": "specgraph-authoring"},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		out := toolText(res)
		Expect(out).To(ContainSubstring("name: specgraph-authoring"))
		Expect(out).To(ContainSubstring("summary:"))
	})

	It("finds the drift skill via specgraph_skills_search", func() {
		res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "specgraph_skills_search",
				Arguments: map[string]any{"query": "drift"},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		out := toolText(res)
		Expect(out).To(ContainSubstring("specgraph-drift"))
	})

	It("rejects malformed skill URIs with a not-found error", func() {
		_, err := mcpCli.ReadResource(ctx, mcp.ReadResourceRequest{
			Params: mcp.ReadResourceParams{URI: "specgraph://skills/foo/bar"},
		})
		Expect(err).To(HaveOccurred())
		Expect(strings.ToLower(err.Error())).To(SatisfyAny(
			ContainSubstring("not found"),
			ContainSubstring("notfound"),
			ContainSubstring("not_found"),
			ContainSubstring("malformed"),
		))
	})
})
