# Task 32 Implementation Plan: `specgraph read-mcp-resource`

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a top-level `specgraph read-mcp-resource <uri>` CLI subcommand that reads an MCP resource from the SpecGraph HTTP server and prints its body to stdout. Enables the Phase B Slice 5 thin Claude Code plugin's session-start hook (Task 31) to fetch `specgraph://prime` end-to-end.

**Architecture:** Top-level cobra command. HTTP transport via `mark3labs/mcp-go` v0.45.0 (`client.NewStreamableHttpClient`) pointed at `<baseURL>/mcp/`, where `baseURL` and bearer token are resolved by the existing `cmd/specgraph/client.go` helpers (`resolveBaseURL`, `newAuthenticatedHTTPClient`). No new flags, no new env vars. See companion design at `docs/plans/2026-04-27-task-32-read-mcp-resource-design.md` for the rejected alternatives and their reasoning.

**Tech Stack:** Go 1.22+, cobra (CLI), mark3labs/mcp-go v0.45.0 (MCP client), connectrpc.com/connect (existing CLI HTTP transport reuse), httptest (in-process MCP server for tests), the project's `task check` quality gate.

---

## Pre-flight (read before starting)

- **Working directory:** `~/Code/github.com/.worktrees/specgraph/cursor-plugin`
- **Bead:** `spgr-bncv` (Phase B Slice 5 umbrella). This plan covers Task 32 of the parent plan `docs/plans/2026-04-20-multi-platform-plugin-plan.md`.
- **Expected stack at start:** `@` is empty, parent is `ktulmvpr 3f9ab37b docs(plans): task 32 design adjustment for read-mcp-resource`, grandparent is `zppkyxtu de33a079 feat(plugin): replace 13-skill Claude Code plugin with thin MCP client`, great-grandparent is `main`. Verify with `jj --no-pager log -r 'main..@' --no-graph -T 'change_id.short() ++ "  " ++ description.first_line() ++ "\n"'`.
- **DCO email:** `Sean Brandt <4678+seanb4t@users.noreply.github.com>` (noreply form). The plan's parent doc uses `4678+seanb4t@users.noreply.github.com` — that fails the DCO hook; do NOT copy it.
- **License header order on `.go` files:** `Copyright 2026 SpecGraph Contributors` first, then `SPDX-License-Identifier: Apache-2.0`. Match the existing pattern in `cmd/specgraph/client.go:1-2`.
- **Pre-commit ritual** (apply before EVERY `jj commit`):
  1. `jj --no-pager status`
  2. If `.beads/issues.jsonl` is dirty → `jj --no-pager restore --from @- .beads/issues.jsonl` (background bd noise unless you ran `bd` commands yourself).
  3. If `web/src/lib/api/gen/*.ts` is dirty → `jj --no-pager restore --from main --to @ web/src/lib/api/gen/` (protoc-gen-es regen drift).
  4. After commit, before any subsequent commit: confirm `@` is empty. If non-empty, STOP and report.
- **MCP profile note** (relevant to the manual smoke test, NOT the unit test): the unit test in Task 2 Step 2 stands up its own `mcp-go` server stub — no profile gating is exercised. But the manual smoke test in Step 7 hits a real `specgraph serve`, which goes through `internal/mcp/profiles.go:ProfileFromClientInfo`. The CLI initializes with `clientInfo.Name = "specgraph-cli"`, which defaults to `ProfileCore`. If the smoke test fails because `specgraph://prime` is gated above `ProfileCore`, fix it by either (a) adding `"specgraph-cli"` to the appropriate case in `profiles.go` and `client_id_contract_test.go`, or (b) confirming `prime` is exposed in `ProfileCore`. Discover empirically — do not pre-emptively change profile gating.

---

## Task 1: Rename session-start hook to match top-level command

**Files:**

- Modify: `plugin/specgraph/hooks/session-start.sh:11`

Task 31 shipped the hook calling `specgraph mcp read-resource`. The design (D1) decided against a `mcp` parent — it must call the top-level `specgraph read-mcp-resource`. This task lands the rename as a standalone commit before Task 2 wires up the new subcommand, keeping history honest about the post-Task-31 correction.

- [ ] **Step 1: Edit the hook script**

Open `plugin/specgraph/hooks/session-start.sh`. Change line 11 from:

```bash
specgraph mcp read-resource specgraph://prime 2>&1 || {
```

to:

```bash
specgraph read-mcp-resource specgraph://prime 2>&1 || {
```

Leave everything else untouched. Final file contents:

```bash
#!/usr/bin/env bash
# Copyright 2026 SpecGraph Contributors
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

if ! command -v specgraph >/dev/null 2>&1; then
  echo "specgraph CLI not found; skipping session prime" >&2
  exit 0
fi

specgraph read-mcp-resource specgraph://prime 2>&1 || {
  echo "specgraph prime failed (server unreachable?); session starts without prime" >&2
  exit 0
}
```

- [ ] **Step 2: Verify the script still parses and is executable**

```bash
bash -n plugin/specgraph/hooks/session-start.sh
ls -l plugin/specgraph/hooks/session-start.sh
```

Expected: `bash -n` exits 0 silently (syntax OK). `ls -l` shows `-rwxr-xr-x` (executable bit preserved by the edit).

- [ ] **Step 3: Pre-commit ritual**

```bash
jj --no-pager status
```

Expected: only `M plugin/specgraph/hooks/session-start.sh`. If `.beads/issues.jsonl` is also dirty, restore it first:

```bash
jj --no-pager restore --from @- .beads/issues.jsonl
```

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "fix(plugin): rename session-start hook to read-mcp-resource

Task 31 shipped the hook calling \`specgraph mcp read-resource\`. The
design decision (no \`mcp\` parent command — see
docs/plans/2026-04-27-task-32-read-mcp-resource-design.md §D1) requires
\`specgraph read-mcp-resource\` as a top-level command. This commit
lands the rename so Task 2 of the same slice can introduce the new
subcommand without retroactively editing the Task 31 commit.

The subcommand itself does not exist yet — the hook is non-functional
between this commit and the next, but it fails soft (\`exit 0\`) so
sessions still start.

spgr-bncv

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 5: Verify commit landed cleanly**

```bash
jj --no-pager log -r @- --no-graph -T 'description'
```

Expected: full subject + body + DCO trailer.

```bash
jj --no-pager status
```

Expected: `The working copy has no changes.` (Working copy now points to a fresh empty descendant.)

---

## Task 2: Implement `specgraph read-mcp-resource` subcommand (TDD)

**Files:**

- Create: `cmd/specgraph/read_mcp_resource.go`
- Create: `cmd/specgraph/read_mcp_resource_test.go`

This task is TDD: write the failing test first, watch it fail for the right reason, implement the minimum to pass, then verify. Single commit at the end.

The test pattern follows `cmd/specgraph/lifecycle_test.go` and `cmd/specgraph/test_helpers_test.go` — spin up an `httptest.NewServer` mounting the project's MCP server at `/mcp/`, write a temp config pointing at it, swap the package-level `cfgFile` variable, set `SPECGRAPH_API_KEY` via `t.Setenv`. The test exercises the real `internal/mcp` server and `internal/authoring.Composer` end-to-end against a real `mark3labs/mcp-go` HTTP client — no mocks of the protocol.

- [ ] **Step 1: Sanity-check the imports needed**

Confirm the v0.45.0 mcp-go API surface this task uses:

```bash
grep -n "^func NewStreamableHttpClient\|^func (c \*Client) Initialize\|^func (c \*Client) ReadResource" \
  ~/go/pkg/mod/github.com/mark3labs/mcp-go@v0.45.0/client/*.go
grep -n "^func WithHTTPBasicClient" \
  ~/go/pkg/mod/github.com/mark3labs/mcp-go@v0.45.0/client/transport/streamable_http.go
```

Expected: matches for all four. The constructor is `client.NewStreamableHttpClient`, the option is `transport.WithHTTPBasicClient`, the methods are `(*Client).Initialize` and `(*Client).ReadResource`.

- [ ] **Step 2: Write the failing test**

The test uses `mcp-go`'s own server library to stand up a minimal MCP server with a stubbed `specgraph://prime` resource. This avoids dragging the project's loopback-ConnectRPC composer wiring into the test — the goal is to verify the CLI's MCP wire-up (cobra → resolveBaseURL → mcp-go HTTP client → Initialize → ReadResource → stdout), not to re-test the composer (the composer has its own coverage in `internal/mcp/resources_test.go:TestPrimeResource`).

Create `cmd/specgraph/read_mcp_resource_test.go`:

```go
// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

func TestReadMCPResource_Prime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mcpHandler := newStubMCPHandler(t)
	mux := http.NewServeMux()
	mux.Handle("/mcp/", mcpHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	body := fmt.Sprintf("server:\n  remote: %s\n", srv.URL)
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	old := cfgFile
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = old })

	t.Setenv("SPECGRAPH_API_KEY", "spgr_sk_test_key")

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"read-mcp-resource", "specgraph://prime"})
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("execute: %v\nout: %s", err, out.String())
	}

	got := out.String()
	if !strings.Contains(got, "SpecGraph Session Prime") {
		t.Errorf("expected prime body to contain \"SpecGraph Session Prime\", got:\n%s", got)
	}
}

// newStubMCPHandler stands up a minimal MCP server using mcp-go's own server
// library, with a stubbed specgraph://prime resource. The handler is wrapped
// in auth.RequireAuth so the test also exercises the bearer-token round trip.
func newStubMCPHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "test", Key: "spgr_sk_test_key", Name: "test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("auth store: %v", err)
	}

	srv := mcpserver.NewMCPServer(
		"specgraph-test", "0.0.0",
		mcpserver.WithResourceCapabilities(false, false),
	)
	srv.AddResource(
		mcp.Resource{
			URI:      "specgraph://prime",
			Name:     "prime",
			MIMEType: "text/markdown",
		},
		func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "specgraph://prime",
					MIMEType: "text/markdown",
					Text:     "# SpecGraph Session Prime\n\nstub body for CLI test\n",
				},
			}, nil
		},
	)
	httpSrv := mcpserver.NewStreamableHTTPServer(srv)
	return auth.RequireAuth(store)(http.StripPrefix("/mcp", httpSrv))
}
```

The test follows the canonical CLI test pattern (`cmd/specgraph/lifecycle_test.go`, `cmd/specgraph/test_helpers_test.go`): `httptest.NewServer` + temp config file + `cfgFile` swap + `t.Setenv` for the API key. The MCP server is mounted at `/mcp/` with `auth.RequireAuth` wrapping, mirroring `serve.go:258` exactly.

- [ ] **Step 3: Run the test to confirm it fails for the expected reason**

```bash
go test ./cmd/specgraph/ -run TestReadMCPResource_Prime -v
```

Expected: FAIL with `unknown command "read-mcp-resource"` (cobra reports unknown subcommand).

If the failure is anything else (panic, network error, auth refusal, compile error from the test scaffolding), STOP — the test setup is wrong, not the missing implementation. Most likely cause: a typo in the imports or a misuse of `mcp-go`'s API. Re-read the verified API in Step 1's output and fix.

- [ ] **Step 4: Stub the cobra command (still failing)**

Create `cmd/specgraph/read_mcp_resource.go`:

```go
// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"github.com/spf13/cobra"
)

var readMCPResourceCmd = &cobra.Command{
	Use:   "read-mcp-resource <uri>",
	Short: "Read an MCP resource from the SpecGraph server and print its body.",
	Long: "Reads the requested MCP resource via streamable-HTTP transport from " +
		"the configured SpecGraph server (see resolveBaseURL) and prints the " +
		"first text content body to stdout. Bearer auth comes from " +
		"SPECGRAPH_API_KEY or the credentials file.",
	Args: cobra.ExactArgs(1),
	RunE: runReadMCPResource,
}

func init() {
	rootCmd.AddCommand(readMCPResourceCmd)
}

func runReadMCPResource(_ *cobra.Command, _ []string) error {
	return errors.New("not implemented")
}
```

Run:

```bash
go build ./cmd/specgraph
go test ./cmd/specgraph/ -run TestReadMCPResource_Prime -v
```

Expected: build succeeds; test FAILs with `not implemented`. The cobra dispatch now reaches `runReadMCPResource`.

- [ ] **Step 5: Implement the subcommand**

Replace the body of `runReadMCPResource` in `cmd/specgraph/read_mcp_resource.go` and add the necessary imports:

```go
// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
)

var readMCPResourceCmd = &cobra.Command{
	Use:   "read-mcp-resource <uri>",
	Short: "Read an MCP resource from the SpecGraph server and print its body.",
	Long: "Reads the requested MCP resource via streamable-HTTP transport from " +
		"the configured SpecGraph server (see resolveBaseURL) and prints the " +
		"first text content body to stdout. Bearer auth comes from " +
		"SPECGRAPH_API_KEY or the credentials file.",
	Args: cobra.ExactArgs(1),
	RunE: runReadMCPResource,
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

	c, err := client.NewStreamableHttpClient(mcpURL, transport.WithHTTPBasicClient(httpClient))
	if err != nil {
		return fmt.Errorf("mcp client: %w", err)
	}
	defer c.Close()

	ctx := cmd.Context()
	if err := c.Start(ctx); err != nil {
		return fmt.Errorf("mcp start: %w", err)
	}

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "specgraph-cli",
				Version: buildVersion(),
			},
		},
	}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}

	readReq := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: uri},
	}
	resp, err := c.ReadResource(ctx, readReq)
	if err != nil {
		return fmt.Errorf("read resource %s: %w", uri, err)
	}

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	for _, content := range resp.Contents {
		switch v := content.(type) {
		case mcp.TextResourceContents:
			fmt.Fprint(out, v.Text)
		case mcp.BlobResourceContents:
			fmt.Fprintf(errOut, "warning: skipping non-text resource content (uri=%s, mime=%s)\n", v.URI, v.MIMEType)
		default:
			fmt.Fprintf(errOut, "warning: skipping unknown resource content type %T\n", content)
		}
	}
	fmt.Fprintln(out)
	return nil
}
```

Notes on the implementation:

- **No URL validation on `uri`** — the spec lets servers interpret any URI. Failing on a malformed URI is the server's job.
- **`Close()` is deferred immediately after construction** so a failed `Start` still cleans up.
- **Multi-content response** is printed in order; binary content is skipped with a stderr warning rather than printing base64 noise.
- **`buildVersion()`** already exists in `cmd/specgraph/main.go` — reuse it for `ClientInfo.Version`. No additional wiring.

- [ ] **Step 6: Run the test to verify it passes**

```bash
go test ./cmd/specgraph/ -run TestReadMCPResource_Prime -v
```

Expected: PASS. The test prints output containing the substring `"SpecGraph Session Prime"` from the stub's response body.

If FAIL with auth error (`401`/`403`): the `t.Setenv("SPECGRAPH_API_KEY", ...)` value must match the API key registered in `newStubMCPHandler`. Both are `"spgr_sk_test_key"` in this plan — verify both match.

If FAIL with `unsupported protocol version` or similar protocol-negotiation error: confirm the test imports `mcp.LATEST_PROTOCOL_VERSION` and that the server stub uses the default protocol (which `mcp-go`'s `NewMCPServer` does automatically).

- [ ] **Step 7: Manual smoke test against a real server**

Build and exercise end-to-end:

```bash
go build -o /tmp/specgraph ./cmd/specgraph
# In another terminal, in a project with .specgraph.yaml:
#   /tmp/specgraph serve
/tmp/specgraph read-mcp-resource specgraph://prime
```

Expected: prime body printed to stdout, exit 0. If the server is not running, expect a network error and non-zero exit — that's correct (the hook script handles this case via `||`).

If the smoke test fails with a profile/permission error (resource not exposed to your client), consult the Pre-flight "MCP profile note" and add `"specgraph-cli"` to `internal/mcp/profiles.go:ProfileFromClientInfo` plus a row to `internal/mcp/client_id_contract_test.go:TestProfileFromClientInfo_Contract`. Land that as its own commit (named e.g. `feat(mcp): add specgraph-cli profile mapping`) BEFORE the Task 2 main commit.

Then exercise the hook end-to-end if you have a Claude Code session available:

```bash
# In a project with .specgraph.yaml and a running specgraph serve:
claude --plugin-dir ~/Code/github.com/.worktrees/specgraph/cursor-plugin/plugin/specgraph
# In the new session, observe that the prime body appears in the initial context.
```

This step is best-effort — if no Claude Code session is reachable, document that in the commit message and continue.

- [ ] **Step 8: Quality gate**

```bash
task check
```

Expected: exit 0 (fmt:check + license:check + lint + build + unit tests with `-race`). If license:check flags either new file, fix the header (Copyright before SPDX, "SpecGraph Contributors" not "Sean Brandt") and re-run.

- [ ] **Step 9: Pre-commit ritual**

```bash
jj --no-pager status
```

Expected: `A cmd/specgraph/read_mcp_resource.go`, `A cmd/specgraph/read_mcp_resource_test.go`, plus possibly `M internal/mcp/profiles.go` + `M internal/mcp/client_id_contract_test.go` if Step 6 forced a profile mapping.

If `.beads/issues.jsonl` is dirty:

```bash
jj --no-pager restore --from @- .beads/issues.jsonl
```

If `web/src/lib/api/gen/*.ts` is dirty:

```bash
jj --no-pager restore --from main --to @ web/src/lib/api/gen/
```

- [ ] **Step 10: Commit**

If Step 6 required a profile mapping, commit it FIRST as its own change:

```bash
jj --no-pager commit -m "feat(mcp): add specgraph-cli to ProfileFromClientInfo

Maps clientInfo.Name=\"specgraph-cli\" to ProfileAuthoring (or whichever
profile the prime resource lives in). The new \`specgraph
read-mcp-resource\` CLI subcommand reports this name during MCP
Initialize; without this mapping the CLI would be denied access to the
prime resource that motivated the subcommand.

Updates client_id_contract_test.go to lock the mapping in.

spgr-bncv

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then commit the subcommand:

```bash
jj --no-pager commit -m "feat(cli): specgraph read-mcp-resource subcommand

Top-level cobra command that reads an MCP resource via streamable-HTTP
transport (mark3labs/mcp-go v0.45.0) from the configured SpecGraph
server and prints the text body to stdout. Reuses the existing
resolveBaseURL / newAuthenticatedHTTPClient helpers in client.go for
URL and bearer-token resolution; no new flags or env vars.

Used by the Phase B Slice 5 thin Claude Code plugin's session-start
hook to fetch specgraph://prime end-to-end. Pairs with the prior
fix(plugin) commit that renamed the hook to call this subcommand.

Test (cmd/specgraph/read_mcp_resource_test.go) wires a real in-process
internal/mcp server behind httptest, follows the canonical cfgFile
swap pattern from lifecycle_test.go, and exercises the live mcp-go
HTTP client end-to-end against the real composer output — no protocol
mocks.

See docs/plans/2026-04-27-task-32-read-mcp-resource-design.md for the
design and the rejected alternatives (--server flag, --api-key flag,
SPECGRAPH_SERVER env var; deferred koanf adoption is spgr-5kd5).

spgr-bncv

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 11: Verify commit landed cleanly**

```bash
jj --no-pager log -r 'main..@-' --no-graph -T 'change_id.short() ++ "  " ++ description.first_line() ++ "\n"'
jj --no-pager status
```

Expected stack (top to bottom):

- (empty `@`)
- `feat(cli): specgraph read-mcp-resource subcommand`
- (optional) `feat(mcp): add specgraph-cli to ProfileFromClientInfo`
- `fix(plugin): rename session-start hook to read-mcp-resource`
- `docs(plans): task 32 design adjustment for read-mcp-resource`
- `feat(plugin): replace 13-skill Claude Code plugin with thin MCP client`
- `main`

`jj --no-pager status` must print `The working copy has no changes.`

---

## Success criteria

When this plan is complete:

1. `task check` passes from the worktree root.
2. `go test ./cmd/specgraph/ -run TestReadMCPResource_Prime -v` passes.
3. `jj --no-pager log -r 'main..@-' --no-graph` shows two-or-three new commits on top of `ktulmvpr 3f9ab37b`.
4. The session-start hook calls `specgraph read-mcp-resource specgraph://prime` (Task 1) and that command exists (Task 2).
5. Manual smoke test (Step 7) prints the prime body or — if no server reachable — fails cleanly with a non-zero exit.

---

## What this plan does NOT cover

- Tasks 33–38 of the parent plan (`docs/plans/2026-04-20-multi-platform-plugin-plan.md`).
- Any change to `internal/mcp` profile gating beyond what's required for the test to pass.
- Adoption of a global `--server` flag, `--api-key` flag, or `SPECGRAPH_SERVER` env var (rejected; see design doc §D4).
- Adoption of koanf for layered config + env provider (deferred to bead `spgr-5kd5`).
- Editing the parent plan's Task 32 prose to point at this design+plan pair (folded into Task 33's CLAUDE.md/doc upkeep scope per design §"Deferred work").
