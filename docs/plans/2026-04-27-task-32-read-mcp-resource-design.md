# Task 32 Design Adjustment: `specgraph read-mcp-resource`

Adjustment for Task 32 of `docs/plans/2026-04-20-multi-platform-plugin-plan.md`. The original plan was written before PR #923 landed and is materially wrong about transport, helper functions, and command shape. This document captures the corrected design agreed during brainstorming on 2026-04-27.

Tracked under `spgr-bncv` (Phase B Slice 5).

## Context: what changed since the original plan

The original plan (line 2953) prescribed a `specgraph mcp read-resource` subcommand built on a `newMCPStdioClient` helper attached to a parent `mcpCmd`. Three of those assumptions are no longer true:

1. **PR #923 (`feat(mcp)!: drop stdio transport, HTTP-only`)** removed the entire stdio MCP transport from the server. The MCP endpoint is now HTTP-only, mounted at `/mcp/` with `auth.RequireAuth` middleware (`cmd/specgraph/serve.go:258`).
2. **PR #923 also removed the `specgraph mcp` parent command.** No `cmd/specgraph/mcp.go` exists today; the only mcp-named CLI file is `serve_mcp_test.go` (HTTP endpoint auth tests).
3. **`newMCPStdioClient` was never reintroduced.** A grep across `cmd/` and `internal/` finds no stdio MCP client constructor of any name.

The plan code as written would not compile, and even if it did, would not reach a server.

What is true today, and reusable: the CLI already has `resolveBaseURL()`, `resolveAPIKey()`, and `newAuthenticatedHTTPClient(project)` (`cmd/specgraph/client.go:52-119`). They feed every existing ConnectRPC command. The new MCP HTTP client wraps the same outputs.

## Decisions

### D1. Top-level command, not a parent

The new subcommand is `specgraph read-mcp-resource <uri>`, top-level. No reintroduction of a `specgraph mcp` parent.

The `mcp` parent name is ambiguous — it reads as "this command **is** MCP" or "this command **uses** MCP," and the latter is what we mean. The verb-form `read-mcp-resource` reads as "read [MCP resource]" — adjective-modified noun — and sets up symmetric siblings (`call-mcp-tool`, `get-mcp-prompt`) if they ever exist.

### D2. HTTP transport via mark3labs/mcp-go

The CLI constructs an `mcp-go` HTTP/streamable client pointed at `<baseURL>/mcp/`, where `baseURL` comes from the existing `resolveBaseURL()` resolution chain (project `.specgraph.yaml` → global config → fallback). The HTTP client is `newAuthenticatedHTTPClient(project)`, which already injects the bearer token resolved from `SPECGRAPH_API_KEY` env var or the credentials file.

No new client construction logic. No stdio transport. No env var for the server URL (see D4).

### D3. Output is plain text on stdout

`ReadResource` returns `[]ResourceContents`. The command iterates and prints each `TextContents` body to stdout, terminated with a final newline. Binary content (`BlobContents`) prints a one-line stderr warning and is skipped. Non-zero exit on transport failure or read error; the session-start hook script soft-fails (`|| { echo …; exit 0; }`) so a missing server never blocks session start.

### D4. No new env vars, no new flags

Considered and explicitly rejected during brainstorming:

- **`--server` global flag.** Would solve a real ad-hoc-override usability gap but bundles unrelated work into Slice 5 and adds flag bloat.
- **`--api-key` global flag.** API keys on argv leak into `ps -ef`, shell history, log files. `SPECGRAPH_API_KEY` env var already serves this need.
- **`SPECGRAPH_SERVER` env var.** A hand-rolled env-var rung in `resolveBaseURL` would lock in a non-koanf-natural name as public interface, making the eventual config refactor harder. The current scattered pattern (`SPECGRAPH_API_KEY` in `client.go:90`, `SPECGRAPH_PG_URL` in `serve.go:74`) is exactly what we don't want to extend.

The deferred work — adopting koanf with the env provider for declarative env→config mapping — is filed as `spgr-5kd5`. Slice 5's PR description references it.

### D5. Tests use the existing `cfgFile` swap pattern

Tests follow the canonical pattern from `lifecycle_test.go` and `test_helpers_test.go`:

```go
srv := startTestMCPServer(t)
cfgPath := filepath.Join(t.TempDir(), "config.yaml")
os.WriteFile(cfgPath, []byte(fmt.Sprintf("server:\n  remote: %s\n", srv.URL)), 0o600)
old := cfgFile; cfgFile = cfgPath
t.Cleanup(func() { cfgFile = old })
t.Setenv("SPECGRAPH_API_KEY", "test-key")
```

`SPECGRAPH_API_KEY` is already public interface, so `t.Setenv` for auth is fine. The `cfgFile` swap covers the server URL.

When koanf migration lands, all tests using `cfgFile` swap migrate together to whatever pattern koanf adoption defines.

### D6. Hook script rename, separate commit

Task 31 already shipped `plugin/specgraph/hooks/session-start.sh` calling `specgraph mcp read-resource specgraph://prime`. With D1's top-level command, that line becomes `specgraph read-mcp-resource specgraph://prime`. The rename lands as a standalone commit before Task 32's main work, not as a `jj edit` amendment of the Task 31 commit. History stays honest: the rename is visible as a deliberate post-facto correction.

## Implementation shape

Slice 5's Task 32 is **two commits** in this order on top of `zppkyxtu de33a079` (Task 31):

### Commit 1: `fix(plugin): rename session-start hook to read-mcp-resource`

One-line edit to `plugin/specgraph/hooks/session-start.sh`:

```diff
-specgraph mcp read-resource specgraph://prime 2>&1 || {
+specgraph read-mcp-resource specgraph://prime 2>&1 || {
```

Body: cite the design decision (top-level command, not `mcp` parent). Reference `spgr-bncv`.

### Commit 2: `feat(cli): specgraph read-mcp-resource subcommand`

**Files:**

- Create: `cmd/specgraph/read_mcp_resource.go`
- Create: `cmd/specgraph/read_mcp_resource_test.go`

**Subcommand structure:**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 SpecGraph Contributors

package main

import (
    "fmt"
    "strings"

    "github.com/mark3labs/mcp-go/client"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/spf13/cobra"
)

var readMCPResourceCmd = &cobra.Command{
    Use:   "read-mcp-resource <uri>",
    Short: "Read an MCP resource from the SpecGraph server and print its body.",
    Args:  cobra.ExactArgs(1),
    RunE:  runReadMCPResource,
}

func init() {
    rootCmd.AddCommand(readMCPResourceCmd)
}

func runReadMCPResource(cmd *cobra.Command, args []string) error {
    uri := args[0]
    baseURL, project, err := resolveBaseURL()
    if err != nil {
        return fmt.Errorf("resolve server URL: %w", err)
    }
    mcpURL := strings.TrimRight(baseURL, "/") + "/mcp/"
    httpClient := newAuthenticatedHTTPClient(project)

    c, err := client.NewStreamableHttpClient(mcpURL, client.WithHTTPClient(httpClient))
    if err != nil {
        return fmt.Errorf("mcp client: %w", err)
    }
    defer c.Close()

    if err := c.Start(cmd.Context()); err != nil {
        return fmt.Errorf("mcp start: %w", err)
    }
    if _, err := c.Initialize(cmd.Context(), mcp.InitializeRequest{}); err != nil {
        return fmt.Errorf("mcp initialize: %w", err)
    }

    resp, err := c.ReadResource(cmd.Context(), mcp.ReadResourceRequest{
        Params: mcp.ReadResourceParams{URI: uri},
    })
    if err != nil {
        return fmt.Errorf("read resource %s: %w", uri, err)
    }
    for _, c := range resp.Contents {
        if text, ok := c.(mcp.TextResourceContents); ok {
            fmt.Print(text.Text)
            continue
        }
        fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping non-text resource content (type %T)\n", c)
    }
    fmt.Println()
    return nil
}
```

(Exact mcp-go API names verified during implementation; the shape above is illustrative.)

**Test:**

```go
func TestReadMCPResource(t *testing.T) {
    srv := startTestMCPServerWithPrime(t) // helper: in-process MCP server with prime resource
    t.Cleanup(srv.Close)

    cfgPath := writeTempConfig(t, srv.URL)
    old := cfgFile; cfgFile = cfgPath
    t.Cleanup(func() { cfgFile = old })
    t.Setenv("SPECGRAPH_API_KEY", "test-key")

    var out bytes.Buffer
    rootCmd.SetOut(&out)
    rootCmd.SetArgs([]string{"read-mcp-resource", "specgraph://prime"})
    require.NoError(t, rootCmd.Execute())
    require.Contains(t, out.String(), "SpecGraph Session Prime")
}
```

`startTestMCPServerWithPrime` registers the existing `internal/mcp` resource handler against a `httptest.NewServer` so the test exercises real server-side composer output.

## Manual verification

After commit 2:

```sh
go build ./cmd/specgraph
./specgraph serve &  # in another terminal, against a project with .specgraph.yaml
./specgraph read-mcp-resource specgraph://prime
# expect: prime markdown body, exit 0
```

Then exercise the hook end-to-end: launch Claude Code in the project directory and confirm session-start emits the prime body.

## Deferred work

- **`spgr-5kd5`: Adopt koanf for layered config + env provider.** Replaces hand-rolled `os.Getenv` calls with declarative env→config mapping. When that lands, the ad-hoc server URL override becomes natural and tests migrate off `cfgFile` swap.
- **Plan document update.** Task 32 in `docs/plans/2026-04-20-multi-platform-plugin-plan.md` (line 2953) should be edited to reference this design doc rather than carrying the obsolete stdio code blocks. Best done as part of Task 33 (CLAUDE.md update) since it's adjacent doc-keeping work.

## Open questions

None at write time. The design is implementable as written.
