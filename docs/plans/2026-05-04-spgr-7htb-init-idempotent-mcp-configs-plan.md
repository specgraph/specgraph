# Idempotent `specgraph init` with managed MCP configs (spgr-7htb) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `specgraph init` idempotent and config-only (no Docker startup).
Add the responsibility of generating the three per-harness MCP config files
(Cursor `.cursor/mcp.json`, Claude Code `.mcp.json`, OpenCode `opencode.json`)
from `.specgraph.yaml` + global config via JSON Merge Patch (RFC 7396).

**Architecture:** New package `internal/config/mcpconfigs/` exposing
`ManagedConfigs(slug, serverURL) []ManagedConfig` (pure render of merge-patch
documents) and `Sync(projectDir, configs) ([]SyncResult, error)` (read existing
file → apply RFC 7396 merge patch via `evanphx/json-patch/v5` → canonicalize via
`encoding/json` → write only if changed). `cmd/specgraph/init.go`'s `runInit`
becomes idempotent: drops the "already initialized" guard and the `runUp` call,
adds a slug-consistency check between arg and existing `.specgraph.yaml`, and
calls `mcpconfigs.Sync` to maintain the per-harness configs.

**Tech Stack:** Go 1.22+, `github.com/evanphx/json-patch/v5` (new dependency,
RFC 7396 JSON Merge Patch), `encoding/json` (stdlib, for canonical
pretty-printing), `gopkg.in/yaml.v3` (existing), Cobra (existing).

**Companion design:** `docs/plans/2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md`

---

## Pre-flight (read before starting)

- **Working directory:** `~/Code/github.com/.worktrees/specgraph/cursor-plugin`
- **Bead:** `spgr-7htb`. Slice umbrella: `spgr-bncv`.
- **Expected stack at start:** `@` is empty, parent is the most recent commit
  on `main` (post-#928). Verify with
  `jj --no-pager log -r 'main..@' --no-graph -T 'change_id.short() ++ "  " ++ description.first_line() ++ "\n"'`.
- **DCO email:** `Sean Brandt <4678+seanb4t@users.noreply.github.com>` (noreply form).
- **License header on `.go` files:** match the project's `LICENSE_HEADER` file
  bytes-for-bytes — `// SPDX-License-Identifier: Apache-2.0` then
  `// Copyright 2026 Sean Brandt`. `task check` runs
  `addlicense -check -f LICENSE_HEADER` which compares the header text
  exactly; any deviation (e.g., a different copyright holder string) fails.
  See the existing pattern in `internal/config/project.go:1-2` and
  `cmd/specgraph/init.go:1-2`.
- **Pre-commit ritual** (apply before EVERY `jj commit`):
  1. `jj --no-pager status`
  2. If `.beads/issues.jsonl` is dirty → `jj --no-pager restore --from @- .beads/issues.jsonl`
     (background bd noise unless you ran `bd` commands yourself).
  3. If `web/src/lib/api/gen/*.ts` is dirty → `jj --no-pager restore --from main --to @ web/src/lib/api/gen/`.
  4. After commit, before any subsequent commit: confirm `@` is empty.
- **gh auth note:** before any `jj git push`, run `gh auth switch -u seanb4t -h github.com`.
  The corporate SSO account doesn't have push access to `specgraph/specgraph`.
- **DO NOT run `specgraph init` against this repo.** This repo is intentionally
  not init'd; Task 38 (dogfood cutover) handles that. The committed
  `.cursor/mcp.json`, `.mcp.json`, and `opencode.json` at the repo root remain
  hand-maintained until Task 38.

---

## File structure

| File | Action | Responsibility |
|---|---|---|
| `go.mod`, `go.sum` | modify | Add `github.com/evanphx/json-patch/v5` dependency |
| `internal/config/mcpconfigs/configs.go` | create | `ManagedConfig` type, `ManagedConfigs(slug, serverURL)`, three per-harness builders, `ensureMCPSuffix` helper |
| `internal/config/mcpconfigs/configs_test.go` | create | Per-harness golden tests for the merge-patch documents |
| `internal/config/mcpconfigs/sync.go` | create | `SyncResult` type, `Sync(projectDir, configs)`, `syncOne` helper, JSONC sibling check, JSON merge + canonicalize + diff-and-write |
| `internal/config/mcpconfigs/sync_test.go` | create | Table-driven `Sync` tests covering create/update/no-op/preserve-siblings/preserve-customizations/jsonc-sibling/malformed-json/idempotence |
| `cmd/specgraph/init.go` | modify | Make `runInit` idempotent: drop `runUp`, drop "already initialized" guard, add slug-consistency check, resolve server URL, call `mcpconfigs.Sync`, print results |
| `cmd/specgraph/init_test.go` | modify | Add tests for the slug × existing matrix and the runs-twice idempotence case |
| `CLAUDE.md` | modify | Document the per-harness MCP config writer responsibility |
| `docs/verification/cursor.md` | modify | Note `specgraph init` writes the file; the committed example will be replaced during Task 38 |
| `docs/verification/opencode.md` | modify | Same note |

---

## Task 1: Add `github.com/evanphx/json-patch/v5` dependency

**Files:**

- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Verify the module is not already present**

```bash
grep -n "evanphx/json-patch" go.mod || echo "(not present, proceed)"
```

Expected: `(not present, proceed)`. If a match is shown, stop and report.

- [ ] **Step 2: Add the dependency**

```bash
go get github.com/evanphx/json-patch/v5@latest
```

Expected: completes without error; `go.mod` updated with a new `require` line for `github.com/evanphx/json-patch/v5`.

- [ ] **Step 3: Tidy modules**

```bash
go mod tidy
```

Expected: completes without error; `go.sum` updated with checksum entries.

- [ ] **Step 4: Smoke-test the API**

Create `/tmp/jsonpatch_smoke.go`:

```go
package main

import (
	"fmt"
	jsonpatch "github.com/evanphx/json-patch/v5"
)

func main() {
	out, err := jsonpatch.MergePatch([]byte(`{"a":1,"keep":"me"}`), []byte(`{"a":2,"new":"field"}`))
	fmt.Println(string(out), err)
}
```

Run from the repo root:

```bash
go run /tmp/jsonpatch_smoke.go
rm /tmp/jsonpatch_smoke.go
```

Expected output:

```text
{"a":2,"keep":"me","new":"field"} <nil>
```

(Confirms RFC 7396 semantics: `a` replaced, `keep` preserved, `new` added.)

- [ ] **Step 5: Pre-commit ritual**

```bash
jj --no-pager status
```

Expected: `M go.mod` and `M go.sum`. If `.beads/issues.jsonl` is also dirty:

```bash
jj --no-pager restore --from @- .beads/issues.jsonl
```

- [ ] **Step 6: Commit**

```bash
jj --no-pager commit go.mod go.sum -m "chore(deps): add github.com/evanphx/json-patch/v5

Adds RFC 7396 JSON Merge Patch support, used by the new
internal/config/mcpconfigs package (introduced in subsequent commits) to
maintain per-harness MCP config files (.cursor/mcp.json, .mcp.json,
opencode.json) idempotently from .specgraph.yaml + global config.

The library supports both RFC 6902 (JSON Patch) and RFC 7396 (JSON Merge
Patch); we only use the merge-patch path via jsonpatch.MergePatch.

spgr-7htb

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 7: Verify clean**

```bash
jj --no-pager status
```

Expected: `The working copy has no changes.`

---

## Task 2: `mcpconfigs` package — `ManagedConfigs` and per-harness builders

**Files:**

- Create: `internal/config/mcpconfigs/configs.go`
- Create: `internal/config/mcpconfigs/configs_test.go`

This task lands the pure render functions: no I/O, just constructing the
JSON Merge Patch document for each harness from `(slug, serverURL)`.

- [ ] **Step 1: Write the failing test file**

Create `internal/config/mcpconfigs/configs_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcpconfigs

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestEnsureMCPSuffix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"http://127.0.0.1:7890", "http://127.0.0.1:7890/mcp/"},
		{"http://127.0.0.1:7890/", "http://127.0.0.1:7890/mcp/"},
		{"http://127.0.0.1:7890/mcp/", "http://127.0.0.1:7890/mcp/"},
		{"https://specgraph.example.com", "https://specgraph.example.com/mcp/"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := ensureMCPSuffix(tc.in)
			if got != tc.want {
				t.Errorf("ensureMCPSuffix(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestManagedConfigs_PathsAndCount(t *testing.T) {
	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	if got, want := len(configs), 3; got != want {
		t.Fatalf("ManagedConfigs returned %d entries, want %d", got, want)
	}
	wantPaths := map[string]bool{
		".cursor/mcp.json": false,
		".mcp.json":        false,
		"opencode.json":    false,
	}
	for _, c := range configs {
		if _, ok := wantPaths[c.Path]; !ok {
			t.Errorf("unexpected path %q", c.Path)
			continue
		}
		wantPaths[c.Path] = true
	}
	for path, seen := range wantPaths {
		if !seen {
			t.Errorf("missing path %q", path)
		}
	}
}

func TestManagedConfigs_Cursor(t *testing.T) {
	got := patchFor(t, ManagedConfigs("specgraph", "http://127.0.0.1:7890"), ".cursor/mcp.json")
	want := map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"url": "http://127.0.0.1:7890/mcp/",
				"headers": map[string]any{
					"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Cursor patch mismatch.\n got: %v\nwant: %v", got, want)
	}
}

func TestManagedConfigs_ClaudeCode(t *testing.T) {
	got := patchFor(t, ManagedConfigs("specgraph", "http://127.0.0.1:7890"), ".mcp.json")
	want := map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"type": "http",
				"url":  "http://127.0.0.1:7890/mcp/",
				"headers": map[string]any{
					"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Claude Code patch mismatch.\n got: %v\nwant: %v", got, want)
	}
}

func TestManagedConfigs_OpenCode(t *testing.T) {
	got := patchFor(t, ManagedConfigs("specgraph", "http://127.0.0.1:7890"), "opencode.json")
	want := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"specgraph": map[string]any{
				"type":    "remote",
				"url":     "http://127.0.0.1:7890/mcp/",
				"enabled": true,
				"headers": map[string]any{
					"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("OpenCode patch mismatch.\n got: %v\nwant: %v", got, want)
	}
}

func TestManagedConfigs_SlugFlowsThrough(t *testing.T) {
	configs := ManagedConfigs("my-other-project", "http://127.0.0.1:7890")
	for _, c := range configs {
		var m map[string]any
		if err := json.Unmarshal(c.Patch, &m); err != nil {
			t.Fatalf("unmarshal %s: %v", c.Path, err)
		}
		// Walk to the headers; both shapes (mcpServers.specgraph.headers and
		// mcp.specgraph.headers) end at headers.X-Specgraph-Project.
		var server map[string]any
		switch c.Path {
		case ".cursor/mcp.json", ".mcp.json":
			server = m["mcpServers"].(map[string]any)["specgraph"].(map[string]any)
		case "opencode.json":
			server = m["mcp"].(map[string]any)["specgraph"].(map[string]any)
		}
		headers := server["headers"].(map[string]any)
		if got := headers["X-Specgraph-Project"]; got != "my-other-project" {
			t.Errorf("%s: X-Specgraph-Project = %v, want %q", c.Path, got, "my-other-project")
		}
	}
}

// patchFor decodes the patch for the named harness path and returns it as a
// generic map. Fails the test if the path isn't present.
func patchFor(t *testing.T, configs []ManagedConfig, path string) map[string]any {
	t.Helper()
	for _, c := range configs {
		if c.Path != path {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(c.Patch, &m); err != nil {
			t.Fatalf("unmarshal %s patch: %v", path, err)
		}
		return m
	}
	t.Fatalf("path %q not found in configs", path)
	return nil
}
```

- [ ] **Step 2: Verify tests fail**

```bash
go test ./internal/config/mcpconfigs/ -v
```

Expected: build error (`package mcpconfigs is not in std` or similar — the
package doesn't exist yet).

- [ ] **Step 3: Implement `configs.go`**

Create `internal/config/mcpconfigs/configs.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package mcpconfigs renders and synchronizes per-harness MCP configuration
// files (Cursor .cursor/mcp.json, Claude Code .mcp.json, OpenCode
// opencode.json) from a (slug, serverURL) pair.
//
// The mutation primitive is RFC 7396 JSON Merge Patch (via
// github.com/evanphx/json-patch/v5). Each per-harness builder produces a
// patch document that names only the fields specgraph manages — url,
// Authorization header, X-Specgraph-Project header, and harness-specific
// shape fields like type and enabled. Applying the patch to an existing
// file updates managed keys and preserves all siblings and user
// customizations under the specgraph entry.
package mcpconfigs

import (
	"encoding/json"
	"strings"
)

// ManagedConfig pairs a project-relative file path with the JSON Merge Patch
// document specgraph manages for that file's specgraph server entry.
type ManagedConfig struct {
	// Path is the file path relative to the project root (e.g. ".cursor/mcp.json").
	Path string

	// Patch is the RFC 7396 JSON Merge Patch document. It contains only
	// fields specgraph manages; applying it preserves siblings and
	// user-added fields.
	Patch json.RawMessage
}

// ManagedConfigs returns the patches for the three currently-supported
// harnesses (Cursor, Claude Code, OpenCode). slug is the project slug from
// .specgraph.yaml; serverURL is the resolved server base URL (without /mcp/
// suffix; the helper appends it).
func ManagedConfigs(slug, serverURL string) []ManagedConfig {
	mcpURL := ensureMCPSuffix(serverURL)
	return []ManagedConfig{
		cursorConfig(slug, mcpURL),
		claudeCodeConfig(slug, mcpURL),
		openCodeConfig(slug, mcpURL),
	}
}

// ensureMCPSuffix returns serverURL with a trailing "/mcp/" segment, leaving
// the URL unchanged if it already ends with that suffix.
func ensureMCPSuffix(serverURL string) string {
	trimmed := strings.TrimRight(serverURL, "/")
	if strings.HasSuffix(trimmed, "/mcp") {
		return trimmed + "/"
	}
	return trimmed + "/mcp/"
}

// cursorConfig returns the merge patch for .cursor/mcp.json. Cursor uses
// ${env:NAME} env-var substitution.
func cursorConfig(slug, mcpURL string) ManagedConfig {
	patch, _ := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"url": mcpURL,
				"headers": map[string]any{
					"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": slug,
				},
			},
		},
	})
	return ManagedConfig{Path: ".cursor/mcp.json", Patch: patch}
}

// claudeCodeConfig returns the merge patch for .mcp.json. Claude Code uses
// ${NAME} env-var substitution and requires "type": "http" for HTTP MCP.
func claudeCodeConfig(slug, mcpURL string) ManagedConfig {
	patch, _ := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"type": "http",
				"url":  mcpURL,
				"headers": map[string]any{
					"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": slug,
				},
			},
		},
	})
	return ManagedConfig{Path: ".mcp.json", Patch: patch}
}

// openCodeConfig returns the merge patch for opencode.json. OpenCode uses
// {env:NAME} env-var substitution (no leading $), wraps servers under "mcp"
// (singular), and requires "type": "remote" for HTTP MCP. The top-level
// "$schema" sibling tells OpenCode which schema to validate against.
func openCodeConfig(slug, mcpURL string) ManagedConfig {
	patch, _ := json.Marshal(map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"specgraph": map[string]any{
				"type":    "remote",
				"url":     mcpURL,
				"enabled": true,
				"headers": map[string]any{
					"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": slug,
				},
			},
		},
	})
	return ManagedConfig{Path: "opencode.json", Patch: patch}
}
```

- [ ] **Step 4: Run tests; verify all pass**

```bash
go test ./internal/config/mcpconfigs/ -v
```

Expected: 6 tests pass (`TestEnsureMCPSuffix`, `TestManagedConfigs_PathsAndCount`,
`TestManagedConfigs_Cursor`, `TestManagedConfigs_ClaudeCode`,
`TestManagedConfigs_OpenCode`, `TestManagedConfigs_SlugFlowsThrough`).

- [ ] **Step 5: Run `task check`**

```bash
task check
```

Expected: exit 0. If license:check flags the new files, fix the header
(SPDX first, Copyright second — match `internal/config/project.go:1-2`).

- [ ] **Step 6: Pre-commit ritual + commit**

```bash
jj --no-pager status
```

Expected: `A internal/config/mcpconfigs/configs.go`,
`A internal/config/mcpconfigs/configs_test.go`.

```bash
jj --no-pager commit internal/config/mcpconfigs/configs.go internal/config/mcpconfigs/configs_test.go -m "feat(mcpconfigs): ManagedConfigs render for three supported harnesses

Adds the internal/config/mcpconfigs package with ManagedConfig type and
ManagedConfigs(slug, serverURL) function that returns the three RFC 7396
JSON Merge Patch documents specgraph manages for Cursor (.cursor/mcp.json),
Claude Code (.mcp.json), and OpenCode (opencode.json).

Each per-harness builder constructs only the fields specgraph owns:
- url (with /mcp/ suffix appended via ensureMCPSuffix helper)
- headers.Authorization (env-var substituted via the harness-specific syntax:
  Cursor \${env:NAME}, Claude Code \${NAME}, OpenCode {env:NAME})
- headers.X-Specgraph-Project (the project slug)
- harness-specific shape fields: type=http (Claude Code), type=remote +
  enabled=true + top-level \$schema (OpenCode)

Pure render — no I/O. Sync(...) lands in a subsequent commit.

spgr-7htb

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 7: Verify clean**

```bash
jj --no-pager status
```

Expected: `The working copy has no changes.`

---

## Task 3: `mcpconfigs.Sync` — happy path (create, merge, preserve)

**Files:**

- Create: `internal/config/mcpconfigs/sync.go`
- Create: `internal/config/mcpconfigs/sync_test.go`

This task lands the I/O-heavy `Sync` function with the happy paths: file
missing → create, existing file with other servers → preserve siblings,
existing file with stale specgraph entry → update managed fields, existing
file with user customizations under specgraph → preserve user fields.

- [ ] **Step 1: Write the failing test file**

Create `internal/config/mcpconfigs/sync_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcpconfigs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// syncFixtures returns the three canonical-content map[string]any documents
// the Sync function should emit for slug=specgraph, serverURL=http://127.0.0.1:7890.
func syncFixtures() (cursor, claude, opencode map[string]any) {
	cursor = map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"url": "http://127.0.0.1:7890/mcp/",
				"headers": map[string]any{
					"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	claude = map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"type": "http",
				"url":  "http://127.0.0.1:7890/mcp/",
				"headers": map[string]any{
					"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	opencode = map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"specgraph": map[string]any{
				"type":    "remote",
				"url":     "http://127.0.0.1:7890/mcp/",
				"enabled": true,
				"headers": map[string]any{
					"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	return
}

func TestSync_CreatesMissingFiles(t *testing.T) {
	dir := t.TempDir()
	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")

	results, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	wantPaths := []string{".cursor/mcp.json", ".mcp.json", "opencode.json"}
	gotByPath := map[string]string{}
	for _, r := range results {
		gotByPath[r.Path] = r.Action
	}
	for _, p := range wantPaths {
		if got := gotByPath[p]; got != "created" {
			t.Errorf("%s: action = %q, want %q", p, got, "created")
		}
	}

	cursor, claude, opencode := syncFixtures()
	assertFileEquals(t, filepath.Join(dir, ".cursor/mcp.json"), cursor)
	assertFileEquals(t, filepath.Join(dir, ".mcp.json"), claude)
	assertFileEquals(t, filepath.Join(dir, "opencode.json"), opencode)
}

func TestSync_PreservesOtherServers_Cursor(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{
  "mcpServers": {
    "context7": {
      "url": "https://mcp.context7.com/mcp",
      "headers": {"CONTEXT7_API_KEY": "${env:CONTEXT7}"}
    }
  }
}
`)
	if err := os.WriteFile(cursorPath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := actionFor(results, ".cursor/mcp.json"); got != "updated" {
		t.Errorf(".cursor/mcp.json action = %q, want %q", got, "updated")
	}

	got := readJSON(t, cursorPath)
	servers := got["mcpServers"].(map[string]any)
	if _, ok := servers["context7"]; !ok {
		t.Error("context7 server was not preserved")
	}
	if _, ok := servers["specgraph"]; !ok {
		t.Error("specgraph server was not added")
	}
}

func TestSync_UpdatesStaleSpecgraphEntry(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{
  "mcpServers": {
    "specgraph": {
      "url": "http://old.host:1234/mcp/",
      "headers": {
        "Authorization": "Bearer stale",
        "X-Specgraph-Project": "old-slug"
      }
    },
    "atlassian": {
      "url": "https://mcp.atlassian.com",
      "headers": {"Authorization": "Bearer foo"}
    }
  }
}
`)
	if err := os.WriteFile(cursorPath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	if _, err := Sync(dir, configs); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got := readJSON(t, cursorPath)
	servers := got["mcpServers"].(map[string]any)
	specgraph := servers["specgraph"].(map[string]any)
	if specgraph["url"] != "http://127.0.0.1:7890/mcp/" {
		t.Errorf("url not updated: %v", specgraph["url"])
	}
	headers := specgraph["headers"].(map[string]any)
	if headers["X-Specgraph-Project"] != "specgraph" {
		t.Errorf("project not updated: %v", headers["X-Specgraph-Project"])
	}
	if headers["Authorization"] != "Bearer ${env:SPECGRAPH_API_KEY}" {
		t.Errorf("auth not updated: %v", headers["Authorization"])
	}
	if _, ok := servers["atlassian"]; !ok {
		t.Error("atlassian server was not preserved")
	}
}

func TestSync_PreservesUserCustomizationsUnderSpecgraph(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{
  "mcpServers": {
    "specgraph": {
      "url": "http://old.host:1234/mcp/",
      "headers": {
        "Authorization": "Bearer stale",
        "X-Specgraph-Project": "old-slug",
        "X-User-Custom": "preserve-me"
      },
      "comment": "my dev notes"
    }
  }
}
`)
	if err := os.WriteFile(cursorPath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	if _, err := Sync(dir, configs); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got := readJSON(t, cursorPath)
	specgraph := got["mcpServers"].(map[string]any)["specgraph"].(map[string]any)
	if specgraph["comment"] != "my dev notes" {
		t.Errorf("user comment was not preserved: %v", specgraph["comment"])
	}
	headers := specgraph["headers"].(map[string]any)
	if headers["X-User-Custom"] != "preserve-me" {
		t.Errorf("user custom header was not preserved: %v", headers["X-User-Custom"])
	}
	// And managed fields are still updated to canonical values.
	if headers["Authorization"] != "Bearer ${env:SPECGRAPH_API_KEY}" {
		t.Errorf("managed auth was not updated: %v", headers["Authorization"])
	}
}

// Helper functions used by sync tests.

func actionFor(results []SyncResult, path string) string {
	for _, r := range results {
		if r.Path == path {
			return r.Action
		}
	}
	return ""
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

func assertFileEquals(t *testing.T, path string, want map[string]any) {
	t.Helper()
	got := readJSON(t, path)
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.MarshalIndent(got, "", "  ")
		wj, _ := json.MarshalIndent(want, "", "  ")
		t.Errorf("%s mismatch.\n got: %s\nwant: %s", path, gj, wj)
	}
}

```

- [ ] **Step 2: Verify tests fail**

```bash
go test ./internal/config/mcpconfigs/ -run TestSync -v
```

Expected: build error (`Sync` and `SyncResult` not defined).

- [ ] **Step 3: Implement `sync.go`**

Create `internal/config/mcpconfigs/sync.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcpconfigs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// SyncResult reports what Sync did to a single managed config file.
type SyncResult struct {
	// Path is the project-relative path of the file (matches ManagedConfig.Path).
	Path string

	// Action is one of: "created" (file did not exist before), "updated"
	// (file existed and bytes changed), or "no-op" (file existed with
	// canonical content already).
	Action string
}

// Sync applies each ManagedConfig's merge patch to its target file under
// projectDir. For each file: if the file is missing, write the canonicalized
// patch as the full file content (action "created"). Otherwise read existing
// content, apply the patch via RFC 7396 merge, canonicalize the result, and
// write only if the result differs from the existing bytes (action "updated"
// or "no-op").
//
// Special: for opencode.json, refuse if a sibling opencode.jsonc exists in
// the same directory; OpenCode supports both formats and writing
// opencode.json next to a user's pre-existing opencode.jsonc would create
// ambiguous active-config state.
//
// On any error, returns the partial results collected up to that point and
// the error wrapped with the offending path.
func Sync(projectDir string, configs []ManagedConfig) ([]SyncResult, error) {
	results := make([]SyncResult, 0, len(configs))
	for _, cfg := range configs {
		result, err := syncOne(projectDir, cfg)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func syncOne(projectDir string, cfg ManagedConfig) (SyncResult, error) {
	fullPath := filepath.Join(projectDir, cfg.Path)

	// OpenCode-only: refuse if opencode.jsonc sibling exists.
	if cfg.Path == "opencode.json" {
		jsoncPath := filepath.Join(projectDir, "opencode.jsonc")
		if _, statErr := os.Stat(jsoncPath); statErr == nil {
			return SyncResult{}, fmt.Errorf(
				"found opencode.jsonc alongside opencode.json; consolidate to one file (init does not yet manage opencode.jsonc)",
			)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return SyncResult{}, fmt.Errorf("stat %s: %w", jsoncPath, statErr)
		}
	}

	existing, err := os.ReadFile(fullPath)
	fileExisted := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return SyncResult{}, fmt.Errorf("read %s: %w", fullPath, err)
	}

	// If the file exists, validate it parses as JSON before any merge.
	if fileExisted {
		var probe any
		if jerr := json.Unmarshal(existing, &probe); jerr != nil {
			return SyncResult{}, fmt.Errorf("parse %s: %w", fullPath, jerr)
		}
	}

	// Compute the merged document.
	var merged []byte
	if fileExisted {
		merged, err = jsonpatch.MergePatch(existing, cfg.Patch)
		if err != nil {
			return SyncResult{}, fmt.Errorf("merge patch %s: %w", fullPath, err)
		}
	} else {
		// Equivalent to MergePatch({}, cfg.Patch) — produce the patch as
		// the canonical doc.
		merged, err = jsonpatch.MergePatch([]byte(`{}`), cfg.Patch)
		if err != nil {
			return SyncResult{}, fmt.Errorf("merge patch %s: %w", fullPath, err)
		}
	}

	// Canonicalize: 2-space indent + trailing newline. Map keys are emitted
	// in alphabetical order by encoding/json, giving deterministic output.
	canonical, err := canonicalize(merged)
	if err != nil {
		return SyncResult{}, fmt.Errorf("canonicalize %s: %w", fullPath, err)
	}

	if fileExisted && bytes.Equal(existing, canonical) {
		return SyncResult{Path: cfg.Path, Action: "no-op"}, nil
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil { //nolint:gosec // 0755 is intentional for config dirs
		return SyncResult{}, fmt.Errorf("mkdir %s: %w", filepath.Dir(fullPath), err)
	}
	if err := os.WriteFile(fullPath, canonical, 0o600); err != nil {
		return SyncResult{}, fmt.Errorf("write %s: %w", fullPath, err)
	}

	if fileExisted {
		return SyncResult{Path: cfg.Path, Action: "updated"}, nil
	}
	return SyncResult{Path: cfg.Path, Action: "created"}, nil
}

// canonicalize returns the JSON document re-marshaled with 2-space indent
// and a trailing newline. This is the form Sync compares against existing
// file bytes for the no-op short-circuit.
func canonicalize(data []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal indent: %w", err)
	}
	return append(out, '\n'), nil
}
```

- [ ] **Step 4: Run tests; verify the four happy-path tests pass**

```bash
go test ./internal/config/mcpconfigs/ -run TestSync -v
```

Expected: `TestSync_CreatesMissingFiles`,
`TestSync_PreservesOtherServers_Cursor`,
`TestSync_UpdatesStaleSpecgraphEntry`,
`TestSync_PreservesUserCustomizationsUnderSpecgraph` all pass.

- [ ] **Step 5: Run `task check`**

```bash
task check
```

Expected: exit 0.

- [ ] **Step 6: Pre-commit ritual + commit**

```bash
jj --no-pager status
```

Expected: `A internal/config/mcpconfigs/sync.go`,
`A internal/config/mcpconfigs/sync_test.go`.

```bash
jj --no-pager commit internal/config/mcpconfigs/sync.go internal/config/mcpconfigs/sync_test.go -m "feat(mcpconfigs): Sync applies per-harness merge patches idempotently

Adds Sync(projectDir, configs) and SyncResult to internal/config/mcpconfigs.
Sync iterates the ManagedConfigs and for each: reads existing file (or
treats missing as empty), applies the RFC 7396 merge patch via
evanphx/json-patch/v5, canonicalizes via json.MarshalIndent + trailing
newline, and writes only when the canonical bytes differ from the existing.

Per-file action reported via SyncResult.Action: 'created', 'updated', or
'no-op'.

Special: for opencode.json, refuses if opencode.jsonc sibling exists.
OpenCode is the only managed harness whose docs reference both .json and
.jsonc as supported config formats; lookup precedence isn't documented
for project-root, so writing .json next to a user's .jsonc could leave
either file as the active one. Cursor and Claude Code MCP docs don't
mention .jsonc; no equivalent check for them.

Tests cover: file-missing → created with canonical content; existing file
with another server → siblings preserved alongside our entry; existing
file with stale specgraph entry → managed fields updated to canonical
values, sibling servers preserved; existing file with user fields under
specgraph (custom headers, comments) → user fields preserved while
managed fields move to canonical.

Error-path and idempotence tests land in a subsequent commit.

spgr-7htb

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 7: Verify clean**

---

## Task 4: `mcpconfigs.Sync` — error paths and idempotence

**Files:**

- Modify: `internal/config/mcpconfigs/sync_test.go`

This task adds the remaining test coverage for `Sync`: opencode.jsonc
sibling rejection, malformed JSON rejection, and the run-twice idempotence
guarantee. No new production code; the existing `sync.go` already handles
these paths.

- [ ] **Step 1: Insert the new tests in `sync_test.go`**

Insert the following four test functions in `internal/config/mcpconfigs/sync_test.go`
**immediately before** the helper functions (`actionFor`, `readJSON`,
`assertFileEquals`) so the file's structure stays "tests first, helpers
last":

```go
func TestSync_RefusesOnOpencodeJSONCSibling(t *testing.T) {
	dir := t.TempDir()
	jsoncPath := filepath.Join(dir, "opencode.jsonc")
	if err := os.WriteFile(jsoncPath, []byte(`{"mcp":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	wantSubstr := "opencode.jsonc"
	if got := err.Error(); !strings.Contains(got, wantSubstr) {
		t.Errorf("error %q does not contain %q", got, wantSubstr)
	}

	// Sync stops at OpenCode (the third config); the first two (Cursor,
	// Claude Code) should have completed before the error.
	gotPaths := []string{}
	for _, r := range results {
		gotPaths = append(gotPaths, r.Path)
	}
	wantPaths := []string{".cursor/mcp.json", ".mcp.json"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Errorf("partial results paths = %v, want %v", gotPaths, wantPaths)
	}
}

func TestSync_RefusesOnMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cursorPath, []byte(`{not valid json`), 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "parse") || !strings.Contains(got, ".cursor/mcp.json") {
		t.Errorf("error %q should mention parse failure for .cursor/mcp.json", got)
	}
	if len(results) != 0 {
		t.Errorf("results = %v, want empty (Sync stopped at first config)", results)
	}

	// File should be untouched.
	got, err := os.ReadFile(cursorPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{not valid json` {
		t.Errorf("file was modified: %q", got)
	}
}

func TestSync_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")

	// Run 1: all three files created.
	results1, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 1: %v", err)
	}
	for _, r := range results1 {
		if r.Action != "created" {
			t.Errorf("run 1 %s: action = %q, want %q", r.Path, r.Action, "created")
		}
	}

	// Snapshot bytes after run 1.
	snapshots := map[string][]byte{}
	for _, c := range configs {
		data, err := os.ReadFile(filepath.Join(dir, c.Path))
		if err != nil {
			t.Fatal(err)
		}
		snapshots[c.Path] = data
	}

	// Run 2: all three should be no-ops, file bytes unchanged.
	results2, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 2: %v", err)
	}
	for _, r := range results2 {
		if r.Action != "no-op" {
			t.Errorf("run 2 %s: action = %q, want %q", r.Path, r.Action, "no-op")
		}
	}
	for _, c := range configs {
		got, err := os.ReadFile(filepath.Join(dir, c.Path))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, snapshots[c.Path]) {
			t.Errorf("%s: file bytes changed between run 1 and run 2", c.Path)
		}
	}
}

func TestSync_Idempotent_ReformatsThenStable(t *testing.T) {
	// Existing file is valid JSON but not in canonical 2-space-indent form.
	// Run 1 should rewrite it (action "updated"); run 2 should be no-op.
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// 4-space-indent variant of the canonical specgraph entry — semantically
	// equivalent, format different.
	existing := []byte(`{
    "mcpServers": {
        "specgraph": {
            "url": "http://127.0.0.1:7890/mcp/",
            "headers": {
                "Authorization": "Bearer ${env:SPECGRAPH_API_KEY}",
                "X-Specgraph-Project": "specgraph"
            }
        }
    }
}
`)
	if err := os.WriteFile(cursorPath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results1, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 1: %v", err)
	}
	if got := actionFor(results1, ".cursor/mcp.json"); got != "updated" {
		t.Errorf("run 1 .cursor/mcp.json: action = %q, want %q (format normalization)", got, "updated")
	}

	results2, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 2: %v", err)
	}
	if got := actionFor(results2, ".cursor/mcp.json"); got != "no-op" {
		t.Errorf("run 2 .cursor/mcp.json: action = %q, want %q (already canonical)", got, "no-op")
	}
}
```

Also extend the import block at the top of `sync_test.go` to include
`"bytes"` and `"strings"` (Task 3's test file imports `encoding/json`,
`os`, `path/filepath`, `reflect`, `sort`, `testing`; the new tests need
`bytes` for the idempotence byte comparison and `strings` for substring
matching).

- [ ] **Step 2: Run the new tests**

```bash
go test ./internal/config/mcpconfigs/ -run TestSync -v
```

Expected: all eight `TestSync_*` tests pass (the four from Task 3 plus the
four added here).

- [ ] **Step 3: Run full package tests**

```bash
go test ./internal/config/mcpconfigs/ -v -race
```

Expected: all tests pass under `-race`.

- [ ] **Step 4: Run `task check`**

```bash
task check
```

Expected: exit 0.

- [ ] **Step 5: Pre-commit ritual + commit**

```bash
jj --no-pager status
```

Expected: `M internal/config/mcpconfigs/sync_test.go`.

```bash
jj --no-pager commit internal/config/mcpconfigs/sync_test.go -m "test(mcpconfigs): Sync error paths and idempotence

Adds four test cases covering the parts of Sync that the happy-path tests
in the prior commit didn't reach:

- TestSync_RefusesOnOpencodeJSONCSibling: opencode.jsonc next to
  opencode.json triggers refusal with a path-bearing error message;
  partial results from earlier configs (Cursor, Claude Code) are still
  returned alongside the error so the caller can report what was done.
- TestSync_RefusesOnMalformedJSON: invalid JSON in an existing managed
  file triggers refusal; the file is not modified; results is empty
  (Sync stops at the first failing config, which is .cursor/mcp.json).
- TestSync_Idempotent: running Sync twice in succession against an empty
  directory produces 'created' on run 1 and 'no-op' on run 2, with
  byte-equal file contents between runs.
- TestSync_Idempotent_ReformatsThenStable: starting from a valid but
  non-canonical (4-space-indent) file, run 1 reports 'updated' (format
  normalization), run 2 reports 'no-op' (file is now canonical).

No production code changes; the existing sync.go already handles these
paths via the canonicalize + bytes.Equal short-circuit and the JSONC
sibling check.

spgr-7htb

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 6: Verify clean**

---

## Task 5: Idempotent `runInit` — drop `runUp`, add slug check, call `Sync`

**Files:**

- Modify: `cmd/specgraph/init.go`

This task rewrites `runInit` for the new contract. The existing function:

1. Calls `runUp` with a warning on failure.
2. Errors out if `.specgraph.yaml` already exists.
3. Derives slug from arg or git remote.
4. Writes `.specgraph.yaml`.

The new function:

1. Resolves project state (existing `.specgraph.yaml` or fresh).
2. Validates slug-arg-vs-existing consistency.
3. Writes `.specgraph.yaml` only if it doesn't exist (idempotent).
4. Resolves server URL via `(*GlobalConfig).ResolveServer`.
5. Calls `mcpconfigs.Sync` and prints per-file actions.

Note: the test file `init_test.go` modifications land in Task 6.

- [ ] **Step 1: Read the current `runInit`**

```bash
sed -n '28,63p' cmd/specgraph/init.go
```

Expected: prints the current 36-line `runInit` for reference.

- [ ] **Step 2: Rewrite `cmd/specgraph/init.go`**

Replace the entire file with:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/config/mcpconfigs"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project-slug]",
	Short: "Initialize a SpecGraph project in the current directory",
	Long: "Writes .specgraph.yaml and the per-harness MCP config files " +
		"(.cursor/mcp.json, .mcp.json, opencode.json) for the current project. " +
		"Idempotent: safe to re-run on an already-initialized project; managed " +
		"fields are reset to canonical values, user-added fields are preserved.",
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

var initYes bool

func init() {
	initCmd.Flags().BoolVar(&initYes, "yes", false, "non-interactive (accepted for backward compat; init is always non-interactive)")
	rootCmd.AddCommand(initCmd)
}

func runInit(_ *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	var argSlug string
	if len(args) > 0 {
		argSlug = args[0]
	}

	// Resolve project state: load existing .specgraph.yaml if present.
	var existing *config.ProjectConfig
	if root, findErr := config.FindProjectRoot(cwd); findErr == nil {
		loaded, loadErr := config.LoadProject(root)
		if loadErr != nil {
			return fmt.Errorf("load existing project config: %w", loadErr)
		}
		existing = loaded
		cwd = root
	} else if !errors.Is(findErr, config.ErrProjectNotFound) {
		return fmt.Errorf("find project root: %w", findErr)
	}

	// Slug-consistency check: if both an arg and an existing config are
	// present and the slugs differ, refuse. The slug is identity-defining
	// (storage partition key, X-Specgraph-Project header value) and silent
	// mutation would orphan project data.
	if argSlug != "" && existing != nil && argSlug != existing.Slug {
		return fmt.Errorf(
			"cannot change project slug from %q to %q; edit .specgraph.yaml directly or remove it",
			existing.Slug, argSlug,
		)
	}

	// Determine the slug for this run.
	var pc *config.ProjectConfig
	switch {
	case existing != nil:
		pc = existing
	case argSlug != "":
		pc = &config.ProjectConfig{Slug: argSlug}
	default:
		// Derive from git remote / dir name (config.LoadProject already does
		// this when no .specgraph.yaml exists).
		derived, derErr := config.LoadProject(cwd)
		if derErr != nil {
			return fmt.Errorf("derive project slug: %w", derErr)
		}
		pc = &config.ProjectConfig{Slug: derived.Slug}
	}

	// Write .specgraph.yaml only if it doesn't exist; idempotent.
	if existing == nil {
		if writeErr := config.WriteProject(cwd, pc); writeErr != nil {
			return fmt.Errorf("write project config: %w", writeErr)
		}
		fmt.Printf("Initialized project %s. Config written to .specgraph.yaml\n", pc.Slug)
	}

	// Resolve server URL via global config + project override.
	globalCfg, err := loadGlobalCfg()
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}
	serverURL := globalCfg.ResolveServer(pc.Slug, pc.Server)

	// Sync the per-harness MCP configs.
	configs := mcpconfigs.ManagedConfigs(pc.Slug, serverURL)
	results, syncErr := mcpconfigs.Sync(cwd, configs)
	for _, r := range results {
		fmt.Printf("%s: %s\n", r.Path, r.Action)
	}
	if syncErr != nil {
		return fmt.Errorf("sync mcp configs: %w", syncErr)
	}

	return nil
}
```

Note the import additions: `errors`, `github.com/specgraph/specgraph/internal/config/mcpconfigs`.
Removed: the implicit dependency on `runUp` (no import change required since
`runUp` is in the same package).

- [ ] **Step 3: Verify the file builds**

```bash
go build ./cmd/specgraph/
```

Expected: completes without error.

- [ ] **Step 4: Run all existing tests in the package to confirm nothing else broke**

```bash
go test ./cmd/specgraph/ -v -race -run TestInit
```

Expected: any pre-existing init test should still pass IF it didn't depend
on the removed behavior. New tests land in Task 6. If pre-existing tests
fail because they assume the "already initialized" error, treat the failure
as expected for now and update them in Task 6.

- [ ] **Step 5: Run `task check`**

```bash
task check
```

Expected: exit 0. If license:check flags `init.go`, fix the header — but
the existing header (`SPDX first, Copyright Sean Brandt`) should already
be acceptable; the rewrite preserves it.

- [ ] **Step 6: Pre-commit ritual + commit**

```bash
jj --no-pager status
```

Expected: `M cmd/specgraph/init.go`.

```bash
jj --no-pager commit cmd/specgraph/init.go -m "feat(cli): make 'specgraph init' idempotent and config-only

Rewrites runInit so that init can be safely run as often as the user wants;
each run converges to a function of .specgraph.yaml + global config.

Behavior changes vs the previous runInit:

- Drops the leading runUp call. The previous init brought up the Docker
  compose stack as a side effect, blocking up to ~10 seconds polling
  container health and producing visible Docker output. That is surprising
  for a command intended to be re-runnable. Users who need the server
  brought up should run 'specgraph up' separately.
- Drops the 'project already initialized' error. .specgraph.yaml is treated
  as the source of truth; if it exists, init reads the slug from it and
  proceeds to sync the per-harness MCP configs.
- Adds a slug-consistency check: if an arg is passed and an existing
  .specgraph.yaml has a different slug, refuse with a clear error
  ('cannot change project slug from X to Y; edit .specgraph.yaml directly
  or remove it'). The slug is identity-defining; silent mutation would
  orphan project data on the server.
- Adds the call to mcpconfigs.Sync after .specgraph.yaml resolution. Each
  per-file action ('created' / 'updated' / 'no-op') is printed to stdout.

Test coverage for the slug × existing matrix lands in the next commit.

spgr-7htb

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 7: Verify clean**

---

## Task 6: Init integration tests

**Files:**

- Modify: `cmd/specgraph/init_test.go`

Adds tests covering the slug × existing matrix and idempotence at the
`runInit` level. The existing `init_test.go` likely has tests for the old
behavior; this task may replace some of them.

- [ ] **Step 1: Read the current `init_test.go`**

```bash
cat cmd/specgraph/init_test.go
```

Read carefully and identify which existing tests are still valid under the
new behavior, which need updating, and which need to be removed (e.g., a
test asserting the old "project already initialized" error must go).

- [ ] **Step 2: Replace `cmd/specgraph/init_test.go`**

Replace the entire file with the test suite below. Keep the original file's
license header style. If existing tests not covered here are still valuable
(e.g., slug-derivation edge cases), preserve them at the bottom of the new
file rather than deleting:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInitInDir executes runInit with the given args in workDir, capturing
// stdout. Isolates the global config file (cfgFile package var) and CWD,
// restoring them on cleanup.
func runInitInDir(t *testing.T, workDir string, args []string) (stdout string, err error) {
	t.Helper()

	// Isolate cfgFile so loadGlobalCfg() doesn't touch the developer's
	// real ~/.config/specgraph/config.yaml. Mirrors the pattern in
	// lifecycle_test.go and other tests in this package.
	//
	// We write an empty YAML file rather than just naming a path: when
	// cfgFile != "" loadGlobalCfg dispatches to config.LoadGlobalExplicit,
	// which errors with "config file not found" if the path doesn't exist.
	// An empty YAML body parses to globalDefaults() (DefaultServer
	// http://127.0.0.1:9090), which is fine for these tests.
	oldCfgFile := cfgFile
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if writeErr := os.WriteFile(cfgPath, []byte(""), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = oldCfgFile })

	origDir, dirErr := os.Getwd()
	if dirErr != nil {
		t.Fatal(dirErr)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Capture stdout. Restore inline before reading from the pipe so
	// downstream test helpers see normal stdout.
	origStdout := os.Stdout
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatal(perr)
	}
	os.Stdout = w

	runErr := runInit(initCmd, args)

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Fatal(copyErr)
	}

	return buf.String(), runErr
}

func TestRunInit_FreshProject_NoArg(t *testing.T) {
	dir := t.TempDir()
	out, err := runInitInDir(t, dir, nil)
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}
	// .specgraph.yaml exists.
	if _, statErr := os.Stat(filepath.Join(dir, ".specgraph.yaml")); statErr != nil {
		t.Errorf(".specgraph.yaml not created: %v", statErr)
	}
	// All three configs exist.
	for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
		if _, statErr := os.Stat(filepath.Join(dir, p)); statErr != nil {
			t.Errorf("%s not created: %v", p, statErr)
		}
		if !strings.Contains(out, p+": created") {
			t.Errorf("stdout missing %q: %s", p+": created", out)
		}
	}
}

func TestRunInit_FreshProject_WithArg(t *testing.T) {
	dir := t.TempDir()
	out, err := runInitInDir(t, dir, []string{"explicit-slug"})
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}
	// .specgraph.yaml has the explicit slug.
	data, readErr := os.ReadFile(filepath.Join(dir, ".specgraph.yaml"))
	if readErr != nil {
		t.Fatalf("read .specgraph.yaml: %v", readErr)
	}
	if !strings.Contains(string(data), "explicit-slug") {
		t.Errorf(".specgraph.yaml missing slug; content: %s", data)
	}
	if !strings.Contains(out, "Initialized project explicit-slug") {
		t.Errorf("stdout missing init message: %s", out)
	}
}

func TestRunInit_ExistingProject_NoArg_NoOp(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"my-project"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	out, err := runInitInDir(t, dir, nil)
	if err != nil {
		t.Fatalf("second runInit: %v", err)
	}
	// All three configs report no-op on the second run.
	for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
		if !strings.Contains(out, p+": no-op") {
			t.Errorf("stdout missing %q on idempotent re-run: %s", p+": no-op", out)
		}
	}
	// Should NOT print the "Initialized project" message on the second run.
	if strings.Contains(out, "Initialized project") {
		t.Errorf("stdout has unexpected init message on re-run: %s", out)
	}
}

func TestRunInit_ExistingProject_MatchingArg_NoOp(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"my-project"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	out, err := runInitInDir(t, dir, []string{"my-project"})
	if err != nil {
		t.Fatalf("second runInit with matching arg: %v", err)
	}
	for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
		if !strings.Contains(out, p+": no-op") {
			t.Errorf("stdout missing %q: %s", p+": no-op", out)
		}
	}
}

func TestRunInit_ExistingProject_ConflictingArg_Refuses(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"original-slug"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	_, err := runInitInDir(t, dir, []string{"different-slug"})
	if err == nil {
		t.Fatal("expected slug-conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot change project slug") {
		t.Errorf("error %q should mention slug change", err.Error())
	}
	// .specgraph.yaml should still hold the original slug.
	data, readErr := os.ReadFile(filepath.Join(dir, ".specgraph.yaml"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(data), "original-slug") {
		t.Errorf(".specgraph.yaml mutated; content: %s", data)
	}
	if strings.Contains(string(data), "different-slug") {
		t.Errorf(".specgraph.yaml gained the conflicting slug; content: %s", data)
	}
}

func TestRunInit_Idempotent_ByteEqualSecondRun(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"my-project"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	// Snapshot all three managed config files after run 1.
	snaps := map[string][]byte{}
	for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
		data, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil {
			t.Fatal(err)
		}
		snaps[p] = data
	}

	// Run 2: should be no-op for all configs; bytes unchanged.
	if _, err := runInitInDir(t, dir, nil); err != nil {
		t.Fatalf("second runInit: %v", err)
	}
	for p, want := range snaps {
		got, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: bytes changed between idempotent runs", p)
		}
	}
}
```

- [ ] **Step 3: Run init tests**

```bash
go test ./cmd/specgraph/ -v -race -run TestRunInit
```

Expected: 6 tests pass: `TestRunInit_FreshProject_NoArg`,
`TestRunInit_FreshProject_WithArg`, `TestRunInit_ExistingProject_NoArg_NoOp`,
`TestRunInit_ExistingProject_MatchingArg_NoOp`,
`TestRunInit_ExistingProject_ConflictingArg_Refuses`,
`TestRunInit_Idempotent_ByteEqualSecondRun`.

- [ ] **Step 4: Run all `cmd/specgraph` tests**

```bash
go test ./cmd/specgraph/ -v -race
```

Expected: all package tests pass.

- [ ] **Step 5: Run `task check`**

```bash
task check
```

Expected: exit 0.

- [ ] **Step 6: Pre-commit ritual + commit**

```bash
jj --no-pager status
```

Expected: `M cmd/specgraph/init_test.go`.

```bash
jj --no-pager commit cmd/specgraph/init_test.go -m "test(cli): runInit slug × existing matrix + idempotence

Adds tests covering the new idempotent runInit:

- Fresh project, no slug arg: derives slug, writes .specgraph.yaml, syncs
  all three managed configs ('created' actions).
- Fresh project, with slug arg: uses arg, writes .specgraph.yaml with that
  slug, syncs.
- Existing project, no slug arg: re-uses existing slug, all three configs
  report 'no-op' (no managed-field changes).
- Existing project, matching slug arg: same as no-arg path.
- Existing project, conflicting slug arg: refuses with 'cannot change
  project slug' error; .specgraph.yaml is not mutated.
- Idempotence at the runInit level: byte-equal config files between two
  successive runs.

Tests use t.TempDir() and os.Pipe() to capture stdout cleanly.

spgr-7htb

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 7: Verify clean**

---

## Task 7: Documentation updates

**Files:**

- Modify: `CLAUDE.md`
- Modify: `docs/verification/cursor.md`
- Modify: `docs/verification/opencode.md`

Reflects the new init contract in the contributor and verification docs.
The committed example mcp.json files at the repo root remain hand-written
until Task 38 (dogfood cutover); the verification docs note that init now
generates them.

- [ ] **Step 1: Update `CLAUDE.md`**

Locate the "Documentation" section (or the most appropriate location in
the existing doc) and add a new bullet near the existing "Plugin" entry:

Find this block (under `## Documentation`):

```markdown
- **Plugin** — `plugin/specgraph/` is the thin Claude Code plugin: `.claude-plugin/plugin.json`, `hooks/session-start.sh` (reads `specgraph://prime` via MCP using the `specgraph read-mcp-resource` CLI subcommand), and `routing-guide.md` (stable meta-knowledge routing for the LLM). The previous 13-skill layout is retired; see `docs/plans/2026-04-20-multi-platform-plugin-design.md`.
```

Add after it:

```markdown
- **Per-harness MCP configs** — `.cursor/mcp.json` (Cursor), `.mcp.json` (Claude Code), and `opencode.json` (OpenCode) are managed by `specgraph init`. Running init for a fresh project writes them; re-running on an existing project updates managed fields (URL, Authorization header, X-Specgraph-Project header, harness-specific shape) and preserves siblings + user-added fields under the `specgraph` entry. See `internal/config/mcpconfigs/` for the writer code and `docs/plans/2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md` for the design rationale.
```

- [ ] **Step 2: Update `docs/verification/cursor.md`**

Find the section header `## Setup that worked` and the subsection
`### 2. Project-level \`.cursor/mcp.json\` (committed)`. Add a note at the
top of that subsection (just below the header):

```markdown
> **Note (post-spgr-7htb):** This file is now generated by `specgraph init`
> from `.specgraph.yaml` + global config; running init updates managed fields
> (URL, Authorization header, X-Specgraph-Project) while preserving siblings
> and user customizations. The committed example below is hand-maintained
> until Task 38 (dogfood cutover) when this repo gets `specgraph init`-ed
> for real.
```

- [ ] **Step 3: Update `docs/verification/opencode.md`**

Find the equivalent `### 2. Project-level \`opencode.json\` (committed)`
section. Add the same note (with the path adapted):

```markdown
> **Note (post-spgr-7htb):** This file is now generated by `specgraph init`
> from `.specgraph.yaml` + global config; running init updates managed fields
> (URL, Authorization header, X-Specgraph-Project) while preserving siblings
> and user customizations. The committed example below is hand-maintained
> until Task 38 (dogfood cutover) when this repo gets `specgraph init`-ed
> for real.
```

- [ ] **Step 4: Run markdown lint**

```bash
task lint:markdown
```

Expected: exit 0; no rumdl issues.

- [ ] **Step 5: Run `task check` (full pipeline)**

```bash
task check
```

Expected: exit 0.

- [ ] **Step 6: Pre-commit ritual + commit**

```bash
jj --no-pager status
```

Expected: `M CLAUDE.md`, `M docs/verification/cursor.md`,
`M docs/verification/opencode.md`.

```bash
jj --no-pager commit CLAUDE.md docs/verification/cursor.md docs/verification/opencode.md -m "docs: 'specgraph init' now manages per-harness MCP configs

Documents the new init responsibility in the contributor (CLAUDE.md) and
verification (docs/verification/{cursor,opencode}.md) docs.

CLAUDE.md gains a new bullet under Documentation explaining the
mcpconfigs package and the managed-field semantics. The verification
docs gain a note in their 'project-level config' section flagging that
the file is now generated and pointing at Task 38 (dogfood cutover) for
when this repo's hand-committed examples will be replaced by init
output.

spgr-7htb

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 7: Verify clean**

---

## Task 8: `task pr-prep` and push

**Files:** none

This task runs the full pre-PR gate, pushes the branch, and opens the PR.

- [ ] **Step 1: Run `task pr-prep`**

Note: per CLAUDE.md gotchas, `task clean` flakes on macOS against pnpm
node_modules. If pr-prep fails on the clean step, manually run
`rm -rf web/node_modules` first and retry.

```bash
task pr-prep
```

Expected: exit 0. If a non-clean failure surfaces, debug and fix before
proceeding. (If the clean-step flake hits, that's a known Taskfile
fragility filed as `spgr-rt2m`.)

- [ ] **Step 2: Switch gh auth + push**

```bash
gh auth switch -u seanb4t -h github.com
jj --no-pager bookmark set feat/spgr-7htb-init-mcp-configs -r @-
jj --no-pager git push --bookmark feat/spgr-7htb-init-mcp-configs
```

Expected: push completes. Output includes a `Create a pull request for
'feat/spgr-7htb-init-mcp-configs' on GitHub by visiting:` URL.

- [ ] **Step 3: Open the PR**

```bash
cd ~/Code/github.com/specgraph/
gh pr create --head feat/spgr-7htb-init-mcp-configs --base main \
  --title "spgr-7htb: idempotent 'specgraph init' with managed per-harness MCP configs" \
  --body "$(cat <<'EOF'
## Summary

Closes `spgr-7htb`. Makes `specgraph init` idempotent and config-only (no Docker startup), and adds the responsibility of generating `.cursor/mcp.json`, `.mcp.json`, and `opencode.json` from `.specgraph.yaml` + global config via JSON Merge Patch (RFC 7396).

## What's in this PR (7 commits)

1. `chore(deps)` — add `github.com/evanphx/json-patch/v5`.
2. `feat(mcpconfigs)` — `ManagedConfigs(slug, serverURL)` and per-harness builders. Pure render, golden-file tests.
3. `feat(mcpconfigs)` — `Sync` happy paths: create missing files, preserve other servers, update stale specgraph entries, preserve user customizations.
4. `test(mcpconfigs)` — `Sync` error paths and idempotence: refuse on `opencode.jsonc` sibling, refuse on malformed JSON, byte-equal idempotence, format normalization.
5. `feat(cli)` — rewrite `runInit`: drop `runUp`, drop "already initialized" error, add slug-consistency check, call `mcpconfigs.Sync`.
6. `test(cli)` — `runInit` slug × existing matrix + idempotence.
7. `docs` — CLAUDE.md and verification docs reflect the new init contract.

## Design and decisions

See `docs/plans/2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md` and the companion `-plan.md`. Key decisions: idempotent single command (no separate `configs sync`), JSON Merge Patch via `evanphx/json-patch/v5`, managed fields = url + Authorization + X-Specgraph-Project + harness-specific shape, per-developer URL overrides go to user-level harness config, slug-conflict refuses, `opencode.jsonc` sibling refuses (OpenCode-only check).

## Out of scope (deferred)

- Codex MCP config (Task 36 deferred — no work-machine access).
- `--force` flag for slug-conflict / JSONC overrides.
- Drift detection / `specgraph configs status`.
- Migrating this repo's currently-committed `.cursor/mcp.json`, `.mcp.json`, `opencode.json` files — handled by Task 38 (dogfood cutover).

## Test plan

- [x] `task check` passes (fmt:check, license:check, lint, build, unit tests with -race)
- [x] `task pr-prep` passes (check + integration + e2e)
- [x] `go test ./internal/config/mcpconfigs/ -v -race` covers ManagedConfigs golden + Sync happy paths + Sync error paths + idempotence
- [x] `go test ./cmd/specgraph/ -run TestRunInit -v -race` covers fresh/existing × no-arg/with-arg/conflicting-arg matrix + run-twice idempotence
- [ ] Manual smoke test against this repo: NOT performed (this repo intentionally not init'd until Task 38)

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Expected: `gh pr create` returns the PR URL.

- [ ] **Step 4: Update bead `spgr-7htb` and `spgr-bncv` notes**

```bash
bd update spgr-7htb --status in_progress
```

(The bead is closed when the PR merges; for now mark in-progress.)

- [ ] **Step 5: Watch CI and address findings**

Monitor `gh pr checks <pr-number>`. If any check fails, debug; if
CodeRabbit posts comments, evaluate them per the PR #927 lesson (verify
empirically before accepting suggestions).

---

## Success criteria

When this plan is complete:

1. `task check` passes from the worktree root.
2. `task pr-prep` passes (check + test:integration + test:e2e).
3. `go test ./internal/config/mcpconfigs/ -v -race` passes with all tests:
   `TestEnsureMCPSuffix`, `TestManagedConfigs_*` (5), `TestSync_*` (8 covering
   create / preserve-siblings / update-stale / preserve-user-fields /
   refuse-jsonc-sibling / refuse-malformed-json / idempotent /
   reformats-then-stable).
4. `go test ./cmd/specgraph/ -run TestRunInit -v -race` passes with all
   matrix tests.
5. `jj --no-pager log -r 'main..@-' --no-graph` shows seven new commits.
6. PR is opened on `feat/spgr-7htb-init-mcp-configs` with all 7 commits.
7. Working copy `@` is empty after the final commit.

---

## What this plan does NOT cover

- Codex MCP config support (deferred until Task 36 resumes — no
  work-machine access today).
- `--force` flag for manual override of slug-conflict / JSONC errors.
- Drift detection / `specgraph configs status` diff command.
- Auto-running `init` in CI / post-merge hook.
- Migrating the SpecGraph repo's currently-committed `.cursor/mcp.json`,
  `.mcp.json`, and `opencode.json` files to init-generated outputs —
  handled by Task 38 (dogfood cutover).
- Comment-preserving merge for `opencode.jsonc` — Phase 2 work; init
  refuses for now if the sibling file exists.
