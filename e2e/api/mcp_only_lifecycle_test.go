// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mcppkg "github.com/specgraph/specgraph/internal/mcp"
)

// mcpOnlyLifecycleProject is a dedicated project slug for this Describe. A
// dedicated project (rather than the shared e2eProject or mcpOnlyProject) keeps
// the specs this suite creates isolated from the other MCP-only suite and from
// the ConnectRPC lifecycle tests, mirroring the per-Describe project convention
// used across the suite.
const mcpOnlyLifecycleProject = "mcp-only-lifecycle-project"

// mcpLifecycleAgent is the agent identifier used to drive specs to done via the
// claim + report MCP tools.
const mcpLifecycleAgent = "e2e-mcp-lifecycle-agent"

// mcpLifecycleClient stands up a real in-process MCP server (the full
// internal/mcp.Server backed by the e2e Postgres instance) whose inner
// ConnectRPC client injects the X-Specgraph-Project header for
// mcpOnlyLifecycleProject.
//
// It mirrors mcpProjectClient (mcp_only_authoring_test.go) but wires the
// lifecycle project's scoped transport. As there, the returned client speaks
// ONLY MCP (ReadResource/CallTool): the test bodies never construct a
// specgraphv1connect.*ServiceClient — the amend/supersede/re-entry flow is
// exercised purely through the author/claim/report MCP tools (D-10).
func mcpLifecycleClient(ctx context.Context) (*client.Client, func()) {
	inner := mcppkg.NewClient(projectClientFor(mcpOnlyLifecycleProject), serverInfo.BaseURL)
	srv := mcppkg.NewServer(inner)

	httpSrv := httptest.NewServer(http.StripPrefix("/mcp", srv.HTTPHandler()))
	mcpURL := httpSrv.URL + "/mcp/"

	c, err := client.NewStreamableHttpClient(mcpURL, transport.WithHTTPBasicClient(httpSrv.Client()))
	Expect(err).NotTo(HaveOccurred())

	Expect(c.Start(ctx)).To(Succeed())
	Expect(c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "specgraph-e2e-mcponly-lifecycle", Version: "0.0.0"},
		},
	})).Error().NotTo(HaveOccurred())

	cleanup := func() {
		_ = c.Close()
		httpSrv.Close()
	}
	return c, cleanup
}

var _ = Describe("MCP-only lifecycle (amend/supersede/re-entry)", Ordered, Label("MCPOnly"), func() {
	var (
		ctx     context.Context
		cancel  context.CancelFunc
		mcpCli  *client.Client
		cleanup func()
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		DeferCleanup(cancel)
		mcpCli, cleanup = mcpLifecycleClient(ctx)
		DeferCleanup(cleanup)
	})

	// callTool issues a single MCP tool call and asserts the transport call
	// itself succeeded. The returned result may still be an error result
	// (res.IsError) — callers decide whether that is expected.
	callTool := func(name string, args map[string]any) *mcp.CallToolResult {
		res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{Name: name, Arguments: args},
		})
		Expect(err).NotTo(HaveOccurred())
		return res
	}

	// author is a convenience wrapper that pins the slug and asserts a
	// non-error result (via toolText). Reuses the friendly-YAML fixtures from
	// mcp_only_authoring_test.go to keep the two MCP-only paths in lockstep.
	author := func(slug string, args map[string]any) {
		args["slug"] = slug
		_ = toolText(callTool("author", args))
	}

	// walkToApproved drives a fresh spec from spark through approve using the
	// author tool only (the MCP-only funnel path proven in 06-05).
	walkToApproved := func(slug string) {
		author(slug, map[string]any{"action": "spark", "output": mcpOnlySparkYAML})
		author(slug, map[string]any{"action": "shape", "output": mcpOnlyShapeYAML, "exchanges": mcpOnlyShapeExchanges})
		author(slug, map[string]any{"action": "specify", "output": mcpOnlySpecifyYAML, "exchanges": mcpOnlySpecifyExchanges})
		author(slug, map[string]any{"action": "decompose", "output": mcpOnlyDecomposeYAML, "exchanges": mcpOnlyDecomposeExchanges})
		author(slug, map[string]any{"action": "approve"})
	}

	// driveToDone advances an approved spec to done via the claim + report MCP
	// tools. NOTE the arg-name asymmetry that the plan calls out: the claim tool
	// takes `spec_slug`, but the report (completion) tool takes `slug`.
	driveToDone := func(slug string) {
		walkToApproved(slug)
		_ = toolText(callTool("claim", map[string]any{
			"action":    "claim",
			"spec_slug": slug,
			"agent":     mcpLifecycleAgent,
		}))
		_ = toolText(callTool("report", map[string]any{
			"action": "completion",
			"slug":   slug,
			"agent":  mcpLifecycleAgent,
		}))
	}

	It("amends an in-flight spec back and re-authors the landed stage (LIFE-02)", func() {
		const slug = "lifecycle-amend-spec"
		walkToApproved(slug)

		// Amend an in-flight (approved) spec asking to redo `shape`. This is the
		// canonical, non-degenerate re-entry example — it avoids the spark
		// same-stage no-op. The spec must land ONE stage before `shape`, i.e. at
		// `spark`, and the tool must echo the `shape` next-step hint.
		amend := callTool("author", map[string]any{
			"action":         "amend",
			"slug":           slug,
			"re_entry_stage": "shape",
			"reason":         "scope changed after review",
		})
		amendText := toolText(amend)
		Expect(amendText).To(ContainSubstring("spark"),
			"amend with re_entry_stage=shape must land the spec one stage before shape (spark)")
		Expect(amendText).To(ContainSubstring("action=shape"),
			"amend must echo the land-one-before next-step hint naming the shape stage")

		// Re-author the shape stage. This is the transition that reproduced the
		// #899 no-op before the reroute; it must now succeed (toolText asserts a
		// non-error result).
		_ = toolText(callTool("author", map[string]any{
			"action":    "shape",
			"slug":      slug,
			"output":    mcpOnlyShapeYAML,
			"exchanges": mcpOnlyShapeExchanges,
		}))
	})

	It("supersedes a done spec with a replacement (LIFE-01)", func() {
		const doneSlug = "lifecycle-done-spec"
		const replacementSlug = "lifecycle-replacement"

		// Drive the target spec to done purely via MCP (author + claim + report).
		driveToDone(doneSlug)

		// Create a separate, non-terminal replacement spec (spark is enough — it
		// only needs to exist and be non-terminal for LifecycleSupersedeSpec).
		author(replacementSlug, map[string]any{"action": "spark", "output": mcpOnlySparkYAML})

		// Supersede the done spec with the replacement. reason is optional here.
		_ = toolText(callTool("author", map[string]any{
			"action":   "supersede",
			"slug":     doneSlug,
			"new_slug": replacementSlug,
			"reason":   "rebuilt on the consolidated lifecycle model",
		}))
	})

	It("rejects amend on a done spec (LIFE-01)", func() {
		const slug = "lifecycle-amend-on-done"
		driveToDone(slug)

		// Amend is only valid while in flight; on a done spec it must be rejected.
		res := callTool("author", map[string]any{
			"action":         "amend",
			"slug":           slug,
			"re_entry_stage": "shape",
			"reason":         "should be rejected on a done spec",
		})
		Expect(res.IsError).To(BeTrue(), "amend on a done spec must be rejected")
	})

	It("rejects supersede on a non-done in-flight spec (LIFE-01)", func() {
		const slug = "lifecycle-supersede-inflight"
		const replacementSlug = "lifecycle-supersede-repl"

		// Leave the spec in flight (approved, not done).
		walkToApproved(slug)

		// A valid, non-terminal replacement exists, so the ONLY reason supersede
		// can fail is the non-done precondition on the target (the done-check runs
		// before the replacement-check in LifecycleSupersedeSpec).
		author(replacementSlug, map[string]any{"action": "spark", "output": mcpOnlySparkYAML})

		res := callTool("author", map[string]any{
			"action":   "supersede",
			"slug":     slug,
			"new_slug": replacementSlug,
		})
		Expect(res.IsError).To(BeTrue(), "supersede on a non-done spec must be rejected")
	})
})
