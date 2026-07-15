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

// mcpOnlyProject is a dedicated project slug for this Describe. Using a
// dedicated project (rather than the shared e2eProject) keeps the constitution
// this test writes from contaminating other suites that assert against the
// default project's constitution (e.g. constitution_test.go's "returns
// not-found when no constitution exists"). This mirrors the per-Describe
// project convention used across the suite (conversation-e2e, pipeline-project,
// prime-xsurf-project, …).
const mcpOnlyProject = "mcp-only-project"

// mcpProjectClient stands up a real in-process MCP server (the full
// internal/mcp.Server backed by the e2e Postgres instance) whose inner
// ConnectRPC client injects the X-Specgraph-Project header for mcpOnlyProject.
//
// It mirrors skillsMCPClient (skills_test.go) but wires the project-scoped
// transport (projectClientFor(mcpOnlyProject)) instead of http.DefaultClient.
// The scoped transport is REQUIRED here — unlike the skills tools, the
// constitution/author tools reach handlers that call scopeStore, which
// rejects requests lacking the project header (internal/server/project.go).
// skillsMCPClient's bare http.DefaultClient cannot exercise those write paths,
// so this file defines its own harness rather than reusing it.
//
// Crucially, the returned client speaks ONLY MCP (ReadResource/CallTool). The
// test bodies never construct a specgraphv1connect.*ServiceClient — the "no
// CLI" simulation (D-08). The inner mcppkg.NewClient is the MCP server's own
// wiring to the ConnectRPC backend, not a surface the test calls directly.
func mcpProjectClient(ctx context.Context) (*client.Client, func()) {
	inner := mcppkg.NewClient(projectClientFor(mcpOnlyProject), serverInfo.BaseURL)
	srv := mcppkg.NewServer(inner)

	httpSrv := httptest.NewServer(http.StripPrefix("/mcp", srv.HTTPHandler()))
	mcpURL := httpSrv.URL + "/mcp/"

	c, err := client.NewStreamableHttpClient(mcpURL, transport.WithHTTPBasicClient(httpSrv.Client()))
	Expect(err).NotTo(HaveOccurred())

	Expect(c.Start(ctx)).To(Succeed())
	Expect(c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "specgraph-e2e-mcponly", Version: "0.0.0"},
		},
	})).Error().NotTo(HaveOccurred())

	cleanup := func() {
		_ = c.Close()
		httpSrv.Close()
	}
	return c, cleanup
}

// --- Canonical fixtures ------------------------------------------------------
//
// The friendly-YAML stage `output` payloads and the JSON `exchanges` arrays are
// copied from the proven minimal ConnectRPC fixtures at
// e2e/api/helpers_test.go:111-158 to keep the two verification paths (direct
// ConnectRPC and MCP-only) in lockstep and reduce fixture drift.
//
// The shape fixture includes an `approaches` entry whose name equals
// `chosen_approach` ("default") as a GOOD-PRACTICE FIXTURE CONVENTION mirroring
// helpers_test.go:114-115 — NOT because the server validates the name match.
// authoring_handler.go:141-161 only enforces field-slice sizes and approach/
// decision COUNT limits; no server rule ties chosen_approach to
// approaches[].name.
//
// `output` is friendly snake_case YAML; `exchanges` is a JSON array of
// ConversationExchange objects (role/content/stage/sequence). This is the DUAL
// WIRE-FORMAT CONTRACT documented in 06-04 and taught in 06-02.
const (
	mcpOnlyConstitutionYAML = `layer: project
name: mcp-only-e2e
constraints:
  - "no vendor lock-in"
principles:
  - id: p1
    statement: "specs are the source of truth"
    rationale: "single ground truth for the project"
`

	mcpOnlySparkYAML = `seed: "MCP-only authoring smoke"
signal: "prove the funnel reaches approved via MCP alone"
scope_sniff: small
kill_test: "if the funnel cannot reach approved through MCP"
`

	mcpOnlyShapeYAML = `scope_in:
  - "in-scope"
scope_out:
  - "out-scope"
approaches:
  - name: default
    description: "test approach"
chosen_approach: default
`

	mcpOnlyShapeExchanges = `[{"role":"probe","content":"what is in scope?","stage":"shape","sequence":1},` +
		`{"role":"response","content":"in-scope only","stage":"shape","sequence":2}]`

	mcpOnlySpecifyYAML = `interfaces:
  - name: API
    body: test
verify_criteria:
  - description: passes
`

	mcpOnlySpecifyExchanges = `[{"role":"probe","content":"what are the interfaces?","stage":"specify","sequence":1},` +
		`{"role":"response","content":"API with test body","stage":"specify","sequence":2}]`

	mcpOnlyDecomposeYAML = `strategy: single_unit
slices:
  - id: main
    intent: test
`

	mcpOnlyDecomposeExchanges = `[{"role":"probe","content":"how to decompose?","stage":"decompose","sequence":1},` +
		`{"role":"response","content":"single unit","stage":"decompose","sequence":2}]`

	// Stage-string discipline (review finding #2): the approve exchange JSON
	// carries the exchange-level stage "approve" — the value ValidateExchanges
	// checks. The conversation the accept path records, however, is STORED under
	// storage.SpecStageApproved = "approved" (internal/storage/spec_domain.go:19),
	// and ListConversations matches the stage string EXACTLY. So any list/assert
	// against the approve-gate conversation must filter on "approved", not
	// "approve" (the latter returns an empty list — a false-negative fidelity
	// assertion).
	mcpOnlyApproveExchanges = `[{"role":"probe","content":"ready to approve?","stage":"approve","sequence":1},` +
		`{"role":"response","content":"yes, approved","stage":"approve","sequence":2}]`
)

var _ = Describe("MCP-only authoring", Ordered, Label("MCPOnly"), func() {
	var (
		ctx     context.Context
		cancel  context.CancelFunc
		mcpCli  *client.Client
		cleanup func()
	)

	// Deterministic DB isolation (pass-2 finding #1): the shared e2e Postgres is
	// cleared only once in BeforeSuite (api_suite_test.go:47-48), and
	// constitution_test.go seeds a constitution that runs earlier alphabetically
	// and is never cleared. Re-run ClearAll here so the empty-state prime
	// assertion below cannot be falsified by that seeding, regardless of suite
	// file ordering. serverInfo.Store.ClearAll(ctx) is the exact reset used at
	// api_suite_test.go:48.
	BeforeAll(func() {
		Expect(serverInfo.Store.ClearAll(context.Background())).To(Succeed())
	})

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		DeferCleanup(cancel)
		mcpCli, cleanup = mcpProjectClient(ctx)
		DeferCleanup(cleanup)
	})

	It("specgraph://prime returns the empty-state constitution hint on a fresh project", func() {
		resp, err := mcpCli.ReadResource(ctx, mcp.ReadResourceRequest{
			Params: mcp.ReadResourceParams{URI: "specgraph://prime"},
		})
		Expect(err).NotTo(HaveOccurred())
		text := mcpResourceText(resp)
		Expect(text).To(ContainSubstring("# SpecGraph Session Prime"))
		// Empty-state MCP routing hint from plan 03 (render.ConstitutionEmptyHint):
		// routes an MCP-only agent to the constitution tool / specgraph-constitution
		// skill instead of a CLI it does not have (D-10).
		Expect(text).To(ContainSubstring("No constitution configured"))
		Expect(text).To(ContainSubstring("`constitution` MCP tool"))
		Expect(text).To(ContainSubstring("specgraph-constitution"))
	})

	It("authors the constitution and walks a spec spark→approved via MCP only", func() {
		// (2) Fetch the constitution self-teaching skill via MCP — the path an
		// agent takes after the empty-state prime hint points at it.
		skillRes, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "specgraph_skills_get",
				Arguments: map[string]any{"name": "specgraph-constitution"},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(toolText(skillRes)).To(ContainSubstring("name: specgraph-constitution"))

		// (3) Write the constitution via friendly YAML and confirm it persists by
		// reading it back through `constitution action:get`.
		updRes, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "constitution",
				Arguments: map[string]any{
					"action":       "update",
					"constitution": mcpOnlyConstitutionYAML,
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(updRes.IsError).To(BeFalse(), "constitution update returned an error: %v", updRes.Content)

		getRes, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "constitution",
				Arguments: map[string]any{
					"action": "get",
					"layer":  "project",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		gotConstitution := toolText(getRes)
		Expect(gotConstitution).To(ContainSubstring("mcp-only-e2e"), "constitution name must round-trip")
		Expect(gotConstitution).To(ContainSubstring("PROJECT"), "constitution layer must round-trip")

		// (4) Drive the authoring funnel spark→shape→specify→decompose→approve.
		// spark passes friendly YAML output (and creates the spec); shape/specify/
		// decompose pass friendly YAML output PLUS explicit JSON exchanges; approve
		// now also supplies exchanges — under the Plan 01 server enforcement + Plan
		// 02 MCP threading, the ACCEPT path requires a non-empty conversation, so an
		// exchange-less approve is rejected (this comment was updated per review
		// R2 #6; it previously claimed approve needs no exchanges).
		slug := fmt.Sprintf("mcp-only-spec-%d", time.Now().UnixNano())

		author := func(args map[string]any) {
			args["slug"] = slug
			res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
				Params: mcp.CallToolParams{Name: "author", Arguments: args},
			})
			Expect(err).NotTo(HaveOccurred())
			// toolText fails the spec if the funnel stage returned an error result.
			_ = toolText(res)
		}

		author(map[string]any{"action": "spark", "output": mcpOnlySparkYAML})
		author(map[string]any{"action": "shape", "output": mcpOnlyShapeYAML, "exchanges": mcpOnlyShapeExchanges})
		author(map[string]any{"action": "specify", "output": mcpOnlySpecifyYAML, "exchanges": mcpOnlySpecifyExchanges})
		author(map[string]any{"action": "decompose", "output": mcpOnlyDecomposeYAML, "exchanges": mcpOnlyDecomposeExchanges})
		author(map[string]any{"action": "approve", "exchanges": mcpOnlyApproveExchanges})

		// Prove the approved state through a real READ path (`spec action:get`),
		// not just the approve tool's echoed result (Cursor review 06-05 #2).
		specRes, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "spec",
				Arguments: map[string]any{
					"action": "get",
					"slug":   slug,
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		specText := toolText(specRes)
		Expect(specText).To(ContainSubstring(slug))
		Expect(specText).To(ContainSubstring("approved"), "spec get must reflect the approved stage")

		// (5) Conversation fidelity: every required stage recorded a non-empty,
		// retrievable conversation (criteria #1/#3/#4). Query the `conversation`
		// tool (action:list) filtered per-stage and assert the returned log is
		// non-empty and its exchange content round-trips.
		//
		// Stage-string discipline (review finding #2): the exchange JSON uses the
		// exchange-level stage ("shape"/"specify"/"decompose"/"approve", validated
		// by ValidateExchanges), but the STORED + queried conversation stage for
		// the approve gate is "approved" (storage.SpecStageApproved,
		// spec_domain.go:19). ListConversations matches the stage string EXACTLY,
		// so the approve filter MUST be "approved" — "approve" returns an empty
		// list (false negative). shape/specify/decompose exchange and stored
		// stages coincide.
		listConversation := func(filterStage string) string {
			res, callErr := mcpCli.CallTool(ctx, mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "conversation",
					Arguments: map[string]any{
						"action": "list",
						"slug":   slug,
						"stage":  filterStage,
					},
				},
			})
			Expect(callErr).NotTo(HaveOccurred())
			return toolText(res)
		}

		for _, sc := range []struct{ filterStage, wantContent string }{
			{"shape", "in-scope only"},
			{"specify", "API with test body"},
			{"decompose", "single unit"},
			{"approved", "yes, approved"}, // stored stage, NOT the exchange stage "approve"
		} {
			listText := listConversation(sc.filterStage)
			Expect(listText).To(ContainSubstring("conversationLogs"),
				"stage %q must have a recorded conversation", sc.filterStage)
			Expect(listText).To(ContainSubstring(sc.wantContent),
				"stage %q conversation content must round-trip", sc.filterStage)
		}
	})

	It("rejects a post-spark stage with valid output but no exchanges", func() {
		// The real server ValidateExchanges rejects an empty exchange list for
		// shape ("at least one exchange required") before any stage transition.
		res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "author",
				Arguments: map[string]any{
					"action": "shape",
					"slug":   fmt.Sprintf("mcp-only-neg-noex-%d", time.Now().UnixNano()),
					"output": mcpOnlyShapeYAML,
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.IsError).To(BeTrue(), "shape with no exchanges must be rejected")
	})

	It("rejects a post-spark stage whose exchange is missing sequence", func() {
		// An exchange without a `sequence` field defaults to 0, which the server
		// sequence invariant rejects ("sequence 0 must be >= 1", validate.go:74-75).
		const missingSequence = `[{"role":"probe","content":"no sequence here","stage":"shape"}]`
		res, err := mcpCli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "author",
				Arguments: map[string]any{
					"action":    "shape",
					"slug":      fmt.Sprintf("mcp-only-neg-noseq-%d", time.Now().UnixNano()),
					"output":    mcpOnlyShapeYAML,
					"exchanges": missingSequence,
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.IsError).To(BeTrue(), "shape with a sequence-less exchange must be rejected")
	})
})
