// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mcppkg "github.com/specgraph/specgraph/internal/mcp"
)

// mcpOnlyConvProject is a dedicated project slug for the conversation-fidelity
// Describe. A per-Describe project (rather than reusing mcpOnlyProject) keeps
// this suite's fidelity assertions deterministic and free of cross-test
// project-state bleed (threat T-08-11), mirroring the per-Describe project
// convention used across the MCP-only suite (mcp_only_authoring_test.go:24-33).
const mcpOnlyConvProject = "mcp-only-conv-project"

// mcpConvProjectClient stands up an in-process MCP server whose inner
// ConnectRPC client injects the X-Specgraph-Project header for
// mcpOnlyConvProject. It is the direct analogue of mcpProjectClient
// (mcp_only_authoring_test.go:49) but scoped to this Describe's isolated
// project so the two MCP-only suites cannot contaminate each other's specs or
// conversation logs.
func mcpConvProjectClient(ctx context.Context) (*client.Client, func()) {
	inner := mcppkg.NewClient(projectClientFor(mcpOnlyConvProject), serverInfo.BaseURL)
	srv := mcppkg.NewServer(inner)

	httpSrv := httptest.NewServer(http.StripPrefix("/mcp", srv.HTTPHandler()))
	mcpURL := httpSrv.URL + "/mcp/"

	c, err := client.NewStreamableHttpClient(mcpURL, transport.WithHTTPBasicClient(httpSrv.Client()))
	Expect(err).NotTo(HaveOccurred())

	Expect(c.Start(ctx)).To(Succeed())
	Expect(c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "specgraph-e2e-mcponly-conv", Version: "0.0.0"},
		},
	})).Error().NotTo(HaveOccurred())

	cleanup := func() {
		_ = c.Close()
		httpSrv.Close()
	}
	return c, cleanup
}

// This spec is the D-10 phase gate (CONV-01): an MCP-only backstop proving that
// conversation recording is protocol-enforced, not agent-discretionary. It
// would have caught #906 — it exercises the only remaining recording path
// (inline-with-save via the `author` tool) and asserts both the positive
// (every required stage has a non-empty, retrievable conversation) and the
// negative (omitting exchanges on a required stage is rejected — a missing
// conversation cannot silently pass).
//
// Fixtures (mcpOnlySparkYAML, mcpOnlyShapeYAML, mcpOnlyShapeExchanges,
// mcpOnlyApproveExchanges, …) are shared with mcp_only_authoring_test.go — the
// two files live in the same api_test package.
var _ = Describe("MCP-only conversation fidelity", Label("MCPOnly"), func() {
	var (
		ctx     context.Context
		cancel  context.CancelFunc
		mcpCli  *client.Client
		cleanup func()
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		DeferCleanup(cancel)
		mcpCli, cleanup = mcpConvProjectClient(ctx)
		DeferCleanup(cleanup)
	})

	// author drives one funnel stage via the `author` tool and fails the spec
	// (through toolText) if the stage returned an error result.
	author := func(slug string, args map[string]any) {
		args["slug"] = slug
		res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{Name: "author", Arguments: args},
		})
		Expect(err).NotTo(HaveOccurred())
		_ = toolText(res)
	}

	// listConversation returns the JSON text of `conversation action:list`
	// filtered to a single stage.
	listConversation := func(slug, filterStage string) string {
		res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "conversation",
				Arguments: map[string]any{
					"action": "list",
					"slug":   slug,
					"stage":  filterStage,
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		return toolText(res)
	}

	It("records a non-empty retrievable conversation at every required stage", func() {
		slug := fmt.Sprintf("mcp-conv-pos-%d", time.Now().UnixNano())

		// Drive spark→shape→specify→decompose→approve, supplying exchanges at
		// every required stage (approve now supplies exchanges under the Plan
		// 01/02 enforcement).
		author(slug, map[string]any{"action": "spark", "output": mcpOnlySparkYAML})
		author(slug, map[string]any{"action": "shape", "output": mcpOnlyShapeYAML, "exchanges": mcpOnlyShapeExchanges})
		author(slug, map[string]any{"action": "specify", "output": mcpOnlySpecifyYAML, "exchanges": mcpOnlySpecifyExchanges})
		author(slug, map[string]any{"action": "decompose", "output": mcpOnlyDecomposeYAML, "exchanges": mcpOnlyDecomposeExchanges})
		author(slug, map[string]any{"action": "approve", "exchanges": mcpOnlyApproveExchanges})

		// Conversation fidelity (criteria #1/#3/#4): after the funnel completes,
		// each required stage's conversation is retrievable and non-empty, and its
		// exchange content round-trips.
		//
		// Stage-string discipline (review finding #2): the exchange JSON carries
		// the exchange-level stage ("shape"/"specify"/"decompose"/"approve",
		// validated by ValidateExchanges), but the accept path STORES the approve
		// conversation under storage.SpecStageApproved = "approved"
		// (internal/storage/spec_domain.go:19). ListConversations matches the
		// stage string EXACTLY, so the approve filter MUST be "approved" —
		// "approve" returns an empty list (a false-negative fidelity assertion).
		// shape/specify/decompose exchange and stored stages coincide.
		for _, sc := range []struct{ filterStage, wantContent string }{
			{"shape", "in-scope only"},
			{"specify", "API with test body"},
			{"decompose", "single unit"},
			{"approved", "yes, approved"}, // stored stage, NOT the exchange stage "approve"
		} {
			listText := listConversation(slug, sc.filterStage)
			Expect(listText).To(ContainSubstring("conversationLogs"),
				"stage %q must have a recorded conversation", sc.filterStage)
			Expect(listText).To(ContainSubstring("exchangeCount"),
				"stage %q conversation must record exchanges", sc.filterStage)
			Expect(listText).To(ContainSubstring(sc.wantContent),
				"stage %q conversation content must round-trip", sc.filterStage)
		}
	})

	It("rejects approve without exchanges — a missing conversation cannot silently pass", func() {
		slug := fmt.Sprintf("mcp-conv-neg-%d", time.Now().UnixNano())

		// Drive the spec to the decompose gate with valid conversations.
		author(slug, map[string]any{"action": "spark", "output": mcpOnlySparkYAML})
		author(slug, map[string]any{"action": "shape", "output": mcpOnlyShapeYAML, "exchanges": mcpOnlyShapeExchanges})
		author(slug, map[string]any{"action": "specify", "output": mcpOnlySpecifyYAML, "exchanges": mcpOnlySpecifyExchanges})
		author(slug, map[string]any{"action": "decompose", "output": mcpOnlyDecomposeYAML, "exchanges": mcpOnlyDecomposeExchanges})

		// Negative case (criteria #2/#3, threat T-08-12): approve with the
		// `exchanges` argument omitted must NOT silently pass. Do NOT assert a
		// specific server Connect code (review R2 #6): after 08-02's client-side
		// empty-exchanges guard, the MCP `author` tool may return an error result
		// BEFORE the server InvalidArgument round-trip. Either the client guard OR
		// the server rejection satisfies the backstop — assert on res.IsError
		// (mirroring the shape-without-exchanges negative at
		// mcp_only_authoring_test.go) with a corroborating error-text check.
		res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "author",
				Arguments: map[string]any{
					"action": "approve",
					"slug":   slug,
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.IsError).To(BeTrue(),
			"approve with no exchanges must be rejected (client guard or server InvalidArgument)")

		errText := toolErrorText(res)
		Expect(errText).To(SatisfyAny(
			ContainSubstring("exchanges"),
			ContainSubstring("InvalidArgument"),
			ContainSubstring("invalid_argument"),
		), "rejection must reference the missing exchanges / InvalidArgument")
	})
})

// toolErrorText concatenates the text content of an error tool result. Unlike
// toolText it does not assert res.IsError is false — it is used to inspect the
// message of a deliberately-failing call.
func toolErrorText(res *mcp.CallToolResult) string {
	Expect(res).NotTo(BeNil())
	var out string
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			out += tc.Text
		}
	}
	return out
}
