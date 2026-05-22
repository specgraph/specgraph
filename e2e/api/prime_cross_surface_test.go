// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/encoding/prototext"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	mcppkg "github.com/specgraph/specgraph/internal/mcp"

	"github.com/specgraph/specgraph/e2e/testutil"
)

// Prime cross-surface fact presence (spgr-8ar piece E, Task 8).
//
// Goal: a single integration check that all three "prime" surfaces —
// (1) ExecutionService.GetPrime RPC, (2) the specgraph://prime[…] MCP
// resource, and (3) the `specgraph prime` CLI — surface the same seeded
// constitution fact for both project and spec scopes. This is the safety
// net that fails if any surface diverges from the unified Composer path.
var _ = Describe("Prime cross-surface fact presence (spgr-8ar piece E)", Ordered, func() {
	const (
		projectSlug = "prime-xsurf-project"
		specSlug    = "prime-xsurf-spec"
	)

	var (
		ctx                    context.Context
		httpClient             *http.Client
		constClient            specgraphv1connect.ConstitutionServiceClient
		specClient             specgraphv1connect.SpecServiceClient
		execClient             specgraphv1connect.ExecutionServiceClient
		cli                    *testutil.CLIRunner
		tmpDir                 string
		uniqueConstitutionName string
		uniqueConstraint       string
		mcpCli                 *client.Client
		mcpCleanup             func()
	)

	BeforeAll(func() {
		ctx = context.Background()
		httpClient = projectClientFor(projectSlug)
		constClient = specgraphv1connect.NewConstitutionServiceClient(httpClient, serverInfo.BaseURL)
		specClient = specgraphv1connect.NewSpecServiceClient(httpClient, serverInfo.BaseURL)
		execClient = specgraphv1connect.NewExecutionServiceClient(httpClient, serverInfo.BaseURL)

		nonce := fmt.Sprintf("%d", time.Now().UnixNano())
		uniqueConstitutionName = "xsurf-const-" + nonce
		// Embed the nonce in the first constraint so it lands in the top-5
		// constraint slice that the markdown renderer prints. The constitution
		// Name itself is not surfaced by the markdown renderer (it stays in
		// the proto), so we rely on this constraint for the MCP + CLI checks
		// and use the name for the structural proto assertion on the RPC path.
		uniqueConstraint = "xsurf-constraint-" + nonce

		// Seed a project-scope constitution containing both the unique name
		// and a unique top-5 constraint.
		_, err := constClient.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Name:  uniqueConstitutionName,
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Principles: []*specv1.Principle{
					{Id: "xsurf-p1", Statement: "test principle", Rationale: "test"},
				},
				Constraints: []string{uniqueConstraint, "secondary constraint"},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		// Seed a spec so the spec-scope prime has a valid target.
		_, err = specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   specSlug,
			Intent: "cross-surface prime test target",
		}))
		Expect(err).NotTo(HaveOccurred())

		// Stand up an in-process MCP server pointing at the e2e ConnectRPC
		// server. The loopback HTTP client uses the project-scoped transport
		// so MCP resource handlers (which call ExecutionService.GetPrime under
		// the hood) hit the same project as the RPC and CLI surfaces.
		mcpInnerClient := mcppkg.NewClient(httpClient, serverInfo.BaseURL)
		mcpSrv := mcppkg.NewServer(mcpInnerClient)
		httpSrv := httptest.NewServer(http.StripPrefix("/mcp", mcpSrv.HTTPHandler()))
		mcpURL := httpSrv.URL + "/mcp/"

		c, err := client.NewStreamableHttpClient(mcpURL, transport.WithHTTPBasicClient(httpSrv.Client()))
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Start(ctx)).To(Succeed())
		Expect(c.Initialize(ctx, mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
				ClientInfo:      mcp.Implementation{Name: "specgraph-e2e-xsurf", Version: "0.0.0"},
			},
		})).Error().NotTo(HaveOccurred())
		mcpCli = c
		mcpCleanup = func() {
			_ = c.Close()
			httpSrv.Close()
		}

		// Build a CLI runner anchored at a tmp dir and write a minimal
		// .specgraph.yaml so resolveBaseURL picks up the same project slug
		// AND the e2e server URL. We write the file by hand rather than
		// running `specgraph init` because init has no --server flag — its
		// generated config would point the CLI at the global default
		// (127.0.0.1:9090) instead of the random-port e2e server.
		tmpDir, err = os.MkdirTemp("", "specgraph-prime-xsurf-*")
		Expect(err).NotTo(HaveOccurred())
		projectYAML := fmt.Sprintf("project: %s\nserver: %s\n", projectSlug, serverInfo.BaseURL)
		Expect(os.WriteFile(tmpDir+"/.specgraph.yaml", []byte(projectYAML), 0o600)).To(Succeed())
		cli = testutil.NewCLIRunner(cliBinaryPath, serverInfo.ConfigPath, "")
	})

	AfterAll(func() {
		if mcpCleanup != nil {
			mcpCleanup()
		}
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	})

	It("project scope: RPC, MCP, and CLI all surface the seeded constitution fact", func() {
		// (1) RPC — ExecutionService.GetPrime with empty slug returns a
		// ProjectView. The full proto serialized as text must mention both
		// the unique constitution name and the unique constraint.
		rpcResp, err := execClient.GetPrime(ctx, connect.NewRequest(&specv1.GetPrimeRequest{Slug: ""}))
		Expect(err).NotTo(HaveOccurred())
		pview := rpcResp.Msg.GetProjectView()
		Expect(pview).NotTo(BeNil(), "RPC response missing ProjectView")
		rpcText := prototext.Format(rpcResp.Msg)
		Expect(rpcText).To(ContainSubstring(uniqueConstitutionName), "RPC ProjectView must carry seeded constitution name")
		Expect(rpcText).To(ContainSubstring(uniqueConstraint), "RPC ProjectView must carry seeded constraint")

		// (2) MCP — the specgraph://prime resource is the markdown digest
		// that LLM clients consume. The unique constraint must appear in the
		// rendered top-5 constraint list.
		mcpResp, err := mcpCli.ReadResource(ctx, mcp.ReadResourceRequest{
			Params: mcp.ReadResourceParams{URI: "specgraph://prime"},
		})
		Expect(err).NotTo(HaveOccurred())
		mcpText := mcpResourceText(mcpResp)
		Expect(mcpText).To(ContainSubstring("# SpecGraph Session Prime"))
		Expect(mcpText).To(ContainSubstring(uniqueConstraint), "MCP project prime must carry seeded constraint")

		// (3) CLI — `specgraph prime` (no slug) renders the same markdown
		// digest. We assert on the unique constraint, which appears in the
		// rendered Top constraints list.
		cliResult := cli.RunInDir(tmpDir, "prime")
		Expect(cliResult.ExitCode).To(Equal(0), "cli stderr: %s", cliResult.Stderr)
		Expect(cliResult.Stdout).To(ContainSubstring("# SpecGraph Session Prime"))
		Expect(cliResult.Stdout).To(ContainSubstring(uniqueConstraint), "CLI project prime must carry seeded constraint")
	})

	It("spec scope: RPC, MCP, and CLI all surface the seeded constitution fact for the same spec", func() {
		// (1) RPC — same call but with the seeded slug; response is a SpecView.
		rpcResp, err := execClient.GetPrime(ctx, connect.NewRequest(&specv1.GetPrimeRequest{Slug: specSlug}))
		Expect(err).NotTo(HaveOccurred())
		sview := rpcResp.Msg.GetSpecView()
		Expect(sview).NotTo(BeNil(), "RPC response missing SpecView")
		Expect(sview.GetSpec().GetSlug()).To(Equal(specSlug))
		rpcText := prototext.Format(rpcResp.Msg)
		Expect(rpcText).To(ContainSubstring(uniqueConstitutionName), "RPC SpecView must carry seeded constitution name")
		Expect(rpcText).To(ContainSubstring(uniqueConstraint), "RPC SpecView must carry seeded constraint")

		// (2) MCP — specgraph://prime/spec/<slug>.
		mcpResp, err := mcpCli.ReadResource(ctx, mcp.ReadResourceRequest{
			Params: mcp.ReadResourceParams{URI: "specgraph://prime/spec/" + specSlug},
		})
		Expect(err).NotTo(HaveOccurred())
		mcpText := mcpResourceText(mcpResp)
		Expect(mcpText).To(ContainSubstring("# Prime: " + specSlug))
		Expect(mcpText).To(ContainSubstring(uniqueConstraint), "MCP spec prime must carry seeded constraint")

		// (3) CLI — `specgraph prime <slug>`.
		cliResult := cli.RunInDir(tmpDir, "prime", specSlug)
		Expect(cliResult.ExitCode).To(Equal(0), "cli stderr: %s", cliResult.Stderr)
		Expect(cliResult.Stdout).To(ContainSubstring("# Prime: " + specSlug))
		Expect(cliResult.Stdout).To(ContainSubstring(uniqueConstraint), "CLI spec prime must carry seeded constraint")
	})
})

// mcpResourceText concatenates all TextResourceContents bodies from a
// ReadResource response. Mirrors the helper pattern used by skills_test.go.
func mcpResourceText(resp *mcp.ReadResourceResult) string {
	out := ""
	for _, content := range resp.Contents {
		if tc, ok := content.(mcp.TextResourceContents); ok {
			out += tc.Text
		}
	}
	return out
}
