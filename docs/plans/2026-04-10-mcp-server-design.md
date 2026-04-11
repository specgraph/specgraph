# MCP Server Design

> **Date**: 2026-04-10
> **Status**: Draft
> **Phase**: 3 (Coordination & Export)

## Overview

Add a full-featured MCP (Model Context Protocol) server to SpecGraph, exposing specs, graph queries, authoring workflows, and execution primitives to AI agents via the MCP standard. The MCP layer is a thin translation adapter — it holds no business logic and forwards all operations to existing ConnectRPC handlers.

### Goals

- Enable Claude Code, Cursor, and other MCP-compatible clients to interact with SpecGraph natively
- Support Gastown polecats as first-class MCP consumers for execution workflows
- Expose tools, resources, and prompts covering the full SpecGraph surface
- Tier tool visibility by client role (core, authoring, execution)
- Isolate the MCP SDK dependency so it can be swapped with minimal blast radius

### Non-Goals

- Replacing the CLI or ConnectRPC API
- Embedding storage or handler logic in the MCP layer
- Building a custom MCP protocol implementation

## SDK Choice

**`github.com/mark3labs/mcp-go`** (MIT license, compatible with Apache-2.0).

Chosen over `modelcontextprotocol/go-sdk` for server-side ergonomics: per-session tools, tool filtering, and middleware — features that directly support tiered tool exposure. The SDK is isolated to 3 files, making future replacement straightforward.

### Isolation Strategy

Only three files import `mcp-go`:

| File | Imports | Purpose |
|------|---------|---------|
| `server.go` | SDK server, transports | Wire MCP server instance |
| `convert.go` | SDK tool/resource/prompt types | Map our types ↔ SDK types |
| `tiers.go` | SDK per-session tools interface | Hook into session filtering |

All tool handlers, resource handlers, prompt handlers, and the ConnectRPC client abstraction use SpecGraph-owned types exclusively.

**Swap cost**: Rewrite `server.go`, `convert.go`, `tiers.go`. Zero changes to handlers or anything outside `internal/mcp/`.

## Architecture

### Transport Modes

#### Stdio (`specgraph mcp`)

```text
Claude Code / Cursor
    ↓ stdin/stdout (JSON-RPC)
specgraph mcp (lightweight process)
    ↓ ConnectRPC over HTTP
specgraph serve (full server)
```

- Reads server address + API key from existing config or flags
- Creates a ConnectRPC client, wraps it in the MCP adapter, wires to stdio transport
- No storage, no handlers — translation only

#### HTTP (embedded in `specgraph serve`)

```text
Gastown polecat / remote client
    ↓ HTTP (streamable HTTP / SSE)
specgraph serve
    ├── /connect/*  (existing ConnectRPC)
    └── /mcp/*      (MCP HTTP endpoint)
            ↓ in-process call
        ConnectRPC handlers
```

- MCP HTTP endpoint mounts alongside ConnectRPC on the same server
- Same auth interceptor — MCP requests carry the same API key / OIDC token
- Calls ConnectRPC handlers in-process via the same client interface as stdio mode

### ConnectRPC Client Abstraction

Both transport modes use the same interface:

```go
// internal/mcp/client.go
type SpecGraphClient interface {
    GetSpec(ctx context.Context, slug string) (*specv1.GetSpecResponse, error)
    ListSpecs(ctx context.Context, ...) (*specv1.ListSpecsResponse, error)
    // ... one method per RPC needed by MCP handlers
}
```

Two implementations:

- **`RemoteClient`** — makes ConnectRPC HTTP calls (used by `specgraph mcp` stdio mode)
- **`LocalClient`** — calls through ConnectRPC generated client stubs pointed at `localhost` (used by `specgraph serve` HTTP mode). This keeps the code path identical to stdio mode (same interceptors, same auth) while avoiding a network hop via loopback optimization.

## Package Structure

```text
internal/mcp/
├── server.go          # MCP server setup, transport wiring (imports SDK)
├── client.go          # SpecGraphClient interface + RemoteClient/LocalClient
├── convert.go         # Our types ↔ SDK types (imports SDK)
├── tiers.go           # Tier definitions, session→tier mapping (imports SDK)
├── tools.go           # Tool registry, ToolDef type, ToolHandler type
├── tools_spec.go      # Spec CRUD tool handlers
├── tools_decision.go  # Decision tool handlers
├── tools_graph.go     # Graph query tool handlers
├── tools_authoring.go # Authoring workflow tool handlers
├── tools_execution.go # Execution/slice tool handlers
├── tools_lifecycle.go # Drift, lint, sync, export tool handlers
├── resources.go       # MCP resource definitions + handlers
└── prompts.go         # MCP prompt definitions + handlers

cmd/specgraph/
├── mcp.go             # `specgraph mcp` command (stdio transport)
└── serve.go           # (modified) also starts MCP HTTP endpoint
```

## Tool Tiers

### Tier Model

Three tiers, each inclusive of the one below:

| Tier | Target Client | Tool Count | Focus |
|------|---------------|------------|-------|
| **core** | General MCP clients | 7 | Read-heavy: browse specs, query graph, read constitution, view findings |
| **authoring** | Claude Code, Cursor | +7 (14) | Core + authoring funnel, decisions, drift, analytical passes |
| **execution** | Gastown polecats | +6 (20) | Authoring + claims, slices, progress reporting, bundles |

### Capability Negotiation

During MCP `initialize`, the client passes metadata to select a tier:

```json
{
  "clientInfo": {
    "name": "claude-code",
    "version": "1.0",
    "metadata": { "specgraph.role": "authoring" }
  }
}
```

Mapping:

- `"specgraph.role": "execution"` → execution tier
- `"specgraph.role": "authoring"` → authoring tier
- Anything else (or absent) → core tier

Implemented via `mcp-go`'s per-session tools feature — the registry filters tools by the session's negotiated tier before returning them via `tools/list`.

### Tool Registry Type

```go
type Tier int

const (
    TierCore      Tier = iota
    TierAuthoring
    TierExecution
)

type ToolDef struct {
    Name        string
    Description string
    Tier        Tier
    Schema      map[string]any  // JSON Schema for parameters
    Handler     ToolHandler
}

type ToolHandler func(ctx context.Context, params map[string]any) (*ToolResult, error)

type ToolResult struct {
    Content []Content
    IsError bool
}

type Content struct {
    Type string // "text", "image", "resource"
    Text string
}
```

## Tools

### Core Tier

Resource-oriented CRUD and graph queries.

| Tool | Actions | Maps to RPCs |
|------|---------|--------------|
| `spec` | `get`, `list`, `create`, `update`, `changes`, `compare` | GetSpec, ListSpecs, CreateSpec, UpdateSpec, ListChanges, CompareVersions |
| `decision` | `get`, `list`, `create`, `update` | GetDecision, ListDecisions, CreateDecision, UpdateDecision |
| `edge` | `add`, `remove`, `list` | AddEdge, RemoveEdge, ListEdges |
| `graph_query` | `dependencies`, `transitive_deps`, `impact`, `ready`, `critical_path`, `full` | GetDependencies, GetTransitiveDeps, GetImpact, GetReady, GetCriticalPath, GetFullGraph |
| `constitution` | `get`, `update` | GetConstitution, UpdateConstitution |
| `findings` | `list` | ListFindings |
| `health` | — | Health |

### Authoring Tier (adds to core)

Workflow-oriented tools for multi-step processes.

| Tool | Actions | Maps to RPCs |
|------|---------|--------------|
| `author` | `spark`, `shape`, `specify`, `decompose`, `approve`, `amend`, `supersede` | Spark, Shape, Specify, Decompose, Approve, Amend, Supersede |
| `conversation` | `record`, `list` | RecordConversation, ListConversations |
| `analytical_pass` | `run`, `store` | RunAnalyticalPass, StoreFindings |
| `drift` | `check`, `acknowledge` | CheckDrift, AcknowledgeDrift |
| `lint` | — | Lint |
| `sync` | `beads`, `github`, `status` | SyncBeads, SyncGitHub, GetSyncStatus |
| `export` | `export`, `import`, `verify` | ExportProject, ImportProject, VerifyExport |

### Execution Tier (adds to authoring)

Precise tools for Gastown polecats and execution agents.

| Tool | Actions | Maps to RPCs |
|------|---------|--------------|
| `claim` | `claim`, `unclaim`, `heartbeat` | ClaimSpec, UnclaimSpec, Heartbeat |
| `slice` | `list`, `get`, `claim`, `complete` | ListSlices, GetSlice, ClaimSlice, CompleteSlice |
| `bundle` | — | GenerateBundle |
| `prime` | — | GetPrime |
| `report` | `progress`, `blocker`, `completion` | ReportProgress, ReportBlocker, ReportCompletion |
| `execution_events` | — | GetExecutionEvents |

## Resources

URI-based read-only context for client context windows.

| Resource | URI Pattern | Maps to |
|----------|-------------|---------|
| Spec | `specgraph://spec/{slug}` | GetSpec |
| Spec list | `specgraph://specs` | ListSpecs |
| Decision | `specgraph://decision/{slug}` | GetDecision |
| Constitution (merged) | `specgraph://constitution` | GetConstitution |
| Constitution layer | `specgraph://constitution/{layer}` | GetConstitution (filtered) |
| Graph | `specgraph://graph` | GetFullGraph |
| Ready specs | `specgraph://graph/ready` | GetReady |
| Findings | `specgraph://findings` | ListFindings |
| Changes | `specgraph://spec/{slug}/changes` | ListChanges |

Constitution layers: `user`, `org`, `project`, `domain`.

### Subscriptions

Initial implementation is poll-based: the MCP server polls ConnectRPC for changes on a configurable interval (default 5s) and emits notifications to subscribed clients. Event-driven notifications can be added later without changing the MCP API surface.

## Prompts

Reusable templates that guide agents through workflows.

| Prompt | Arguments | Maps to |
|--------|-----------|---------|
| `spark` | `topic`, `context` | GetPrompts(spark) |
| `shape` | `spec_slug` | GetPrompts(shape) |
| `specify` | `spec_slug` | GetPrompts(specify) |
| `decompose` | `spec_slug` | GetPrompts(decompose) |
| `constitution_check` | `spec_slug` | Analytical pass template |
| `dependency_review` | `spec_slug` | Analytical pass template |

## Error Handling

### ConnectRPC → MCP Error Mapping

| ConnectRPC Code | MCP Behavior |
|-----------------|--------------|
| `CodeNotFound` | Tool result with `isError: true`, descriptive message |
| `CodeAborted` | Tool result with `isError: true`, "concurrent modification, retry" |
| `CodeInvalidArgument` | Tool result with `isError: true`, validation details |
| `CodeUnauthenticated` | MCP protocol-level error (not a tool result) |
| `CodeInternal` | Tool result with `isError: true`, sanitized message |

**Principle**: Auth failures are protocol errors (client can't proceed). Everything else is a tool result (agent can reason about it and retry).

### Connection Lifecycle

- **Stdio**: process exits when stdin closes. Clean shutdown.
- **HTTP**: session timeout after idle period. Subscriptions cleaned up on disconnect.
- **Server unreachable** (stdio mode): returns MCP error on every tool call — "cannot reach specgraph server at {address}".

## Auth

Reuses existing ConnectRPC auth entirely:

- **Stdio mode**: `specgraph mcp` passes the user's configured API key with every ConnectRPC call
- **HTTP mode**: same auth interceptor as ConnectRPC (local API key + OIDC)
- No separate MCP auth layer
