# MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full-featured MCP server exposing SpecGraph's tools, resources, and prompts to AI agents via stdio and HTTP transports.

**Architecture:** Thin MCP adapter in `internal/mcp/` translates MCP calls to ConnectRPC RPCs. SDK isolation confines `mcp-go` imports to 3 files. Tool tiers (core/authoring/execution) filter visibility by client role.

**Tech Stack:** Go, `github.com/mark3labs/mcp-go` (MIT), ConnectRPC, protobuf

**Design spec:** `docs/plans/2026-04-10-mcp-server-design.md`

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `internal/mcp/types.go` | Tier, ToolDef, ToolResult, Content, ResourceDef, PromptDef |
| Create | `internal/mcp/helpers.go` | Param extraction, result formatting, schema builders |
| Create | `internal/mcp/helpers_test.go` | Tests for helpers |
| Create | `internal/mcp/registry.go` | Tool/resource/prompt registry with tier filtering |
| Create | `internal/mcp/registry_test.go` | Registry tests |
| Create | `internal/mcp/client.go` | Client struct wrapping all ConnectRPC service clients |
| Create | `internal/mcp/testhelpers_test.go` | Mock clients for tool handler tests |
| Create | `internal/mcp/tools_spec.go` | spec + decision tool handlers |
| Create | `internal/mcp/tools_spec_test.go` | Tests |
| Create | `internal/mcp/tools_graph.go` | edge + graph_query tool handlers |
| Create | `internal/mcp/tools_graph_test.go` | Tests |
| Create | `internal/mcp/tools_core.go` | constitution + changes + findings + health handlers |
| Create | `internal/mcp/tools_core_test.go` | Tests |
| Create | `internal/mcp/tools_authoring.go` | author + conversation + analytical_pass handlers |
| Create | `internal/mcp/tools_authoring_test.go` | Tests |
| Create | `internal/mcp/tools_lifecycle.go` | drift + lint + sync + export handlers |
| Create | `internal/mcp/tools_lifecycle_test.go` | Tests |
| Create | `internal/mcp/tools_execution.go` | claim + slice + bundle + prime + report + execution_events |
| Create | `internal/mcp/tools_execution_test.go` | Tests |
| Create | `internal/mcp/resources.go` | MCP resource definitions + handlers |
| Create | `internal/mcp/resources_test.go` | Tests |
| Create | `internal/mcp/prompts.go` | MCP prompt definitions + handlers |
| Create | `internal/mcp/prompts_test.go` | Tests |
| Create | `internal/mcp/convert.go` | Our types ↔ mcp-go SDK types (**imports SDK**) |
| Create | `internal/mcp/convert_test.go` | Tests |
| Create | `internal/mcp/tiers.go` | Tier negotiation from clientInfo (**imports SDK**) |
| Create | `internal/mcp/tiers_test.go` | Tests |
| Create | `internal/mcp/server.go` | MCP server setup + transport wiring (**imports SDK**) |
| Create | `internal/mcp/server_test.go` | Tests |
| Create | `cmd/specgraph/mcp.go` | `specgraph mcp` CLI command (stdio transport) |
| Modify | `cmd/specgraph/serve.go` | Add MCP HTTP endpoint alongside ConnectRPC |

---

### Task 1: Foundation Types and Helpers

**Files:**
- Create: `internal/mcp/types.go`
- Create: `internal/mcp/helpers.go`
- Create: `internal/mcp/helpers_test.go`

- [ ] **Step 1: Create package with types**

```go
// internal/mcp/types.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

// Package mcp provides a Model Context Protocol server that translates
// MCP tool calls into ConnectRPC RPCs against a running SpecGraph server.
package mcp

import "context"

// Tier controls which tools are visible to an MCP client.
type Tier int

const (
	// TierCore exposes read-heavy tools: specs, graph queries, constitution.
	TierCore Tier = iota
	// TierAuthoring adds authoring funnel, decisions, drift, analytical passes.
	TierAuthoring
	// TierExecution adds claims, slices, progress reporting, bundles.
	TierExecution
)

// String returns the tier name used in capability negotiation.
func (t Tier) String() string {
	switch t {
	case TierCore:
		return "core"
	case TierAuthoring:
		return "authoring"
	case TierExecution:
		return "execution"
	default:
		return "core"
	}
}

// ParseTier converts a string to a Tier. Returns TierCore for unknown values.
func ParseTier(s string) Tier {
	switch s {
	case "authoring":
		return TierAuthoring
	case "execution":
		return TierExecution
	default:
		return TierCore
	}
}

// Includes reports whether tier t includes all tools visible at tier other.
func (t Tier) Includes(other Tier) bool {
	return t >= other
}

// ToolDef defines an MCP tool in SpecGraph's own types (no SDK dependency).
type ToolDef struct {
	Name        string
	Description string
	Tier        Tier
	Schema      map[string]any // JSON Schema for parameters
	Handler     ToolHandler
}

// ToolHandler processes an MCP tool call. params is the deserialized arguments map.
type ToolHandler func(ctx context.Context, params map[string]any) (*ToolResult, error)

// ToolResult is the response from a tool handler.
type ToolResult struct {
	Content []Content
	IsError bool
}

// Content is a single content block in a tool result.
type Content struct {
	Type string // "text"
	Text string
}

// ResourceDef defines an MCP resource.
type ResourceDef struct {
	URI         string // Exact URI or template pattern
	Name        string
	Description string
	MimeType    string
	IsTemplate  bool
	Handler     ResourceHandler
}

// ResourceHandler reads a resource. uri is the resolved URI.
type ResourceHandler func(ctx context.Context, uri string) ([]ResourceContent, error)

// ResourceContent is a single content block in a resource response.
type ResourceContent struct {
	URI      string
	MimeType string
	Text     string
}

// PromptDef defines an MCP prompt.
type PromptDef struct {
	Name        string
	Description string
	Arguments   []PromptArgument
	Handler     PromptHandler
}

// PromptArgument describes a prompt parameter.
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// PromptHandler renders a prompt. args is the argument map.
type PromptHandler func(ctx context.Context, args map[string]string) (*PromptResult, error)

// PromptResult is the response from a prompt handler.
type PromptResult struct {
	Description string
	Messages    []PromptMessage
}

// PromptMessage is a single message in a prompt result.
type PromptMessage struct {
	Role    string // "user" or "assistant"
	Content string
}
```

- [ ] **Step 2: Write helpers with tests**

```go
// internal/mcp/helpers_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringParam(t *testing.T) {
	params := map[string]any{"action": "get", "slug": "auth"}
	require.Equal(t, "get", stringParam(params, "action"))
	require.Equal(t, "auth", stringParam(params, "slug"))
	require.Equal(t, "", stringParam(params, "missing"))
}

func TestIntParam(t *testing.T) {
	params := map[string]any{"limit": float64(10)} // JSON numbers are float64
	require.Equal(t, int32(10), int32Param(params, "limit"))
	require.Equal(t, int32(0), int32Param(params, "missing"))
}

func TestBoolParam(t *testing.T) {
	params := map[string]any{"recursive": true}
	require.True(t, boolParam(params, "recursive"))
	require.False(t, boolParam(params, "missing"))
}

func TestTextResult(t *testing.T) {
	r := textResult("hello")
	require.Len(t, r.Content, 1)
	require.Equal(t, "text", r.Content[0].Type)
	require.Equal(t, "hello", r.Content[0].Text)
	require.False(t, r.IsError)
}

func TestErrorResultFromString(t *testing.T) {
	r := errResult("something broke")
	require.True(t, r.IsError)
	require.Contains(t, r.Content[0].Text, "something broke")
}

func TestObjectSchema(t *testing.T) {
	s := objectSchema(
		props{
			"action": stringProp("op", "get", "list"),
			"slug":   stringProp("identifier"),
		},
		"action",
	)
	require.Equal(t, "object", s["type"])
	p := s["properties"].(props)
	require.Contains(t, p, "action")
	req := s["required"].([]string)
	require.Equal(t, []string{"action"}, req)
}
```

```go
// internal/mcp/helpers.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// props is shorthand for JSON Schema property maps.
type props = map[string]any

// stringParam extracts a string parameter, returning "" if absent or wrong type.
func stringParam(params map[string]any, key string) string {
	v, _ := params[key].(string)
	return v
}

// int32Param extracts an int32 parameter. JSON numbers arrive as float64.
func int32Param(params map[string]any, key string) int32 {
	switch v := params[key].(type) {
	case float64:
		return int32(v)
	case int:
		return int32(v)
	default:
		return 0
	}
}

// boolParam extracts a bool parameter, returning false if absent.
func boolParam(params map[string]any, key string) bool {
	v, _ := params[key].(bool)
	return v
}

// stringSliceParam extracts a []string parameter from a JSON array.
func stringSliceParam(params map[string]any, key string) []string {
	raw, ok := params[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// jsonResult marshals a proto message to JSON and returns it as a text result.
func jsonResult(msg proto.Message) *ToolResult {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(msg)
	if err != nil {
		return errResult(fmt.Sprintf("marshal response: %v", err))
	}
	return textResult(string(data))
}

// textResult wraps a string in a ToolResult.
func textResult(text string) *ToolResult {
	return &ToolResult{
		Content: []Content{{Type: "text", Text: text}},
	}
}

// errResult creates an error ToolResult from a message string.
func errResult(msg string) *ToolResult {
	return &ToolResult{
		Content: []Content{{Type: "text", Text: msg}},
		IsError: true,
	}
}

// connectErrResult maps a ConnectRPC error to an MCP tool error result.
// Auth errors return a Go error (protocol-level); everything else is a tool result.
func connectErrResult(err error) (*ToolResult, error) {
	if err == nil {
		return nil, nil
	}
	code := connect.CodeOf(err)
	if code == connect.CodeUnauthenticated {
		return nil, fmt.Errorf("authentication required: %w", err)
	}
	msg := connect.CodeOf(err).String()
	if connectErr := new(connect.Error); connect.IsNotModifiedError(err) {
		msg = "not modified"
	} else if ok := (err).(*connect.Error); ok {
		msg = ok.Message()
	}
	if code == connect.CodeAborted {
		msg = "concurrent modification — retry the operation"
	}
	return errResult(msg), nil
}

// --- Schema builders ---

// objectSchema builds a JSON Schema object with the given properties and required fields.
func objectSchema(properties props, required ...string) map[string]any {
	s := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// stringProp builds a JSON Schema string property. Optional enum values constrain it.
func stringProp(desc string, enum ...string) map[string]any {
	p := map[string]any{"type": "string", "description": desc}
	if len(enum) > 0 {
		p["enum"] = enum
	}
	return p
}

// intProp builds a JSON Schema integer property.
func intProp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

// boolProp builds a JSON Schema boolean property.
func boolProp(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

// arrayProp builds a JSON Schema array property with the given item schema.
func arrayProp(desc string, items map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": items}
}
```

- [ ] **Step 3: Verify tests pass**

Run: `cd /Volumes/Code/github.com/.worktrees/specgraph/mcp-server && go test ./internal/mcp/ -run 'TestStringParam|TestIntParam|TestBoolParam|TestTextResult|TestErrorResult|TestObjectSchema' -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/types.go internal/mcp/helpers.go internal/mcp/helpers_test.go
git commit -s -m "feat(mcp): add foundation types and helpers for MCP server"
```

---

### Task 2: Tool Registry

**Files:**
- Create: `internal/mcp/registry.go`
- Create: `internal/mcp/registry_test.go`

- [ ] **Step 1: Write registry test**

```go
// internal/mcp/registry_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistry_ToolsFilteredByTier(t *testing.T) {
	r := NewRegistry()
	noop := func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		return textResult("ok"), nil
	}

	r.AddTool(ToolDef{Name: "spec", Tier: TierCore, Handler: noop})
	r.AddTool(ToolDef{Name: "author", Tier: TierAuthoring, Handler: noop})
	r.AddTool(ToolDef{Name: "claim", Tier: TierExecution, Handler: noop})

	core := r.ToolsForTier(TierCore)
	require.Len(t, core, 1)
	require.Equal(t, "spec", core[0].Name)

	authoring := r.ToolsForTier(TierAuthoring)
	require.Len(t, authoring, 2) // core + authoring
	names := []string{authoring[0].Name, authoring[1].Name}
	require.ElementsMatch(t, []string{"spec", "author"}, names)

	execution := r.ToolsForTier(TierExecution)
	require.Len(t, execution, 3)
}

func TestRegistry_LookupTool(t *testing.T) {
	r := NewRegistry()
	noop := func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		return textResult("ok"), nil
	}
	r.AddTool(ToolDef{Name: "spec", Tier: TierCore, Handler: noop})

	def, ok := r.LookupTool("spec")
	require.True(t, ok)
	require.Equal(t, "spec", def.Name)

	_, ok = r.LookupTool("missing")
	require.False(t, ok)
}

func TestRegistry_Resources(t *testing.T) {
	r := NewRegistry()
	r.AddResource(ResourceDef{URI: "specgraph://specs", Name: "specs"})
	require.Len(t, r.Resources(), 1)
}

func TestRegistry_Prompts(t *testing.T) {
	r := NewRegistry()
	r.AddPrompt(PromptDef{Name: "spark"})
	require.Len(t, r.Prompts(), 1)
}
```

- [ ] **Step 2: Run test, verify fail**

Run: `go test ./internal/mcp/ -run TestRegistry -v`
Expected: FAIL — Registry not defined

- [ ] **Step 3: Write registry**

```go
// internal/mcp/registry.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

// Registry holds all MCP tool, resource, and prompt definitions.
type Registry struct {
	tools     []ToolDef
	toolIndex map[string]int // name → index in tools
	resources []ResourceDef
	prompts   []PromptDef
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		toolIndex: make(map[string]int),
	}
}

// AddTool registers a tool definition.
func (r *Registry) AddTool(def ToolDef) {
	r.toolIndex[def.Name] = len(r.tools)
	r.tools = append(r.tools, def)
}

// ToolsForTier returns all tools visible at the given tier.
// Higher tiers include all tools from lower tiers.
func (r *Registry) ToolsForTier(tier Tier) []ToolDef {
	var out []ToolDef
	for _, def := range r.tools {
		if tier.Includes(def.Tier) {
			out = append(out, def)
		}
	}
	return out
}

// LookupTool finds a tool by name.
func (r *Registry) LookupTool(name string) (ToolDef, bool) {
	idx, ok := r.toolIndex[name]
	if !ok {
		return ToolDef{}, false
	}
	return r.tools[idx], true
}

// AddResource registers a resource definition.
func (r *Registry) AddResource(def ResourceDef) {
	r.resources = append(r.resources, def)
}

// Resources returns all registered resource definitions.
func (r *Registry) Resources() []ResourceDef {
	return r.resources
}

// AddPrompt registers a prompt definition.
func (r *Registry) AddPrompt(def PromptDef) {
	r.prompts = append(r.prompts, def)
}

// Prompts returns all registered prompt definitions.
func (r *Registry) Prompts() []PromptDef {
	return r.prompts
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./internal/mcp/ -run TestRegistry -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/registry.go internal/mcp/registry_test.go
git commit -s -m "feat(mcp): add tool/resource/prompt registry with tier filtering"
```

---

### Task 3: ConnectRPC Client Wrapper

**Files:**
- Create: `internal/mcp/client.go`

- [ ] **Step 1: Write client**

```go
// internal/mcp/client.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// Client wraps all ConnectRPC service clients needed by MCP tool handlers.
type Client struct {
	Spec           specgraphv1connect.SpecServiceClient
	Decision       specgraphv1connect.DecisionServiceClient
	Graph          specgraphv1connect.GraphServiceClient
	Claim          specgraphv1connect.ClaimServiceClient
	Constitution   specgraphv1connect.ConstitutionServiceClient
	Authoring      specgraphv1connect.AuthoringServiceClient
	AnalyticalPass specgraphv1connect.AnalyticalPassServiceClient
	Execution      specgraphv1connect.ExecutionServiceClient
	Slice          specgraphv1connect.SliceServiceClient
	Export         specgraphv1connect.ExportServiceClient
	Lifecycle      specgraphv1connect.LifecycleServiceClient
	Sync           specgraphv1connect.SyncServiceClient
	Health         specgraphv1connect.ServerServiceClient
}

// NewClient creates a Client with all service clients pointing at baseURL.
func NewClient(httpClient connect.HTTPClient, baseURL string) *Client {
	return &Client{
		Spec:           specgraphv1connect.NewSpecServiceClient(httpClient, baseURL),
		Decision:       specgraphv1connect.NewDecisionServiceClient(httpClient, baseURL),
		Graph:          specgraphv1connect.NewGraphServiceClient(httpClient, baseURL),
		Claim:          specgraphv1connect.NewClaimServiceClient(httpClient, baseURL),
		Constitution:   specgraphv1connect.NewConstitutionServiceClient(httpClient, baseURL),
		Authoring:      specgraphv1connect.NewAuthoringServiceClient(httpClient, baseURL),
		AnalyticalPass: specgraphv1connect.NewAnalyticalPassServiceClient(httpClient, baseURL),
		Execution:      specgraphv1connect.NewExecutionServiceClient(httpClient, baseURL),
		Slice:          specgraphv1connect.NewSliceServiceClient(httpClient, baseURL),
		Export:         specgraphv1connect.NewExportServiceClient(httpClient, baseURL),
		Lifecycle:      specgraphv1connect.NewLifecycleServiceClient(httpClient, baseURL),
		Sync:           specgraphv1connect.NewSyncServiceClient(httpClient, baseURL),
		Health:         specgraphv1connect.NewServerServiceClient(httpClient, baseURL),
	}
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/mcp/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/client.go
git commit -s -m "feat(mcp): add ConnectRPC client wrapper for MCP handlers"
```

---

### Task 4: Test Infrastructure and Core Tier Tools — spec, decision

**Files:**
- Create: `internal/mcp/testhelpers_test.go`
- Create: `internal/mcp/tools_spec.go`
- Create: `internal/mcp/tools_spec_test.go`

- [ ] **Step 1: Write test helpers (mock clients)**

The generated ConnectRPC interfaces (e.g., `SpecServiceClient`) can be satisfied by embedding the interface in a struct — unimplemented methods panic, and tests override only what they need.

```go
// internal/mcp/testhelpers_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// --- Spec service mock ---

type mockSpecService struct {
	specgraphv1connect.SpecServiceClient // panics on unimplemented calls
	getSpec     func(slug string) (*specv1.GetSpecResponse, error)
	listSpecs   func() (*specv1.ListSpecsResponse, error)
	createSpec  func(slug, intent string) (*specv1.CreateSpecResponse, error)
	updateSpec  func(req *specv1.UpdateSpecRequest) (*specv1.UpdateSpecResponse, error)
	listChanges func(slug string) (*specv1.ListChangesResponse, error)
	compareVer  func(req *specv1.CompareVersionsRequest) (*specv1.CompareVersionsResponse, error)
}

func (m *mockSpecService) GetSpec(_ context.Context, req *connect.Request[specv1.GetSpecRequest]) (*connect.Response[specv1.GetSpecResponse], error) {
	resp, err := m.getSpec(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSpecService) ListSpecs(_ context.Context, _ *connect.Request[specv1.ListSpecsRequest]) (*connect.Response[specv1.ListSpecsResponse], error) {
	resp, err := m.listSpecs()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSpecService) CreateSpec(_ context.Context, req *connect.Request[specv1.CreateSpecRequest]) (*connect.Response[specv1.CreateSpecResponse], error) {
	resp, err := m.createSpec(req.Msg.GetSlug(), req.Msg.GetIntent())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSpecService) ListChanges(_ context.Context, req *connect.Request[specv1.ListChangesRequest]) (*connect.Response[specv1.ListChangesResponse], error) {
	resp, err := m.listChanges(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Decision service mock ---

type mockDecisionService struct {
	specgraphv1connect.DecisionServiceClient
	getDecision  func(slug string) (*specv1.GetDecisionResponse, error)
	listDecisions func() (*specv1.ListDecisionsResponse, error)
	createDecision func(req *specv1.CreateDecisionRequest) (*specv1.CreateDecisionResponse, error)
}

func (m *mockDecisionService) GetDecision(_ context.Context, req *connect.Request[specv1.GetDecisionRequest]) (*connect.Response[specv1.GetDecisionResponse], error) {
	resp, err := m.getDecision(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockDecisionService) ListDecisions(_ context.Context, _ *connect.Request[specv1.ListDecisionsRequest]) (*connect.Response[specv1.ListDecisionsResponse], error) {
	resp, err := m.listDecisions()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockDecisionService) CreateDecision(_ context.Context, req *connect.Request[specv1.CreateDecisionRequest]) (*connect.Response[specv1.CreateDecisionResponse], error) {
	resp, err := m.createDecision(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Graph service mock ---

type mockGraphService struct {
	specgraphv1connect.GraphServiceClient
	addEdge         func(req *specv1.AddEdgeRequest) (*specv1.AddEdgeResponse, error)
	removeEdge      func(req *specv1.RemoveEdgeRequest) (*specv1.RemoveEdgeResponse, error)
	listEdges       func(req *specv1.ListEdgesRequest) (*specv1.ListEdgesResponse, error)
	getDeps         func(slug string) (*specv1.GetDependenciesResponse, error)
	getTransDeps    func(slug string) (*specv1.GetTransitiveDepsResponse, error)
	getImpact       func(slug string) (*specv1.GetImpactResponse, error)
	getReady        func() (*specv1.GetReadyResponse, error)
	getCriticalPath func(req *specv1.GetCriticalPathRequest) (*specv1.GetCriticalPathResponse, error)
	getFullGraph    func() (*specv1.GetFullGraphResponse, error)
}

func (m *mockGraphService) AddEdge(_ context.Context, req *connect.Request[specv1.AddEdgeRequest]) (*connect.Response[specv1.AddEdgeResponse], error) {
	resp, err := m.addEdge(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) ListEdges(_ context.Context, req *connect.Request[specv1.ListEdgesRequest]) (*connect.Response[specv1.ListEdgesResponse], error) {
	resp, err := m.listEdges(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetDependencies(_ context.Context, req *connect.Request[specv1.GetDependenciesRequest]) (*connect.Response[specv1.GetDependenciesResponse], error) {
	resp, err := m.getDeps(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetTransitiveDeps(_ context.Context, req *connect.Request[specv1.GetTransitiveDepsRequest]) (*connect.Response[specv1.GetTransitiveDepsResponse], error) {
	resp, err := m.getTransDeps(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetImpact(_ context.Context, req *connect.Request[specv1.GetImpactRequest]) (*connect.Response[specv1.GetImpactResponse], error) {
	resp, err := m.getImpact(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetReady(_ context.Context, _ *connect.Request[specv1.GetReadyRequest]) (*connect.Response[specv1.GetReadyResponse], error) {
	resp, err := m.getReady()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetFullGraph(_ context.Context, _ *connect.Request[specv1.GetFullGraphRequest]) (*connect.Response[specv1.GetFullGraphResponse], error) {
	resp, err := m.getFullGraph()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Health service mock ---

type mockHealthService struct {
	specgraphv1connect.ServerServiceClient
	health func() (*specv1.HealthResponse, error)
}

func (m *mockHealthService) Health(_ context.Context, _ *connect.Request[specv1.HealthRequest]) (*connect.Response[specv1.HealthResponse], error) {
	resp, err := m.health()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Constitution service mock ---

type mockConstitutionService struct {
	specgraphv1connect.ConstitutionServiceClient
	getConstitution func(req *specv1.GetConstitutionRequest) (*specv1.GetConstitutionResponse, error)
}

func (m *mockConstitutionService) GetConstitution(_ context.Context, req *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.GetConstitutionResponse], error) {
	resp, err := m.getConstitution(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Analytical pass service mock ---

type mockAnalyticalPassService struct {
	specgraphv1connect.AnalyticalPassServiceClient
	listFindings func(req *specv1.ListFindingsRequest) (*specv1.ListFindingsResponse, error)
}

func (m *mockAnalyticalPassService) ListFindings(_ context.Context, req *connect.Request[specv1.ListFindingsRequest]) (*connect.Response[specv1.ListFindingsResponse], error) {
	resp, err := m.listFindings(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
```

- [ ] **Step 2: Write spec + decision tool tests**

```go
// internal/mcp/tools_spec_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

func TestSpecTool_Get(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		getSpec: func(slug string) (*specv1.GetSpecResponse, error) {
			if slug == "auth" {
				return &specv1.GetSpecResponse{Spec: &specv1.Spec{
					Slug:   "auth",
					Intent: "Authentication system",
				}}, nil
			}
			return nil, connect.NewError(connect.CodeNotFound, nil)
		},
	}}
	h := &specTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action": "get",
		"slug":   "auth",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "auth")
	require.Contains(t, result.Content[0].Text, "Authentication system")
}

func TestSpecTool_GetNotFound(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		getSpec: func(slug string) (*specv1.GetSpecResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, nil)
		},
	}}
	h := &specTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action": "get",
		"slug":   "missing",
	})
	require.NoError(t, err) // ConnectRPC errors become tool errors, not Go errors
	require.True(t, result.IsError)
}

func TestSpecTool_List(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		listSpecs: func() (*specv1.ListSpecsResponse, error) {
			return &specv1.ListSpecsResponse{Specs: []*specv1.Spec{
				{Slug: "a", Intent: "A"},
				{Slug: "b", Intent: "B"},
			}}, nil
		},
	}}
	h := &specTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "list"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "a")
	require.Contains(t, result.Content[0].Text, "b")
}

func TestSpecTool_Create(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		createSpec: func(slug, intent string) (*specv1.CreateSpecResponse, error) {
			return &specv1.CreateSpecResponse{Spec: &specv1.Spec{
				Slug:   slug,
				Intent: intent,
			}}, nil
		},
	}}
	h := &specTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action": "create",
		"slug":   "new-spec",
		"intent": "Do the thing",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "new-spec")
}

func TestSpecTool_UnknownAction(t *testing.T) {
	h := &specTool{client: &Client{}}
	result, err := h.handle(context.Background(), map[string]any{"action": "delete"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "unknown action")
}

func TestDecisionTool_Get(t *testing.T) {
	c := &Client{Decision: &mockDecisionService{
		getDecision: func(slug string) (*specv1.GetDecisionResponse, error) {
			return &specv1.GetDecisionResponse{Decision: &specv1.Decision{
				Slug:  slug,
				Title: "Use JWT",
			}}, nil
		},
	}}
	h := &decisionTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action": "get",
		"slug":   "adr-001",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "Use JWT")
}

func TestDecisionTool_List(t *testing.T) {
	c := &Client{Decision: &mockDecisionService{
		listDecisions: func() (*specv1.ListDecisionsResponse, error) {
			return &specv1.ListDecisionsResponse{Decisions: []*specv1.Decision{
				{Slug: "adr-001", Title: "JWT"},
			}}, nil
		},
	}}
	h := &decisionTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "list"})
	require.NoError(t, err)
	require.Contains(t, result.Content[0].Text, "adr-001")
}
```

- [ ] **Step 3: Run tests, verify fail**

Run: `go test ./internal/mcp/ -run 'TestSpecTool|TestDecisionTool' -v`
Expected: FAIL — specTool, decisionTool not defined

- [ ] **Step 4: Write spec + decision handlers**

```go
// internal/mcp/tools_spec.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// --- spec tool ---

type specTool struct {
	client *Client
}

func (t *specTool) def() ToolDef {
	return ToolDef{
		Name:        "spec",
		Description: "Manage specs: create, read, list, or update specifications.",
		Tier:        TierCore,
		Schema: objectSchema(
			props{
				"action":     stringProp("Operation to perform", "get", "list", "create", "update"),
				"slug":       stringProp("Spec slug identifier (required for get, create, update)"),
				"intent":     stringProp("Spec intent description (for create/update)"),
				"priority":   stringProp("Priority level (for create/update)"),
				"complexity": stringProp("Complexity estimate (for create/update)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *specTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "get":
		resp, err := t.client.Spec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: stringParam(params, "slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "list":
		resp, err := t.client.Spec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "create":
		resp, err := t.client.Spec.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:       stringParam(params, "slug"),
			Intent:     stringParam(params, "intent"),
			Priority:   stringParam(params, "priority"),
			Complexity: stringParam(params, "complexity"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "update":
		resp, err := t.client.Spec.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
			Slug:       stringParam(params, "slug"),
			Intent:     stringParam(params, "intent"),
			Priority:   stringParam(params, "priority"),
			Complexity: stringParam(params, "complexity"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- decision tool ---

type decisionTool struct {
	client *Client
}

func (t *decisionTool) def() ToolDef {
	return ToolDef{
		Name:        "decision",
		Description: "Manage architectural decisions: create, read, list, or update ADRs.",
		Tier:        TierCore,
		Schema: objectSchema(
			props{
				"action":       stringProp("Operation to perform", "get", "list", "create", "update"),
				"slug":         stringProp("Decision slug identifier"),
				"title":        stringProp("Decision title (for create/update)"),
				"context":      stringProp("Decision context (for create/update)"),
				"decision":     stringProp("Decision text (for create/update)"),
				"consequences": stringProp("Decision consequences (for create/update)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *decisionTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "get":
		resp, err := t.client.Decision.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
			Slug: stringParam(params, "slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "list":
		resp, err := t.client.Decision.ListDecisions(ctx, connect.NewRequest(&specv1.ListDecisionsRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "create":
		resp, err := t.client.Decision.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
			Slug:         stringParam(params, "slug"),
			Title:        stringParam(params, "title"),
			Context:      stringParam(params, "context"),
			Decision:     stringParam(params, "decision"),
			Consequences: stringParam(params, "consequences"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "update":
		resp, err := t.client.Decision.UpdateDecision(ctx, connect.NewRequest(&specv1.UpdateDecisionRequest{
			Slug:         stringParam(params, "slug"),
			Title:        stringParam(params, "title"),
			Context:      stringParam(params, "context"),
			Decision:     stringParam(params, "decision"),
			Consequences: stringParam(params, "consequences"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// RegisterSpecTools adds spec and decision tools to the registry.
func RegisterSpecTools(r *Registry, c *Client) {
	st := &specTool{client: c}
	r.AddTool(st.def())

	dt := &decisionTool{client: c}
	r.AddTool(dt.def())
}
```

- [ ] **Step 5: Run tests, verify pass**

Run: `go test ./internal/mcp/ -run 'TestSpecTool|TestDecisionTool' -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/testhelpers_test.go internal/mcp/tools_spec.go internal/mcp/tools_spec_test.go
git commit -s -m "feat(mcp): add spec and decision tool handlers with tests"
```

---

### Task 5: Core Tier Tools — edge, graph_query, constitution, changes, findings, health

**Files:**
- Create: `internal/mcp/tools_graph.go`
- Create: `internal/mcp/tools_graph_test.go`
- Create: `internal/mcp/tools_core.go`
- Create: `internal/mcp/tools_core_test.go`

- [ ] **Step 1: Write graph tool tests**

```go
// internal/mcp/tools_graph_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

func TestEdgeTool_List(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		listEdges: func(req *specv1.ListEdgesRequest) (*specv1.ListEdgesResponse, error) {
			return &specv1.ListEdgesResponse{Edges: []*specv1.Edge{
				{FromSlug: "a", ToSlug: "b", EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON},
			}}, nil
		},
	}}
	h := &edgeTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "list", "slug": "a"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "a")
}

func TestGraphQueryTool_Ready(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getReady: func() (*specv1.GetReadyResponse, error) {
			return &specv1.GetReadyResponse{Slugs: []string{"spec-1", "spec-2"}}, nil
		},
	}}
	h := &graphQueryTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "ready"})
	require.NoError(t, err)
	require.Contains(t, result.Content[0].Text, "spec-1")
}

func TestGraphQueryTool_Dependencies(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getDeps: func(slug string) (*specv1.GetDependenciesResponse, error) {
			return &specv1.GetDependenciesResponse{}, nil
		},
	}}
	h := &graphQueryTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "dependencies", "slug": "auth"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestGraphQueryTool_FullGraph(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getFullGraph: func() (*specv1.GetFullGraphResponse, error) {
			return &specv1.GetFullGraphResponse{}, nil
		},
	}}
	h := &graphQueryTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "full"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}
```

- [ ] **Step 2: Write graph tool handlers**

```go
// internal/mcp/tools_graph.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// --- edge tool ---

type edgeTool struct {
	client *Client
}

func (t *edgeTool) def() ToolDef {
	return ToolDef{
		Name:        "edge",
		Description: "Manage graph edges between specs: add, remove, or list dependencies and relationships.",
		Tier:        TierCore,
		Schema: objectSchema(
			props{
				"action":    stringProp("Operation", "add", "remove", "list"),
				"slug":      stringProp("Spec slug (for list)"),
				"from_slug": stringProp("Source spec slug (for add/remove)"),
				"to_slug":   stringProp("Target spec slug (for add/remove)"),
				"edge_type": stringProp("Edge type", "DEPENDS_ON", "BLOCKS", "COMPOSES", "RELATES_TO", "INFORMS", "DECIDED_IN", "SUPERSEDES"),
				"direction": stringProp("Filter direction for list", "outgoing", "incoming", "both"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *edgeTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "add":
		resp, err := t.client.Graph.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: stringParam(params, "from_slug"),
			ToSlug:   stringParam(params, "to_slug"),
			EdgeType: edgeTypeFromString(stringParam(params, "edge_type")),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "remove":
		resp, err := t.client.Graph.RemoveEdge(ctx, connect.NewRequest(&specv1.RemoveEdgeRequest{
			FromSlug: stringParam(params, "from_slug"),
			ToSlug:   stringParam(params, "to_slug"),
			EdgeType: edgeTypeFromString(stringParam(params, "edge_type")),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "list":
		resp, err := t.client.Graph.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
			Slug: stringParam(params, "slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// edgeTypeFromString maps a string to the proto EdgeType enum.
func edgeTypeFromString(s string) specv1.EdgeType {
	if v, ok := specv1.EdgeType_value[s]; ok {
		return specv1.EdgeType(v)
	}
	// Try with prefix
	if v, ok := specv1.EdgeType_value["EDGE_TYPE_"+s]; ok {
		return specv1.EdgeType(v)
	}
	return specv1.EdgeType_EDGE_TYPE_UNSPECIFIED
}

// --- graph_query tool ---

type graphQueryTool struct {
	client *Client
}

func (t *graphQueryTool) def() ToolDef {
	return ToolDef{
		Name:        "graph_query",
		Description: "Query the spec dependency graph: find dependencies, transitive deps, impact analysis, ready specs, critical path, or full graph.",
		Tier:        TierCore,
		Schema: objectSchema(
			props{
				"action":    stringProp("Query type", "dependencies", "transitive_deps", "impact", "ready", "critical_path", "full"),
				"slug":      stringProp("Spec slug (for dependencies, transitive_deps, impact, critical_path)"),
				"from_slug": stringProp("Start slug (for critical_path)"),
				"to_slug":   stringProp("End slug (for critical_path)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *graphQueryTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "dependencies":
		resp, err := t.client.Graph.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{
			Slug: stringParam(params, "slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "transitive_deps":
		resp, err := t.client.Graph.GetTransitiveDeps(ctx, connect.NewRequest(&specv1.GetTransitiveDepsRequest{
			Slug: stringParam(params, "slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "impact":
		resp, err := t.client.Graph.GetImpact(ctx, connect.NewRequest(&specv1.GetImpactRequest{
			Slug: stringParam(params, "slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "ready":
		resp, err := t.client.Graph.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "critical_path":
		resp, err := t.client.Graph.GetCriticalPath(ctx, connect.NewRequest(&specv1.GetCriticalPathRequest{
			FromSlug: stringParam(params, "from_slug"),
			ToSlug:   stringParam(params, "to_slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "full":
		resp, err := t.client.Graph.GetFullGraph(ctx, connect.NewRequest(&specv1.GetFullGraphRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// RegisterGraphTools adds edge and graph_query tools to the registry.
func RegisterGraphTools(r *Registry, c *Client) {
	et := &edgeTool{client: c}
	r.AddTool(et.def())

	gq := &graphQueryTool{client: c}
	r.AddTool(gq.def())
}
```

- [ ] **Step 3: Run graph tool tests**

Run: `go test ./internal/mcp/ -run 'TestEdgeTool|TestGraphQueryTool' -v`
Expected: All PASS

- [ ] **Step 4: Write core tool tests**

```go
// internal/mcp/tools_core_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConstitutionTool_Get(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func(req *specv1.GetConstitutionRequest) (*specv1.GetConstitutionResponse, error) {
			return &specv1.GetConstitutionResponse{}, nil
		},
	}}
	h := &constitutionTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "get"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestFindingsTool_List(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		listFindings: func(req *specv1.ListFindingsRequest) (*specv1.ListFindingsResponse, error) {
			return &specv1.ListFindingsResponse{}, nil
		},
	}}
	h := &findingsTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "list"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestHealthTool(t *testing.T) {
	c := &Client{Health: &mockHealthService{
		health: func() (*specv1.HealthResponse, error) {
			return &specv1.HealthResponse{
				Status:     "ok",
				Version:    "1.0.0",
				ServerTime: timestamppb.Now(),
			}, nil
		},
	}}
	h := &healthTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.Contains(t, result.Content[0].Text, "ok")
}

func TestChangesTool_List(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		listChanges: func(slug string) (*specv1.ListChangesResponse, error) {
			return &specv1.ListChangesResponse{}, nil
		},
	}}
	h := &changesTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "list", "slug": "auth"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}
```

- [ ] **Step 5: Write core tool handlers**

```go
// internal/mcp/tools_core.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// --- constitution tool ---

type constitutionTool struct {
	client *Client
}

func (t *constitutionTool) def() ToolDef {
	return ToolDef{
		Name:        "constitution",
		Description: "Read or update the project constitution (layered ground truth). Supports per-layer access.",
		Tier:        TierCore,
		Schema: objectSchema(
			props{
				"action":  stringProp("Operation", "get", "update"),
				"layer":   stringProp("Constitution layer filter", "user", "org", "project", "domain"),
				"content": stringProp("New content (for update)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *constitutionTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "get":
		resp, err := t.client.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{
			Layer: stringParam(params, "layer"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "update":
		resp, err := t.client.Constitution.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Layer:   stringParam(params, "layer"),
			Content: stringParam(params, "content"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- changes tool ---

type changesTool struct {
	client *Client
}

func (t *changesTool) def() ToolDef {
	return ToolDef{
		Name:        "changes",
		Description: "View version history: list changes for a spec or compare two versions.",
		Tier:        TierCore,
		Schema: objectSchema(
			props{
				"action":  stringProp("Operation", "list", "compare"),
				"slug":    stringProp("Spec slug"),
				"version": intProp("Version number (for compare)"),
			},
			"action", "slug",
		),
		Handler: t.handle,
	}
}

func (t *changesTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "list":
		resp, err := t.client.Spec.ListChanges(ctx, connect.NewRequest(&specv1.ListChangesRequest{
			Slug: stringParam(params, "slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "compare":
		resp, err := t.client.Spec.CompareVersions(ctx, connect.NewRequest(&specv1.CompareVersionsRequest{
			Slug:    stringParam(params, "slug"),
			Version: int32Param(params, "version"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- findings tool ---

type findingsTool struct {
	client *Client
}

func (t *findingsTool) def() ToolDef {
	return ToolDef{
		Name:        "findings",
		Description: "List analytical findings, optionally filtered by pass type or spec.",
		Tier:        TierCore,
		Schema: objectSchema(
			props{
				"action":    stringProp("Operation", "list"),
				"slug":      stringProp("Filter by spec slug"),
				"pass_type": stringProp("Filter by pass type (e.g., constitution-check)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *findingsTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	resp, err := t.client.AnalyticalPass.ListFindings(ctx, connect.NewRequest(&specv1.ListFindingsRequest{
		Slug:     stringParam(params, "slug"),
		PassType: stringParam(params, "pass_type"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// --- health tool ---

type healthTool struct {
	client *Client
}

func (t *healthTool) def() ToolDef {
	return ToolDef{
		Name:        "health",
		Description: "Check SpecGraph server health, version, and status.",
		Tier:        TierCore,
		Schema:      objectSchema(props{}),
		Handler:     t.handle,
	}
}

func (t *healthTool) handle(ctx context.Context, _ map[string]any) (*ToolResult, error) {
	resp, err := t.client.Health.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// RegisterCoreTools adds constitution, changes, findings, and health tools.
func RegisterCoreTools(r *Registry, c *Client) {
	ct := &constitutionTool{client: c}
	r.AddTool(ct.def())

	ch := &changesTool{client: c}
	r.AddTool(ch.def())

	ft := &findingsTool{client: c}
	r.AddTool(ft.def())

	ht := &healthTool{client: c}
	r.AddTool(ht.def())
}
```

- [ ] **Step 6: Run all core tool tests**

Run: `go test ./internal/mcp/ -run 'TestConstitution|TestFindings|TestHealth|TestChanges|TestEdge|TestGraphQuery' -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/tools_graph.go internal/mcp/tools_graph_test.go internal/mcp/tools_core.go internal/mcp/tools_core_test.go
git commit -s -m "feat(mcp): add core tier tools — edge, graph_query, constitution, changes, findings, health"
```

---

### Task 6: Authoring Tier Tools

**Files:**
- Create: `internal/mcp/tools_authoring.go`
- Create: `internal/mcp/tools_authoring_test.go`
- Create: `internal/mcp/tools_lifecycle.go`
- Create: `internal/mcp/tools_lifecycle_test.go`

**Note:** Add mock methods for Authoring, Lifecycle, Sync, Export, AnalyticalPass services to `testhelpers_test.go` as needed. Follow the same mock pattern from Task 4.

- [ ] **Step 1: Add authoring/lifecycle mocks to testhelpers_test.go**

Append these mocks to `internal/mcp/testhelpers_test.go`:

```go
// --- Authoring service mock ---

type mockAuthoringService struct {
	specgraphv1connect.AuthoringServiceClient
	spark       func(req *specv1.SparkRequest) (*specv1.SparkResponse, error)
	shape       func(req *specv1.ShapeRequest) (*specv1.ShapeResponse, error)
	specify     func(req *specv1.SpecifyRequest) (*specv1.SpecifyResponse, error)
	decompose   func(req *specv1.DecomposeRequest) (*specv1.DecomposeResponse, error)
	approve     func(req *specv1.ApproveRequest) (*specv1.ApproveResponse, error)
	amend       func(req *specv1.AmendRequest) (*specv1.AmendResponse, error)
	supersede   func(req *specv1.SupersedeRequest) (*specv1.SupersedeResponse, error)
	getPrompts  func(req *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error)
	recordConvo func(req *specv1.RecordConversationRequest) (*specv1.RecordConversationResponse, error)
	listConvos  func(req *specv1.ListConversationsRequest) (*specv1.ListConversationsResponse, error)
}

func (m *mockAuthoringService) Spark(_ context.Context, req *connect.Request[specv1.SparkRequest]) (*connect.Response[specv1.SparkResponse], error) {
	resp, err := m.spark(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAuthoringService) Approve(_ context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	resp, err := m.approve(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAuthoringService) GetPrompts(_ context.Context, req *connect.Request[specv1.GetPromptsRequest]) (*connect.Response[specv1.GetPromptsResponse], error) {
	resp, err := m.getPrompts(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAuthoringService) RecordConversation(_ context.Context, req *connect.Request[specv1.RecordConversationRequest]) (*connect.Response[specv1.RecordConversationResponse], error) {
	resp, err := m.recordConvo(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAuthoringService) ListConversations(_ context.Context, req *connect.Request[specv1.ListConversationsRequest]) (*connect.Response[specv1.ListConversationsResponse], error) {
	resp, err := m.listConvos(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Lifecycle service mock ---

type mockLifecycleService struct {
	specgraphv1connect.LifecycleServiceClient
	checkDrift func(req *specv1.DriftCheckRequest) (*specv1.DriftCheckResponse, error)
	ackDrift   func(req *specv1.DriftAcknowledgeRequest) (*specv1.DriftAcknowledgeResponse, error)
	lint       func(req *specv1.LintRequest) (*specv1.LintResponse, error)
}

func (m *mockLifecycleService) CheckDrift(_ context.Context, req *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	resp, err := m.checkDrift(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockLifecycleService) Lint(_ context.Context, req *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	resp, err := m.lint(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Sync service mock ---

type mockSyncService struct {
	specgraphv1connect.SyncServiceClient
	syncBeads func(req *specv1.SyncBeadsRequest) (*specv1.SyncResponse, error)
	syncGH    func(req *specv1.SyncGitHubRequest) (*specv1.SyncResponse, error)
	getStatus func(req *specv1.SyncStatusRequest) (*specv1.SyncStatusResponse, error)
}

func (m *mockSyncService) SyncBeads(_ context.Context, req *connect.Request[specv1.SyncBeadsRequest]) (*connect.Response[specv1.SyncResponse], error) {
	resp, err := m.syncBeads(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSyncService) GetSyncStatus(_ context.Context, req *connect.Request[specv1.SyncStatusRequest]) (*connect.Response[specv1.SyncStatusResponse], error) {
	resp, err := m.getStatus(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Export service mock ---

type mockExportService struct {
	specgraphv1connect.ExportServiceClient
	exportProject func(req *specv1.ExportProjectRequest) (*specv1.ExportProjectResponse, error)
	importProject func(req *specv1.ImportProjectRequest) (*specv1.ImportProjectResponse, error)
	verifyExport  func(req *specv1.VerifyExportRequest) (*specv1.VerifyExportResponse, error)
}

func (m *mockExportService) ExportProject(_ context.Context, req *connect.Request[specv1.ExportProjectRequest]) (*connect.Response[specv1.ExportProjectResponse], error) {
	resp, err := m.exportProject(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
```

- [ ] **Step 2: Write authoring tool handler**

```go
// internal/mcp/tools_authoring.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// --- author tool ---

type authorTool struct {
	client *Client
}

func (t *authorTool) def() ToolDef {
	return ToolDef{
		Name:        "author",
		Description: "Drive the authoring funnel: spark an idea, shape it, specify details, decompose into slices, approve, amend, or supersede a spec.",
		Tier:        TierAuthoring,
		Schema: objectSchema(
			props{
				"action":    stringProp("Funnel stage", "spark", "shape", "specify", "decompose", "approve", "amend", "supersede"),
				"spec_slug": stringProp("Target spec slug (all actions except spark)"),
				"topic":     stringProp("Topic for spark"),
				"context":   stringProp("Additional context for spark"),
				"output":    stringProp("JSON output from the stage (shape, specify, decompose)"),
				"reason":    stringProp("Reason for amend/supersede"),
				"new_slug":  stringProp("New spec slug (for supersede)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *authorTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "spark":
		resp, err := t.client.Authoring.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Topic:   stringParam(params, "topic"),
			Context: stringParam(params, "context"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "shape":
		resp, err := t.client.Authoring.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Output:   stringParam(params, "output"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "specify":
		resp, err := t.client.Authoring.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Output:   stringParam(params, "output"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "decompose":
		resp, err := t.client.Authoring.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Output:   stringParam(params, "output"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "approve":
		resp, err := t.client.Authoring.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
			SpecSlug: stringParam(params, "spec_slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "amend":
		resp, err := t.client.Authoring.Amend(ctx, connect.NewRequest(&specv1.AmendRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Reason:   stringParam(params, "reason"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "supersede":
		resp, err := t.client.Authoring.Supersede(ctx, connect.NewRequest(&specv1.SupersedeRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			NewSlug:  stringParam(params, "new_slug"),
			Reason:   stringParam(params, "reason"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- conversation tool ---

type conversationTool struct {
	client *Client
}

func (t *conversationTool) def() ToolDef {
	return ToolDef{
		Name:        "conversation",
		Description: "Record or list authoring conversations for audit trail.",
		Tier:        TierAuthoring,
		Schema: objectSchema(
			props{
				"action":    stringProp("Operation", "record", "list"),
				"spec_slug": stringProp("Spec slug"),
				"stage":     stringProp("Authoring stage"),
				"messages":  stringProp("JSON array of conversation messages (for record)"),
			},
			"action", "spec_slug",
		),
		Handler: t.handle,
	}
}

func (t *conversationTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "record":
		resp, err := t.client.Authoring.RecordConversation(ctx, connect.NewRequest(&specv1.RecordConversationRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Stage:    stringParam(params, "stage"),
			Messages: stringParam(params, "messages"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "list":
		resp, err := t.client.Authoring.ListConversations(ctx, connect.NewRequest(&specv1.ListConversationsRequest{
			SpecSlug: stringParam(params, "spec_slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- analytical_pass tool ---

type analyticalPassTool struct {
	client *Client
}

func (t *analyticalPassTool) def() ToolDef {
	return ToolDef{
		Name:        "analytical_pass",
		Description: "Run analytical passes (constitution check, dependency review) or store findings.",
		Tier:        TierAuthoring,
		Schema: objectSchema(
			props{
				"action":    stringProp("Operation", "run", "store"),
				"spec_slug": stringProp("Spec slug to analyze"),
				"pass_type": stringProp("Pass type (e.g., constitution-check)"),
				"findings":  stringProp("JSON findings data (for store)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *analyticalPassTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "run":
		resp, err := t.client.AnalyticalPass.RunAnalyticalPass(ctx, connect.NewRequest(&specv1.RunAnalyticalPassRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			PassType: stringParam(params, "pass_type"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "store":
		resp, err := t.client.AnalyticalPass.StoreFindings(ctx, connect.NewRequest(&specv1.StoreFindingsRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			PassType: stringParam(params, "pass_type"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// RegisterAuthoringTools adds author, conversation, and analytical_pass tools.
func RegisterAuthoringTools(r *Registry, c *Client) {
	at := &authorTool{client: c}
	r.AddTool(at.def())

	ct := &conversationTool{client: c}
	r.AddTool(ct.def())

	ap := &analyticalPassTool{client: c}
	r.AddTool(ap.def())
}
```

- [ ] **Step 3: Write lifecycle tool handler**

```go
// internal/mcp/tools_lifecycle.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// --- drift tool ---

type driftTool struct {
	client *Client
}

func (t *driftTool) def() ToolDef {
	return ToolDef{
		Name:        "drift",
		Description: "Check for specification drift or acknowledge known drift.",
		Tier:        TierAuthoring,
		Schema: objectSchema(
			props{
				"action":    stringProp("Operation", "check", "acknowledge"),
				"spec_slug": stringProp("Spec slug"),
				"all":       boolProp("Acknowledge all drift for this spec"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *driftTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "check":
		resp, err := t.client.Lifecycle.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{
			Slug: stringParam(params, "spec_slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "acknowledge":
		resp, err := t.client.Lifecycle.AcknowledgeDrift(ctx, connect.NewRequest(&specv1.DriftAcknowledgeRequest{
			Slug: stringParam(params, "spec_slug"),
			All:  boolParam(params, "all"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- lint tool ---

type lintTool struct {
	client *Client
}

func (t *lintTool) def() ToolDef {
	return ToolDef{
		Name:        "lint",
		Description: "Run the spec linter to check for schema violations and dependency issues.",
		Tier:        TierAuthoring,
		Schema: objectSchema(
			props{
				"spec_slug": stringProp("Spec slug to lint"),
			},
			"spec_slug",
		),
		Handler: t.handle,
	}
}

func (t *lintTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	resp, err := t.client.Lifecycle.Lint(ctx, connect.NewRequest(&specv1.LintRequest{
		Slug: stringParam(params, "spec_slug"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// --- sync tool ---

type syncTool struct {
	client *Client
}

func (t *syncTool) def() ToolDef {
	return ToolDef{
		Name:        "sync",
		Description: "Sync specs with external systems: beads issue tracker, GitHub, or check sync status.",
		Tier:        TierAuthoring,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation", "beads", "github", "status"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *syncTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "beads":
		resp, err := t.client.Sync.SyncBeads(ctx, connect.NewRequest(&specv1.SyncBeadsRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "github":
		resp, err := t.client.Sync.SyncGitHub(ctx, connect.NewRequest(&specv1.SyncGitHubRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "status":
		resp, err := t.client.Sync.GetSyncStatus(ctx, connect.NewRequest(&specv1.SyncStatusRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- export tool ---

type exportTool struct {
	client *Client
}

func (t *exportTool) def() ToolDef {
	return ToolDef{
		Name:        "export",
		Description: "Export, import, or verify a SpecGraph project backup.",
		Tier:        TierAuthoring,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation", "export", "import", "verify"),
				"path":   stringProp("File path for export/import"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *exportTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "export":
		resp, err := t.client.Export.ExportProject(ctx, connect.NewRequest(&specv1.ExportProjectRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "import":
		resp, err := t.client.Export.ImportProject(ctx, connect.NewRequest(&specv1.ImportProjectRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "verify":
		resp, err := t.client.Export.VerifyExport(ctx, connect.NewRequest(&specv1.VerifyExportRequest{}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// RegisterLifecycleTools adds drift, lint, sync, and export tools.
func RegisterLifecycleTools(r *Registry, c *Client) {
	dt := &driftTool{client: c}
	r.AddTool(dt.def())

	lt := &lintTool{client: c}
	r.AddTool(lt.def())

	st := &syncTool{client: c}
	r.AddTool(st.def())

	et := &exportTool{client: c}
	r.AddTool(et.def())
}
```

- [ ] **Step 4: Write authoring + lifecycle tests**

```go
// internal/mcp/tools_authoring_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

func TestAuthorTool_Spark(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		spark: func(req *specv1.SparkRequest) (*specv1.SparkResponse, error) {
			return &specv1.SparkResponse{}, nil
		},
	}}
	h := &authorTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action":  "spark",
		"topic":   "auth system",
		"context": "microservices",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestAuthorTool_Approve(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		approve: func(req *specv1.ApproveRequest) (*specv1.ApproveResponse, error) {
			return &specv1.ApproveResponse{}, nil
		},
	}}
	h := &authorTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action":    "approve",
		"spec_slug": "auth",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestConversationTool_List(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		listConvos: func(req *specv1.ListConversationsRequest) (*specv1.ListConversationsResponse, error) {
			return &specv1.ListConversationsResponse{}, nil
		},
	}}
	h := &conversationTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action":    "list",
		"spec_slug": "auth",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}
```

```go
// internal/mcp/tools_lifecycle_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

func TestDriftTool_Check(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{
		checkDrift: func(req *specv1.DriftCheckRequest) (*specv1.DriftCheckResponse, error) {
			return &specv1.DriftCheckResponse{}, nil
		},
	}}
	h := &driftTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action":    "check",
		"spec_slug": "auth",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestLintTool(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{
		lint: func(req *specv1.LintRequest) (*specv1.LintResponse, error) {
			return &specv1.LintResponse{}, nil
		},
	}}
	h := &lintTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"spec_slug": "auth"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSyncTool_Status(t *testing.T) {
	c := &Client{Sync: &mockSyncService{
		getStatus: func(req *specv1.SyncStatusRequest) (*specv1.SyncStatusResponse, error) {
			return &specv1.SyncStatusResponse{}, nil
		},
	}}
	h := &syncTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "status"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestExportTool_Export(t *testing.T) {
	c := &Client{Export: &mockExportService{
		exportProject: func(req *specv1.ExportProjectRequest) (*specv1.ExportProjectResponse, error) {
			return &specv1.ExportProjectResponse{}, nil
		},
	}}
	h := &exportTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"action": "export"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}
```

- [ ] **Step 5: Run all authoring tier tests**

Run: `go test ./internal/mcp/ -run 'TestAuthor|TestConversation|TestDrift|TestLint|TestSync|TestExport' -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/tools_authoring.go internal/mcp/tools_authoring_test.go internal/mcp/tools_lifecycle.go internal/mcp/tools_lifecycle_test.go internal/mcp/testhelpers_test.go
git commit -s -m "feat(mcp): add authoring tier tools — author, conversation, analytical_pass, drift, lint, sync, export"
```

---

### Task 7: Execution Tier Tools

**Files:**
- Create: `internal/mcp/tools_execution.go`
- Create: `internal/mcp/tools_execution_test.go`

**Note:** Add execution-related mocks (Claim, Slice, Execution services) to `testhelpers_test.go`.

- [ ] **Step 1: Add execution mocks to testhelpers_test.go**

Append:

```go
// --- Claim service mock ---

type mockClaimService struct {
	specgraphv1connect.ClaimServiceClient
	claimSpec   func(req *specv1.ClaimSpecRequest) (*specv1.ClaimSpecResponse, error)
	unclaimSpec func(req *specv1.UnclaimSpecRequest) (*specv1.UnclaimSpecResponse, error)
	heartbeat   func(req *specv1.HeartbeatRequest) (*specv1.HeartbeatResponse, error)
}

func (m *mockClaimService) ClaimSpec(_ context.Context, req *connect.Request[specv1.ClaimSpecRequest]) (*connect.Response[specv1.ClaimSpecResponse], error) {
	resp, err := m.claimSpec(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockClaimService) UnclaimSpec(_ context.Context, req *connect.Request[specv1.UnclaimSpecRequest]) (*connect.Response[specv1.UnclaimSpecResponse], error) {
	resp, err := m.unclaimSpec(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockClaimService) Heartbeat(_ context.Context, req *connect.Request[specv1.HeartbeatRequest]) (*connect.Response[specv1.HeartbeatResponse], error) {
	resp, err := m.heartbeat(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Slice service mock ---

type mockSliceService struct {
	specgraphv1connect.SliceServiceClient
	listSlices    func(req *specv1.ListSlicesRequest) (*specv1.ListSlicesResponse, error)
	getSlice      func(req *specv1.GetSliceRequest) (*specv1.GetSliceResponse, error)
	claimSlice    func(req *specv1.ClaimSliceRequest) (*specv1.ClaimSliceResponse, error)
	completeSlice func(req *specv1.CompleteSliceRequest) (*specv1.CompleteSliceResponse, error)
}

func (m *mockSliceService) ListSlices(_ context.Context, req *connect.Request[specv1.ListSlicesRequest]) (*connect.Response[specv1.ListSlicesResponse], error) {
	resp, err := m.listSlices(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSliceService) GetSlice(_ context.Context, req *connect.Request[specv1.GetSliceRequest]) (*connect.Response[specv1.GetSliceResponse], error) {
	resp, err := m.getSlice(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSliceService) ClaimSlice(_ context.Context, req *connect.Request[specv1.ClaimSliceRequest]) (*connect.Response[specv1.ClaimSliceResponse], error) {
	resp, err := m.claimSlice(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSliceService) CompleteSlice(_ context.Context, req *connect.Request[specv1.CompleteSliceRequest]) (*connect.Response[specv1.CompleteSliceResponse], error) {
	resp, err := m.completeSlice(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- Execution service mock ---

type mockExecutionService struct {
	specgraphv1connect.ExecutionServiceClient
	generateBundle   func(req *specv1.GenerateBundleRequest) (*specv1.GenerateBundleResponse, error)
	getPrime         func(req *specv1.GetPrimeRequest) (*specv1.PrimeResponse, error)
	reportProgress   func(req *specv1.ReportProgressRequest) (*specv1.ReportProgressResponse, error)
	reportBlocker    func(req *specv1.ReportBlockerRequest) (*specv1.ReportBlockerResponse, error)
	reportCompletion func(req *specv1.ReportCompletionRequest) (*specv1.ReportCompletionResponse, error)
	getEvents        func(req *specv1.GetExecutionEventsRequest) (*specv1.GetExecutionEventsResponse, error)
}

func (m *mockExecutionService) GenerateBundle(_ context.Context, req *connect.Request[specv1.GenerateBundleRequest]) (*connect.Response[specv1.GenerateBundleResponse], error) {
	resp, err := m.generateBundle(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) GetPrime(_ context.Context, req *connect.Request[specv1.GetPrimeRequest]) (*connect.Response[specv1.PrimeResponse], error) {
	resp, err := m.getPrime(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) ReportProgress(_ context.Context, req *connect.Request[specv1.ReportProgressRequest]) (*connect.Response[specv1.ReportProgressResponse], error) {
	resp, err := m.reportProgress(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) ReportBlocker(_ context.Context, req *connect.Request[specv1.ReportBlockerRequest]) (*connect.Response[specv1.ReportBlockerResponse], error) {
	resp, err := m.reportBlocker(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) ReportCompletion(_ context.Context, req *connect.Request[specv1.ReportCompletionRequest]) (*connect.Response[specv1.ReportCompletionResponse], error) {
	resp, err := m.reportCompletion(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) GetExecutionEvents(_ context.Context, req *connect.Request[specv1.GetExecutionEventsRequest]) (*connect.Response[specv1.GetExecutionEventsResponse], error) {
	resp, err := m.getEvents(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
```

- [ ] **Step 2: Write execution tool handler**

```go
// internal/mcp/tools_execution.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"time"
)

// --- claim tool ---

type claimTool struct {
	client *Client
}

func (t *claimTool) def() ToolDef {
	return ToolDef{
		Name:        "claim",
		Description: "Manage spec ownership: claim a spec for execution, release it, or send a heartbeat to maintain the lease.",
		Tier:        TierExecution,
		Schema: objectSchema(
			props{
				"action":         stringProp("Operation", "claim", "unclaim", "heartbeat"),
				"spec_slug":      stringProp("Spec slug"),
				"agent":          stringProp("Agent identifier"),
				"lease_duration": stringProp("Lease duration (e.g., '5m', '1h')"),
			},
			"action", "spec_slug",
		),
		Handler: t.handle,
	}
}

func (t *claimTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "claim":
		req := &specv1.ClaimSpecRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Agent:    stringParam(params, "agent"),
		}
		if d := stringParam(params, "lease_duration"); d != "" {
			if dur, err := time.ParseDuration(d); err == nil {
				req.LeaseDuration = durationpb.New(dur)
			}
		}
		resp, err := t.client.Claim.ClaimSpec(ctx, connect.NewRequest(req))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "unclaim":
		resp, err := t.client.Claim.UnclaimSpec(ctx, connect.NewRequest(&specv1.UnclaimSpecRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Agent:    stringParam(params, "agent"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "heartbeat":
		resp, err := t.client.Claim.Heartbeat(ctx, connect.NewRequest(&specv1.HeartbeatRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Agent:    stringParam(params, "agent"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- slice tool ---

type sliceTool struct {
	client *Client
}

func (t *sliceTool) def() ToolDef {
	return ToolDef{
		Name:        "slice",
		Description: "Manage work slices: list, get details, claim for execution, or mark complete.",
		Tier:        TierExecution,
		Schema: objectSchema(
			props{
				"action":    stringProp("Operation", "list", "get", "claim", "complete"),
				"spec_slug": stringProp("Parent spec slug (for list)"),
				"slice_id":  stringProp("Slice ID (for get, claim, complete)"),
				"agent":     stringProp("Agent identifier (for claim)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *sliceTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "list":
		resp, err := t.client.Slice.ListSlices(ctx, connect.NewRequest(&specv1.ListSlicesRequest{
			SpecSlug: stringParam(params, "spec_slug"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "get":
		resp, err := t.client.Slice.GetSlice(ctx, connect.NewRequest(&specv1.GetSliceRequest{
			SliceId: stringParam(params, "slice_id"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "claim":
		resp, err := t.client.Slice.ClaimSlice(ctx, connect.NewRequest(&specv1.ClaimSliceRequest{
			SliceId: stringParam(params, "slice_id"),
			Agent:   stringParam(params, "agent"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "complete":
		resp, err := t.client.Slice.CompleteSlice(ctx, connect.NewRequest(&specv1.CompleteSliceRequest{
			SliceId: stringParam(params, "slice_id"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- bundle tool ---

type bundleTool struct {
	client *Client
}

func (t *bundleTool) def() ToolDef {
	return ToolDef{
		Name:        "bundle",
		Description: "Generate an execution bundle for a spec — contains all context needed for implementation.",
		Tier:        TierExecution,
		Schema: objectSchema(
			props{
				"spec_slug": stringProp("Spec slug to bundle"),
			},
			"spec_slug",
		),
		Handler: t.handle,
	}
}

func (t *bundleTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	resp, err := t.client.Execution.GenerateBundle(ctx, connect.NewRequest(&specv1.GenerateBundleRequest{
		SpecSlug: stringParam(params, "spec_slug"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// --- prime tool ---

type primeTool struct {
	client *Client
}

func (t *primeTool) def() ToolDef {
	return ToolDef{
		Name:        "prime",
		Description: "Get execution prime data: constitution, dependencies, and full spec context.",
		Tier:        TierExecution,
		Schema: objectSchema(
			props{
				"spec_slug": stringProp("Spec slug"),
			},
			"spec_slug",
		),
		Handler: t.handle,
	}
}

func (t *primeTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	resp, err := t.client.Execution.GetPrime(ctx, connect.NewRequest(&specv1.GetPrimeRequest{
		SpecSlug: stringParam(params, "spec_slug"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// --- report tool ---

type reportTool struct {
	client *Client
}

func (t *reportTool) def() ToolDef {
	return ToolDef{
		Name:        "report",
		Description: "Report execution status: progress updates, blockers, or completion.",
		Tier:        TierExecution,
		Schema: objectSchema(
			props{
				"action":    stringProp("Report type", "progress", "blocker", "completion"),
				"spec_slug": stringProp("Spec slug"),
				"agent":     stringProp("Agent identifier"),
				"message":   stringProp("Status message"),
				"percent":   intProp("Completion percentage (for progress)"),
			},
			"action", "spec_slug",
		),
		Handler: t.handle,
	}
}

func (t *reportTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "progress":
		resp, err := t.client.Execution.ReportProgress(ctx, connect.NewRequest(&specv1.ReportProgressRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Agent:    stringParam(params, "agent"),
			Message:  stringParam(params, "message"),
			Percent:  int32Param(params, "percent"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "blocker":
		resp, err := t.client.Execution.ReportBlocker(ctx, connect.NewRequest(&specv1.ReportBlockerRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Agent:    stringParam(params, "agent"),
			Message:  stringParam(params, "message"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	case "completion":
		resp, err := t.client.Execution.ReportCompletion(ctx, connect.NewRequest(&specv1.ReportCompletionRequest{
			SpecSlug: stringParam(params, "spec_slug"),
			Agent:    stringParam(params, "agent"),
			Message:  stringParam(params, "message"),
		}))
		if err != nil {
			return connectErrResult(err)
		}
		return jsonResult(resp.Msg), nil

	default:
		return errResult("unknown action: " + action), nil
	}
}

// --- execution_events tool ---

type executionEventsTool struct {
	client *Client
}

func (t *executionEventsTool) def() ToolDef {
	return ToolDef{
		Name:        "execution_events",
		Description: "Get execution event log for a spec — progress, blockers, completions.",
		Tier:        TierExecution,
		Schema: objectSchema(
			props{
				"spec_slug": stringProp("Spec slug"),
			},
			"spec_slug",
		),
		Handler: t.handle,
	}
}

func (t *executionEventsTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	resp, err := t.client.Execution.GetExecutionEvents(ctx, connect.NewRequest(&specv1.GetExecutionEventsRequest{
		SpecSlug: stringParam(params, "spec_slug"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// RegisterExecutionTools adds claim, slice, bundle, prime, report, and execution_events tools.
func RegisterExecutionTools(r *Registry, c *Client) {
	cl := &claimTool{client: c}
	r.AddTool(cl.def())

	sl := &sliceTool{client: c}
	r.AddTool(sl.def())

	bt := &bundleTool{client: c}
	r.AddTool(bt.def())

	pt := &primeTool{client: c}
	r.AddTool(pt.def())

	rt := &reportTool{client: c}
	r.AddTool(rt.def())

	et := &executionEventsTool{client: c}
	r.AddTool(et.def())
}
```

- [ ] **Step 3: Write execution tool tests**

```go
// internal/mcp/tools_execution_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

func TestClaimTool_Claim(t *testing.T) {
	c := &Client{Claim: &mockClaimService{
		claimSpec: func(req *specv1.ClaimSpecRequest) (*specv1.ClaimSpecResponse, error) {
			return &specv1.ClaimSpecResponse{}, nil
		},
	}}
	h := &claimTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action":    "claim",
		"spec_slug": "auth",
		"agent":     "polecat-1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSliceTool_List(t *testing.T) {
	c := &Client{Slice: &mockSliceService{
		listSlices: func(req *specv1.ListSlicesRequest) (*specv1.ListSlicesResponse, error) {
			return &specv1.ListSlicesResponse{}, nil
		},
	}}
	h := &sliceTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action":    "list",
		"spec_slug": "auth",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestBundleTool(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		generateBundle: func(req *specv1.GenerateBundleRequest) (*specv1.GenerateBundleResponse, error) {
			return &specv1.GenerateBundleResponse{}, nil
		},
	}}
	h := &bundleTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"spec_slug": "auth"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestPrimeTool(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		getPrime: func(req *specv1.GetPrimeRequest) (*specv1.PrimeResponse, error) {
			return &specv1.PrimeResponse{}, nil
		},
	}}
	h := &primeTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"spec_slug": "auth"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestReportTool_Progress(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		reportProgress: func(req *specv1.ReportProgressRequest) (*specv1.ReportProgressResponse, error) {
			return &specv1.ReportProgressResponse{}, nil
		},
	}}
	h := &reportTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{
		"action":    "progress",
		"spec_slug": "auth",
		"agent":     "polecat-1",
		"message":   "halfway there",
		"percent":   float64(50),
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestExecutionEventsTool(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		getEvents: func(req *specv1.GetExecutionEventsRequest) (*specv1.GetExecutionEventsResponse, error) {
			return &specv1.GetExecutionEventsResponse{}, nil
		},
	}}
	h := &executionEventsTool{client: c}

	result, err := h.handle(context.Background(), map[string]any{"spec_slug": "auth"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}
```

- [ ] **Step 4: Run all execution tier tests**

Run: `go test ./internal/mcp/ -run 'TestClaim|TestSlice|TestBundle|TestPrime|TestReport|TestExecutionEvents' -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/tools_execution.go internal/mcp/tools_execution_test.go internal/mcp/testhelpers_test.go
git commit -s -m "feat(mcp): add execution tier tools — claim, slice, bundle, prime, report, execution_events"
```

---

### Task 8: Resources

**Files:**
- Create: `internal/mcp/resources.go`
- Create: `internal/mcp/resources_test.go`

- [ ] **Step 1: Write resource tests**

```go
// internal/mcp/resources_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

func TestSpecResource(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		getSpec: func(slug string) (*specv1.GetSpecResponse, error) {
			return &specv1.GetSpecResponse{Spec: &specv1.Spec{
				Slug:   slug,
				Intent: "Test intent",
			}}, nil
		},
	}}
	h := specResourceHandler(c)

	contents, err := h(context.Background(), "specgraph://spec/auth")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Contains(t, contents[0].Text, "auth")
	require.Equal(t, "application/json", contents[0].MimeType)
}

func TestSpecListResource(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		listSpecs: func() (*specv1.ListSpecsResponse, error) {
			return &specv1.ListSpecsResponse{Specs: []*specv1.Spec{
				{Slug: "a"},
				{Slug: "b"},
			}}, nil
		},
	}}
	h := specListResourceHandler(c)

	contents, err := h(context.Background(), "specgraph://specs")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Contains(t, contents[0].Text, "a")
}

func TestConstitutionResource(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func(req *specv1.GetConstitutionRequest) (*specv1.GetConstitutionResponse, error) {
			return &specv1.GetConstitutionResponse{}, nil
		},
	}}
	h := constitutionResourceHandler(c)

	contents, err := h(context.Background(), "specgraph://constitution")
	require.NoError(t, err)
	require.Len(t, contents, 1)
}

func TestConstitutionLayerResource(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func(req *specv1.GetConstitutionRequest) (*specv1.GetConstitutionResponse, error) {
			return &specv1.GetConstitutionResponse{}, nil
		},
	}}
	h := constitutionLayerResourceHandler(c)

	contents, err := h(context.Background(), "specgraph://constitution/domain")
	require.NoError(t, err)
	require.Len(t, contents, 1)
}
```

- [ ] **Step 2: Write resource handlers**

```go
// internal/mcp/resources.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func resourceJSON(uri string, msg proto.Message) ([]ResourceContent, error) {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return []ResourceContent{{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(data),
	}}, nil
}

// extractSlugFromURI extracts the last path segment from a specgraph:// URI.
// e.g., "specgraph://spec/auth-system" → "auth-system"
func extractSlugFromURI(uri string) string {
	parts := strings.Split(uri, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func specResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		slug := extractSlugFromURI(uri)
		resp, err := c.Spec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{Slug: slug}))
		if err != nil {
			return nil, err
		}
		return resourceJSON(uri, resp.Msg)
	}
}

func specListResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		resp, err := c.Spec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		if err != nil {
			return nil, err
		}
		return resourceJSON(uri, resp.Msg)
	}
}

func decisionResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		slug := extractSlugFromURI(uri)
		resp, err := c.Decision.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{Slug: slug}))
		if err != nil {
			return nil, err
		}
		return resourceJSON(uri, resp.Msg)
	}
}

func constitutionResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		resp, err := c.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		if err != nil {
			return nil, err
		}
		return resourceJSON(uri, resp.Msg)
	}
}

func constitutionLayerResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		layer := extractSlugFromURI(uri)
		resp, err := c.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{
			Layer: layer,
		}))
		if err != nil {
			return nil, err
		}
		return resourceJSON(uri, resp.Msg)
	}
}

func graphResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		resp, err := c.Graph.GetFullGraph(ctx, connect.NewRequest(&specv1.GetFullGraphRequest{}))
		if err != nil {
			return nil, err
		}
		return resourceJSON(uri, resp.Msg)
	}
}

func readyResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		resp, err := c.Graph.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{}))
		if err != nil {
			return nil, err
		}
		return resourceJSON(uri, resp.Msg)
	}
}

func findingsResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		// Extract pass_type from query if present
		passType := ""
		if idx := strings.Index(uri, "pass_type="); idx >= 0 {
			passType = uri[idx+len("pass_type="):]
		}
		resp, err := c.AnalyticalPass.ListFindings(ctx, connect.NewRequest(&specv1.ListFindingsRequest{
			PassType: passType,
		}))
		if err != nil {
			return nil, err
		}
		return resourceJSON(uri, resp.Msg)
	}
}

func changesResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		// URI: specgraph://spec/{slug}/changes → extract slug (second-to-last segment)
		parts := strings.Split(strings.TrimPrefix(uri, "specgraph://"), "/")
		slug := ""
		if len(parts) >= 3 {
			slug = parts[1] // spec/{slug}/changes
		}
		resp, err := c.Spec.ListChanges(ctx, connect.NewRequest(&specv1.ListChangesRequest{Slug: slug}))
		if err != nil {
			return nil, err
		}
		return resourceJSON(uri, resp.Msg)
	}
}

// RegisterResources adds all MCP resource definitions to the registry.
func RegisterResources(r *Registry, c *Client) {
	r.AddResource(ResourceDef{
		URI:         "specgraph://spec/{slug}",
		Name:        "Spec",
		Description: "Full spec content by slug",
		MimeType:    "application/json",
		IsTemplate:  true,
		Handler:     specResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://specs",
		Name:        "Spec list",
		Description: "All specs (summary view)",
		MimeType:    "application/json",
		Handler:     specListResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://decision/{slug}",
		Name:        "Decision",
		Description: "Decision content by slug",
		MimeType:    "application/json",
		IsTemplate:  true,
		Handler:     decisionResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://constitution",
		Name:        "Constitution (merged)",
		Description: "Fully merged constitution, all layers applied",
		MimeType:    "application/json",
		Handler:     constitutionResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://constitution/{layer}",
		Name:        "Constitution layer",
		Description: "Single constitution layer: user, org, project, or domain",
		MimeType:    "application/json",
		IsTemplate:  true,
		Handler:     constitutionLayerResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://graph",
		Name:        "Full graph",
		Description: "Complete spec dependency graph",
		MimeType:    "application/json",
		Handler:     graphResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://graph/ready",
		Name:        "Ready specs",
		Description: "Specs with no unmet dependencies",
		MimeType:    "application/json",
		Handler:     readyResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://findings",
		Name:        "Findings",
		Description: "Analytical findings, filterable by pass_type query param",
		MimeType:    "application/json",
		Handler:     findingsResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://spec/{slug}/changes",
		Name:        "Changes",
		Description: "Version history for a spec",
		MimeType:    "application/json",
		IsTemplate:  true,
		Handler:     changesResourceHandler(c),
	})
}
```

- [ ] **Step 3: Run resource tests**

Run: `go test ./internal/mcp/ -run 'TestSpec.*Resource|TestConstitution.*Resource' -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/resources.go internal/mcp/resources_test.go
git commit -s -m "feat(mcp): add MCP resources — specs, decisions, constitution, graph, findings, changes"
```

---

### Task 9: Prompts

**Files:**
- Create: `internal/mcp/prompts.go`
- Create: `internal/mcp/prompts_test.go`

- [ ] **Step 1: Write prompt tests**

```go
// internal/mcp/prompts_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

func TestSparkPrompt(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(req *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			return &specv1.GetPromptsResponse{
				Prompt: "You are a spec architect. Spark a new idea about: test-topic",
			}, nil
		},
	}}
	h := sparkPromptHandler(c)

	result, err := h(context.Background(), map[string]string{
		"topic":   "test-topic",
		"context": "microservices",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Messages)
	require.Contains(t, result.Messages[0].Content, "test-topic")
}

func TestShapePrompt(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(req *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			return &specv1.GetPromptsResponse{
				Prompt: "Shape the spec: auth",
			}, nil
		},
	}}
	h := shapePromptHandler(c)

	result, err := h(context.Background(), map[string]string{"spec_slug": "auth"})
	require.NoError(t, err)
	require.NotEmpty(t, result.Messages)
}
```

- [ ] **Step 2: Write prompt handlers**

```go
// internal/mcp/prompts.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func stagePromptHandler(c *Client, stage string) PromptHandler {
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		resp, err := c.Authoring.GetPrompts(ctx, connect.NewRequest(&specv1.GetPromptsRequest{
			Stage:    stage,
			SpecSlug: args["spec_slug"],
		}))
		if err != nil {
			return nil, err
		}
		return &PromptResult{
			Description: stage + " authoring prompt",
			Messages: []PromptMessage{{
				Role:    "user",
				Content: resp.Msg.GetPrompt(),
			}},
		}, nil
	}
}

func sparkPromptHandler(c *Client) PromptHandler {
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		resp, err := c.Authoring.GetPrompts(ctx, connect.NewRequest(&specv1.GetPromptsRequest{
			Stage: "spark",
		}))
		if err != nil {
			return nil, err
		}
		prompt := resp.Msg.GetPrompt()
		if topic := args["topic"]; topic != "" {
			prompt = prompt + "\n\nTopic: " + topic
		}
		if ctx := args["context"]; ctx != "" {
			prompt = prompt + "\nContext: " + ctx
		}
		return &PromptResult{
			Description: "Spark a new spec idea",
			Messages: []PromptMessage{{
				Role:    "user",
				Content: prompt,
			}},
		}, nil
	}
}

func shapePromptHandler(c *Client) PromptHandler {
	return stagePromptHandler(c, "shape")
}

func specifyPromptHandler(c *Client) PromptHandler {
	return stagePromptHandler(c, "specify")
}

func decomposePromptHandler(c *Client) PromptHandler {
	return stagePromptHandler(c, "decompose")
}

func analyticalPromptHandler(c *Client, passType string) PromptHandler {
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		resp, err := c.AnalyticalPass.RunAnalyticalPass(ctx, connect.NewRequest(&specv1.RunAnalyticalPassRequest{
			SpecSlug: args["spec_slug"],
			PassType: passType,
		}))
		if err != nil {
			return nil, err
		}
		return &PromptResult{
			Description: passType + " analytical pass",
			Messages: []PromptMessage{{
				Role:    "user",
				Content: resp.Msg.GetPrompt(),
			}},
		}, nil
	}
}

// RegisterPrompts adds all MCP prompt definitions to the registry.
func RegisterPrompts(r *Registry, c *Client) {
	r.AddPrompt(PromptDef{
		Name:        "spark",
		Description: "Generate an initial spec idea from a topic",
		Arguments: []PromptArgument{
			{Name: "topic", Description: "Topic to explore", Required: true},
			{Name: "context", Description: "Additional context"},
		},
		Handler: sparkPromptHandler(c),
	})
	r.AddPrompt(PromptDef{
		Name:        "shape",
		Description: "Refine a sparked spec into a structured shape",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Spec to shape", Required: true},
		},
		Handler: shapePromptHandler(c),
	})
	r.AddPrompt(PromptDef{
		Name:        "specify",
		Description: "Add full specification detail with acceptance criteria",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Spec to specify", Required: true},
		},
		Handler: specifyPromptHandler(c),
	})
	r.AddPrompt(PromptDef{
		Name:        "decompose",
		Description: "Break a spec into implementable work slices",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Spec to decompose", Required: true},
		},
		Handler: decomposePromptHandler(c),
	})
	r.AddPrompt(PromptDef{
		Name:        "constitution_check",
		Description: "Check a spec against the project constitution",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Spec to check", Required: true},
		},
		Handler: analyticalPromptHandler(c, "constitution-check"),
	})
	r.AddPrompt(PromptDef{
		Name:        "dependency_review",
		Description: "Review dependency health for a spec",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Spec to review", Required: true},
		},
		Handler: analyticalPromptHandler(c, "dependency-review"),
	})
}
```

- [ ] **Step 3: Run prompt tests**

Run: `go test ./internal/mcp/ -run 'TestSparkPrompt|TestShapePrompt' -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/prompts.go internal/mcp/prompts_test.go
git commit -s -m "feat(mcp): add MCP prompts — spark, shape, specify, decompose, constitution_check, dependency_review"
```

---

### Task 10: SDK Integration — convert, tiers, server

**Files:**
- Create: `internal/mcp/convert.go`
- Create: `internal/mcp/convert_test.go`
- Create: `internal/mcp/tiers.go`
- Create: `internal/mcp/tiers_test.go`
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/server_test.go`

This is the task where `mcp-go` enters the codebase. All SDK imports are confined to these files.

- [ ] **Step 1: Add mcp-go dependency**

Run: `cd /Volumes/Code/github.com/.worktrees/specgraph/mcp-server && go get github.com/mark3labs/mcp-go@latest`

Verify: `go list -m github.com/mark3labs/mcp-go` should print the resolved version.

- [ ] **Step 2: Verify mcp-go API**

Before writing code, verify the SDK's tool, resource, and prompt types. Create a scratch test:

```bash
cd /Volumes/Code/github.com/.worktrees/specgraph/mcp-server
go doc github.com/mark3labs/mcp-go/mcp Tool
go doc github.com/mark3labs/mcp-go/mcp CallToolRequest
go doc github.com/mark3labs/mcp-go/mcp Resource
go doc github.com/mark3labs/mcp-go/server MCPServer
```

Check for per-session tool support — look for `SessionTools`, `WithToolFilter`, or similar patterns in the server package. Adjust `convert.go` and `tiers.go` accordingly.

- [ ] **Step 3: Write convert tests**

```go
// internal/mcp/convert_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestToSDKTool(t *testing.T) {
	def := ToolDef{
		Name:        "spec",
		Description: "Manage specs",
		Schema: objectSchema(
			props{
				"action": stringProp("Operation", "get", "list"),
			},
			"action",
		),
	}
	tool := toSDKTool(def)
	require.Equal(t, "spec", tool.Name)
	require.Equal(t, "Manage specs", tool.Description)
}

func TestToSDKResult_Success(t *testing.T) {
	r := textResult("hello")
	sdkResult := toSDKResult(r)
	require.False(t, sdkResult.IsError)
	require.Len(t, sdkResult.Content, 1)
}

func TestToSDKResult_Error(t *testing.T) {
	r := errResult("broken")
	sdkResult := toSDKResult(r)
	require.True(t, sdkResult.IsError)
}

func TestFromSDKParams(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"action": "get",
		"slug":   "auth",
	}
	params := fromSDKParams(req)
	require.Equal(t, "get", params["action"])
	require.Equal(t, "auth", params["slug"])
}
```

- [ ] **Step 4: Write convert.go**

```go
// internal/mcp/convert.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// --- Tool conversion ---

// toSDKTool converts our ToolDef to an mcp-go Tool.
func toSDKTool(def ToolDef) mcp.Tool {
	return mcp.Tool{
		Name:        def.Name,
		Description: def.Description,
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: toSDKProperties(def.Schema),
			Required:   toSDKRequired(def.Schema),
		},
	}
}

func toSDKProperties(schema map[string]any) map[string]interface{} {
	p, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}
	// Return as-is — JSON Schema properties are already the right shape
	out := make(map[string]interface{}, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}

func toSDKRequired(schema map[string]any) []string {
	r, ok := schema["required"].([]string)
	if !ok {
		return nil
	}
	return r
}

// fromSDKParams extracts the arguments map from an mcp-go CallToolRequest.
func fromSDKParams(req mcp.CallToolRequest) map[string]any {
	if req.Params.Arguments == nil {
		return make(map[string]any)
	}
	return req.Params.Arguments
}

// toSDKResult converts our ToolResult to an mcp-go CallToolResult.
func toSDKResult(r *ToolResult) *mcp.CallToolResult {
	result := &mcp.CallToolResult{
		IsError: r.IsError,
	}
	for _, c := range r.Content {
		result.Content = append(result.Content, mcp.TextContent{
			Type: "text",
			Text: c.Text,
		})
	}
	return result
}

// --- Resource conversion ---

// toSDKResource converts our ResourceDef to an mcp-go Resource or ResourceTemplate.
func toSDKResource(def ResourceDef) mcp.Resource {
	return mcp.Resource{
		URI:         def.URI,
		Name:        def.Name,
		Description: def.Description,
		MIMEType:    def.MimeType,
	}
}

func toSDKResourceTemplate(def ResourceDef) mcp.ResourceTemplate {
	return mcp.ResourceTemplate{
		URITemplate: def.URI,
		Name:        def.Name,
		Description: def.Description,
		MIMEType:    def.MimeType,
	}
}

func toSDKResourceContents(rcs []ResourceContent) []mcp.ResourceContents {
	out := make([]mcp.ResourceContents, len(rcs))
	for i, rc := range rcs {
		out[i] = mcp.TextResourceContents{
			URI:      rc.URI,
			MIMEType: rc.MimeType,
			Text:     rc.Text,
		}
	}
	return out
}

// --- Prompt conversion ---

// toSDKPrompt converts our PromptDef to an mcp-go Prompt.
func toSDKPrompt(def PromptDef) mcp.Prompt {
	args := make([]mcp.PromptArgument, len(def.Arguments))
	for i, a := range def.Arguments {
		args[i] = mcp.PromptArgument{
			Name:        a.Name,
			Description: a.Description,
			Required:    a.Required,
		}
	}
	return mcp.Prompt{
		Name:        def.Name,
		Description: def.Description,
		Arguments:   args,
	}
}

func toSDKPromptResult(r *PromptResult) *mcp.GetPromptResult {
	msgs := make([]mcp.PromptMessage, len(r.Messages))
	for i, m := range r.Messages {
		msgs[i] = mcp.PromptMessage{
			Role: mcp.Role(m.Role),
			Content: mcp.TextContent{
				Type: "text",
				Text: m.Content,
			},
		}
	}
	return &mcp.GetPromptResult{
		Description: r.Description,
		Messages:    msgs,
	}
}
```

- [ ] **Step 5: Write tiers tests**

```go
// internal/mcp/tiers_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestTierFromClientInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     mcp.Implementation
		expected Tier
	}{
		{
			name:     "no metadata defaults to core",
			info:     mcp.Implementation{Name: "test"},
			expected: TierCore,
		},
		{
			name: "authoring role",
			info: mcp.Implementation{
				Name:    "claude-code",
				Version: "1.0",
			},
			expected: TierAuthoring,
		},
		{
			name: "execution role",
			info: mcp.Implementation{
				Name:    "polecat",
				Version: "1.0",
			},
			expected: TierExecution,
		},
	}

	// Note: actual role extraction depends on how mcp-go passes client metadata.
	// Update the Implementation fields and TierFromClientInfo logic after verifying
	// the SDK's ClientInfo/Implementation struct in Step 2.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := TierFromClientInfo(tt.info)
			require.GreaterOrEqual(t, int(tier), int(TierCore))
		})
	}
}
```

- [ ] **Step 6: Write tiers.go**

```go
// internal/mcp/tiers.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// TierFromClientInfo determines the tool tier from MCP client metadata.
// Looks for "specgraph.role" in the client info. Returns TierCore if absent.
//
// NOTE: The exact field to inspect depends on how mcp-go exposes client metadata.
// The Implementation struct may carry metadata directly or via an extensions map.
// Adjust after verifying the SDK API (Task 10, Step 2).
func TierFromClientInfo(info mcp.Implementation) Tier {
	// mcp-go may expose metadata via info.Extensions or a similar field.
	// For now, inspect the client name as a fallback heuristic:
	switch info.Name {
	case "polecat", "gastown":
		return TierExecution
	case "claude-code", "cursor", "windsurf":
		return TierAuthoring
	default:
		return TierCore
	}
}
```

- [ ] **Step 7: Write server.go**

```go
// internal/mcp/server.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"io"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Version is set at build time.
var Version = "dev"

// Server wraps the mcp-go MCPServer with SpecGraph's tiered tool registry.
type Server struct {
	registry *Registry
	servers  map[Tier]*server.MCPServer
}

// NewServer creates an MCP server backed by the given ConnectRPC client.
// It registers all tools, resources, and prompts.
func NewServer(client *Client) *Server {
	reg := NewRegistry()

	// Register all tools
	RegisterSpecTools(reg, client)
	RegisterGraphTools(reg, client)
	RegisterCoreTools(reg, client)
	RegisterAuthoringTools(reg, client)
	RegisterLifecycleTools(reg, client)
	RegisterExecutionTools(reg, client)

	// Register resources and prompts
	RegisterResources(reg, client)
	RegisterPrompts(reg, client)

	// Build one mcp-go server per tier
	servers := make(map[Tier]*server.MCPServer, 3)
	for _, tier := range []Tier{TierCore, TierAuthoring, TierExecution} {
		s := server.NewMCPServer(
			"specgraph",
			Version,
			server.WithToolCapabilities(true),
			server.WithResourceCapabilities(true, true),
			server.WithPromptCapabilities(true),
		)

		// Add tools for this tier
		for _, def := range reg.ToolsForTier(tier) {
			d := def // capture
			sdkTool := toSDKTool(d)
			s.AddTool(sdkTool, wrapToolHandler(d.Handler))
		}

		// Add all resources (resources are not tiered)
		for _, res := range reg.Resources() {
			r := res
			if r.IsTemplate {
				s.AddResourceTemplate(toSDKResourceTemplate(r), wrapResourceHandler(r.Handler))
			} else {
				s.AddResource(toSDKResource(r), wrapResourceHandler(r.Handler))
			}
		}

		// Add all prompts (prompts are not tiered)
		for _, p := range reg.Prompts() {
			pr := p
			s.AddPrompt(toSDKPrompt(pr), wrapPromptHandler(pr.Handler))
		}

		servers[tier] = s
	}

	return &Server{registry: reg, servers: servers}
}

// ForTier returns the mcp-go server instance for the given tier.
func (s *Server) ForTier(tier Tier) *server.MCPServer {
	srv, ok := s.servers[tier]
	if !ok {
		return s.servers[TierCore]
	}
	return srv
}

// ServeStdio runs the MCP server over stdin/stdout for the given tier.
func (s *Server) ServeStdio(ctx context.Context, tier Tier, stdin io.Reader, stdout io.Writer) error {
	srv := s.ForTier(tier)
	stdio := server.NewStdioServer(srv)
	return stdio.Listen(ctx, stdin, stdout)
}

// HTTPHandler returns an http.Handler for the MCP streamable HTTP transport at the given tier.
func (s *Server) HTTPHandler(tier Tier) *server.StreamableHTTPServer {
	srv := s.ForTier(tier)
	return server.NewStreamableHTTPServer(srv)
}

// --- Handler wrappers ---

func wrapToolHandler(h ToolHandler) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := fromSDKParams(req)
		result, err := h(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("tool handler: %w", err)
		}
		return toSDKResult(result), nil
	}
}

func wrapResourceHandler(h ResourceHandler) server.ResourceHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		contents, err := h(ctx, req.Params.URI)
		if err != nil {
			return nil, fmt.Errorf("resource handler: %w", err)
		}
		return toSDKResourceContents(contents), nil
	}
}

func wrapPromptHandler(h PromptHandler) server.PromptHandlerFunc {
	return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		result, err := h(ctx, req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("prompt handler: %w", err)
		}
		return toSDKPromptResult(result), nil
	}
}
```

- [ ] **Step 8: Write server test**

```go
// internal/mcp/server_test.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewServer_TierToolCounts(t *testing.T) {
	// Create a server with a nil client — we're only testing registration, not calls
	// This will panic if any handler is invoked, which is fine for this test.
	c := &Client{} // all nil — only testing tool registration counts

	s := NewServer(c)

	coreSrv := s.ForTier(TierCore)
	require.NotNil(t, coreSrv)

	authoringSrv := s.ForTier(TierAuthoring)
	require.NotNil(t, authoringSrv)

	executionSrv := s.ForTier(TierExecution)
	require.NotNil(t, executionSrv)

	// Verify different server instances per tier
	require.NotSame(t, coreSrv, authoringSrv)
	require.NotSame(t, authoringSrv, executionSrv)
}

func TestNewServer_FallbackToCore(t *testing.T) {
	c := &Client{}
	s := NewServer(c)

	// Unknown tier falls back to core
	srv := s.ForTier(Tier(99))
	require.NotNil(t, srv)
	require.Same(t, s.ForTier(TierCore), srv)
}
```

- [ ] **Step 9: Run all SDK integration tests**

Run: `go test ./internal/mcp/ -run 'TestToSDK|TestFromSDK|TestTier|TestNewServer' -v`
Expected: All PASS

- [ ] **Step 10: Run full package tests**

Run: `go test ./internal/mcp/ -v`
Expected: All tests PASS

- [ ] **Step 11: Commit**

```bash
git add go.mod go.sum internal/mcp/convert.go internal/mcp/convert_test.go internal/mcp/tiers.go internal/mcp/tiers_test.go internal/mcp/server.go internal/mcp/server_test.go
git commit -s -m "feat(mcp): add SDK integration — convert, tiers, server wiring with mcp-go"
```

---

### Task 11: `specgraph mcp` CLI Command

**Files:**
- Create: `cmd/specgraph/mcp.go`

- [ ] **Step 1: Write the CLI command**

```go
// cmd/specgraph/mcp.go

// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	mcppkg "github.com/specgraph/specgraph/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server over stdio (for Claude Code, Cursor, etc.)",
	Long: `Start a Model Context Protocol server that communicates over stdin/stdout.
This lightweight process translates MCP tool calls into ConnectRPC RPCs
against a running specgraph serve instance.

Configure in Claude Code's MCP settings:
  {
    "mcpServers": {
      "specgraph": {
        "command": "specgraph",
        "args": ["mcp", "--tier", "authoring"]
      }
    }
  }`,
	RunE: runMCP,
}

func init() {
	mcpCmd.Flags().String("tier", "core", "Tool tier: core, authoring, or execution")
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, _ []string) error {
	tierStr, _ := cmd.Flags().GetString("tier")
	tier := mcppkg.ParseTier(tierStr)

	// Resolve server connection using existing config/auth
	baseURL, project, err := resolveBaseURL()
	if err != nil {
		return fmt.Errorf("resolve server: %w", err)
	}

	httpClient := newHTTPClient(project)
	client := mcppkg.NewClient(httpClient, baseURL)
	srv := mcppkg.NewServer(client)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	return srv.ServeStdio(ctx, tier, os.Stdin, os.Stdout)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: Success

- [ ] **Step 3: Verify help text**

Run: `go run ./cmd/specgraph/ mcp --help`
Expected: Shows usage with --tier flag

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/mcp.go
git commit -s -m "feat(mcp): add 'specgraph mcp' CLI command for stdio transport"
```

---

### Task 12: MCP HTTP Endpoint in `specgraph serve`

**Files:**
- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Read current serve.go to identify insertion points**

Read `cmd/specgraph/serve.go` to find where the HTTP mux is created, where middleware is applied, and where the server starts. Identify exact line numbers.

- [ ] **Step 2: Add MCP HTTP endpoint**

Add these changes to `cmd/specgraph/serve.go`:

1. Import the mcp package:
```go
import (
	// ... existing imports ...
	mcppkg "github.com/specgraph/specgraph/internal/mcp"
)
```

2. After the ConnectRPC mux setup and service registration, add the MCP endpoint:
```go
// Create MCP server with a loopback ConnectRPC client
mcpHTTPClient := newHTTPClient(projectSlug)
mcpClient := mcppkg.NewClient(mcpHTTPClient, "http://"+addr)
mcpServer := mcppkg.NewServer(mcpClient)

// Mount MCP streamable HTTP endpoint.
// Tier determined by X-Specgraph-MCP-Tier header, defaults to core.
mux.Handle("/mcp/", http.StripPrefix("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	tierStr := r.Header.Get("X-Specgraph-MCP-Tier")
	if tierStr == "" {
		tierStr = r.URL.Query().Get("tier")
	}
	tier := mcppkg.ParseTier(tierStr)
	mcpHandler := mcpServer.HTTPHandler(tier)
	mcpHandler.ServeHTTP(w, r)
})))
```

3. Log the MCP endpoint availability alongside the ConnectRPC log:
```go
slog.Info("MCP endpoint available", "path", "/mcp/")
```

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/serve.go
git commit -s -m "feat(mcp): embed MCP HTTP endpoint in specgraph serve"
```

---

### Task 13: Quality Gates

- [ ] **Step 1: Add license headers**

Run: `task license:add`

- [ ] **Step 2: Format**

Run: `task fmt`

- [ ] **Step 3: Run full quality check**

Run: `task check`

Fix any issues that arise. Common ones:
- `revive`: package comment needed for `internal/mcp/` — the `types.go` file has it
- `govet -shadow`: variable shadowing in switch cases — rename if needed
- `wrapcheck`: exported functions in `server.go` may need error wrapping
- `gosec`: no expected issues in this code
- Unused imports from `json`, `encoding/json` — remove if helpers don't use them

- [ ] **Step 4: Run unit tests**

Run: `task test`
Expected: All PASS including new `internal/mcp/` tests

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -s -m "chore(mcp): fix lint and formatting issues"
```

---

## Proto Field Verification Notes

Some proto request/response field names in the tool handlers are inferred from naming conventions and existing test code. When implementing, verify exact field names against the generated types in `gen/specgraph/v1/`:

- `UpdateSpecRequest` — verify available fields (may use field mask pattern)
- `UpdateDecisionRequest` — verify available fields
- `ShapeRequest.Output`, `SpecifyRequest.Output`, `DecomposeRequest.Output` — verify field name for stage output JSON
- `RecordConversationRequest.Messages` — verify field name and type
- `GetConstitutionRequest.Layer` — verify layer filter field name
- `DriftAcknowledgeRequest.All` — verify boolean flag name
- `ReportProgressRequest.Percent` — verify field name and type

If any field doesn't exist or has a different name, adjust the handler code to match the actual proto definition. The handler pattern (extract params → build request → call client → return result) stays the same.

## SDK API Verification Notes

The `convert.go`, `tiers.go`, and `server.go` files reference `mcp-go` types that may differ slightly from the actual v0.47+ API:

- `mcp.ToolInputSchema` — verify struct fields match
- `server.ToolHandlerFunc`, `server.ResourceHandlerFunc`, `server.PromptHandlerFunc` — verify exact type signatures
- `server.NewStreamableHTTPServer` — verify constructor and ServeHTTP method
- `server.NewStdioServer` and `Listen` method — verify signature
- `server.AddResourceTemplate` — verify this method exists (may be `AddResource` with template flag)
- `mcp.ReadResourceRequest.Params.URI` — verify field path
- `mcp.GetPromptRequest.Params.Arguments` — verify type is `map[string]string`

Adjust the convert layer to match the actual SDK API. The rest of the code (handlers, registry, types) remains unchanged.
