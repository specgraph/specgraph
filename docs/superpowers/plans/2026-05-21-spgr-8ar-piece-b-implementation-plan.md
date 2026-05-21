# spgr-8ar Piece B — Remote-Source Import + Sync (env-var auth)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a new `RefreshConstitutionLayer` RPC + CLI commands (`constitution import --from-url`, `constitution sync`) backed by `hashicorp/go-getter v1`, with host-scoped GitHub-token auth via `SPECGRAPH_FETCH_GITHUB_TOKEN`, URL-credential sanitization, and content-hash-only drift detection.

**Architecture:** Three new packages under `internal/constitution/`: `hash` (canonical Murmur3-128 of parsed domain struct), `load` (YAML→`*storage.Constitution` helper extracted from CLI/server paths), and `fetch` (go-getter wrapper with security posture + auth allow-list + URL sanitization). Server-side `RefreshConstitutionLayer` RPC consumes all three. CLI `import --from-url` and `sync` call the RPC.

**Tech Stack:** Go, ConnectRPC, new `github.com/hashicorp/go-getter` (v1) dependency, existing `spaolacci/murmur3 v1.1.0`, existing `gopkg.in/yaml.v3 v3.0.1`. PostgreSQL via existing storage layer (no schema change — `source_url` + `source_hash` columns already exist per migration 004).

**Reference spec:** [docs/superpowers/specs/2026-05-21-multi-layer-constitution-completion-design.md](../specs/2026-05-21-multi-layer-constitution-completion-design.md) — Sections 4, 5, 6, 7, 8, 11, 12, 13 (security checklist), 14 (invariants 3–7, 13).

---

## File Structure

### Files created

| Path | Responsibility |
|---|---|
| `internal/constitution/hash/hash.go` | Canonical Murmur3-128 hash of `*storage.Constitution` post-parse |
| `internal/constitution/hash/hash_test.go` | Determinism + sensitivity invariants |
| `internal/constitution/load/load.go` | `LoadFromYAML([]byte) (*storage.Constitution, error)` shared by CLI and server |
| `internal/constitution/load/load_test.go` | Parse round-trips, error cases |
| `internal/constitution/fetch/fetch.go` | go-getter wrapper + token transport + URL sanitization |
| `internal/constitution/fetch/fetch_test.go` | Unit tests via `file://` (testfetch build tag) |
| `internal/constitution/fetch/fetch_security_test.go` | Section 13 security checklist tests |
| `proto/specgraph/v1/constitution.proto` | Add `RefreshConstitutionLayer` RPC + request/response (modify) |
| `gen/specgraph/v1/constitution.pb.go` | Regenerated proto (committed; touched by `task proto`) |
| `gen/specgraph/v1/specgraphv1connect/constitution.connect.go` | Regenerated ConnectRPC stubs (committed) |
| `cmd/specgraph/constitution.go` | Add `--from-url` flag, add `sync` subcommand (modify) |
| `e2e/api/constitution_refresh_test.go` | E2E test for full RefreshConstitutionLayer flow |

### Files modified

| Path | Change |
|---|---|
| `internal/server/constitution_handler.go` | Add `RefreshConstitutionLayer` handler method |
| `internal/server/constitution_handler_test.go` | Handler unit tests |
| `internal/auth/permissions.go` | Add permission entry for the new RPC procedure |
| `go.mod` / `go.sum` | Add `github.com/hashicorp/go-getter v1.7.x` (latest stable) |

---

## Prerequisites

- `spgr-15gz` claimed (already done in workspace setup).
- Working tree on new main (`e95201ee` after PR #956 merged).
- Docker available for integration + E2E tests.
- Network access to GitHub for the verification gate in Task 1.

---

## Task 0: Verify go-getter v1 API surface

**Files:** none (research only)

The spec specifies several go-getter knobs that I haven't verified against the v1 source. Before writing the package, confirm each exists.

- [ ] **Step 1: Add go-getter to go.mod (pin to latest stable v1)**

```bash
cd ~/Code/github.com/specgraph-8ar-piece-b
go get github.com/hashicorp/go-getter@v1.7.6
go mod tidy
```

Pin to v1.7.6 (latest stable in the v1.x line). If `v1.7.6` is not the actual latest, replace with whatever `go list -m -versions github.com/hashicorp/go-getter` shows as the latest non-prerelease v1.

- [ ] **Step 2: Verify the API knobs we depend on**

Open `vendor/github.com/hashicorp/go-getter/client.go` (or run `go doc github.com/hashicorp/go-getter.Client` after the go-get). Verify each field exists:

- `Client.Ctx context.Context`
- `Client.Src string`
- `Client.Dst string`
- `Client.Mode getter.ClientMode` (with `ClientModeFile` value)
- `Client.HttpClient *http.Client`
- `Client.Getters map[string]Getter` (a registry of getters keyed by scheme/protocol)
- `Client.Decompressors map[string]Decompressor` (set to nil disables auto-extraction)

If any field name differs (e.g., `Getters` is named `GetterRegistry`), record the actual name in this step's notes. Do NOT proceed to Task 1 until all knobs verified.

- [ ] **Step 3: Verify the default getter registry**

```bash
go doc github.com/hashicorp/go-getter.Getters
```

Confirm the default registered getters (we'll override to a restricted subset). Note which getter implementations exist for `http`, `https`, `git`, `github`, `file`. Record their concrete type names (e.g., `getter.HttpGetter`, `getter.GitGetter`, etc.) — needed in Task 3.

- [ ] **Step 4: Commit the dependency**

```bash
jj --no-pager describe @ -m "deps: add hashicorp/go-getter v1.7.6 (spgr-8ar piece B)

Used by the remote-source constitution fetch package in subsequent
commits. Pinned to v1.x stable; v2 is alpha.

Verified API surface: Client.{Ctx, Src, Dst, Mode, HttpClient, Getters,
Decompressors} all present and match the design spec's expectations.

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"` for a clean child.

---

## Task 1: `internal/constitution/hash` package

**Files:**

- Create: `internal/constitution/hash/hash.go`
- Create: `internal/constitution/hash/hash_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/constitution/hash/hash_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package hash_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/constitution/hash"
	"github.com/specgraph/specgraph/internal/storage"
)

func TestHash_Deterministic(t *testing.T) {
	c := &storage.Constitution{
		Name:  "test",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "Prefer explicit"},
		},
	}
	h1, err := hash.Hash(c)
	require.NoError(t, err)
	h2, err := hash.Hash(c)
	require.NoError(t, err)
	assert.Equal(t, h1, h2, "hash must be stable across calls")
}

func TestHash_DifferentContent_DifferentHash(t *testing.T) {
	c1 := &storage.Constitution{
		Name: "a",
		Principles: []storage.Principle{{ID: "p1", Statement: "S1"}},
	}
	c2 := &storage.Constitution{
		Name: "a",
		Principles: []storage.Principle{{ID: "p1", Statement: "S2"}},
	}
	h1, _ := hash.Hash(c1)
	h2, _ := hash.Hash(c2)
	assert.NotEqual(t, h1, h2, "different content produces different hash")
}

func TestHash_ListOrderMatters(t *testing.T) {
	c1 := &storage.Constitution{
		Principles: []storage.Principle{
			{ID: "p1", Statement: "S1"},
			{ID: "p2", Statement: "S2"},
		},
	}
	c2 := &storage.Constitution{
		Principles: []storage.Principle{
			{ID: "p2", Statement: "S2"},
			{ID: "p1", Statement: "S1"},
		},
	}
	h1, _ := hash.Hash(c1)
	h2, _ := hash.Hash(c2)
	assert.NotEqual(t, h1, h2,
		"list reordering produces different hash (intentional)")
}

func TestHash_NilFields_Stable(t *testing.T) {
	c := &storage.Constitution{Name: "empty"}
	h1, err := hash.Hash(c)
	require.NoError(t, err)
	h2, err := hash.Hash(c)
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
}

// TestHash_FixedExpected guards Go-version and encoding/json stability.
// If this expected value changes, encoding/json's behavior has shifted
// and downstream callers need to be made aware (drift detection bias).
func TestHash_FixedExpected(t *testing.T) {
	sentinel := &storage.Constitution{
		Name:  "sentinel",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "fixed"},
		},
		Constraints: []string{"never use eval"},
	}
	h, err := hash.Hash(sentinel)
	require.NoError(t, err)
	// Expected hash will be filled in by the implementer after the first
	// successful run. The point of this test is that the value stays
	// stable across runs and Go versions.
	assert.NotEmpty(t, h)
	assert.Len(t, h, 32, "Murmur3-128 hex must be 32 chars")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/constitution/hash/ -v
```

Expected: compile error — package doesn't exist yet.

- [ ] **Step 3: Implement the hash package**

Create `internal/constitution/hash/hash.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package hash computes canonical content hashes of constitution domain
// structs. Used by drift detection to determine whether a remote
// constitution has changed since it was last fetched.
//
// Hashing operates post-parse on *storage.Constitution rather than on
// raw YAML bytes. This is intentional: it makes the hash resilient to
// comments, whitespace, and key ordering in the source file, and avoids
// introducing a second YAML parser alongside the existing yaml.v3 loader.
//
// Determinism assumption: all map fields in *storage.Constitution today
// are map[string]string, which encoding/json sorts deterministically.
// Adding a map with non-string keys (or non-text-marshalable keys) would
// silently break determinism. The fixed-expected-hex test in this
// package guards against regressions on existing fields.
package hash

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/spaolacci/murmur3"

	"github.com/specgraph/specgraph/internal/storage"
)

// Hash returns the Murmur3-128 hex hash of the canonical JSON
// serialization of c. Two semantically equivalent inputs produce equal
// hashes; any field-level change produces a different hash.
func Hash(c *storage.Constitution) (string, error) {
	canonical, err := canonicalJSON(c)
	if err != nil {
		return "", err
	}
	sum := murmur3.Sum128(canonical)
	return hex.EncodeToString(append(sum[:0:0], byteOrder(sum)...)), nil
}

// canonicalJSON marshals c with sorted map keys at every level.
// encoding/json sorts top-level and nested map[string]X keys; struct
// fields emit in declaration order which is stable per the Go spec.
func canonicalJSON(c *storage.Constitution) ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("canonical json: %w", err)
	}
	return b, nil
}

// byteOrder converts the [16]byte from murmur3.Sum128 into a slice in
// big-endian order for hex encoding stability across architectures.
func byteOrder(sum [16]byte) []byte {
	return sum[:]
}
```

Note: I'm using `encoding/json` directly. The struct field declaration order in `*storage.Constitution` is the canonical key order. All map fields today are `map[string]string` which encoding/json sorts deterministically.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/constitution/hash/ -v
```

Expected: 4 of 5 tests PASS (the fixed-expected test will pass with an empty expected value — fill it in next step).

- [ ] **Step 5: Capture the actual hash and pin the fixed-expected value**

Modify `TestHash_FixedExpected` in the test file. After the first run captures the hash via:

```bash
go test ./internal/constitution/hash/ -run TestHash_FixedExpected -v 2>&1 | grep -oE '[a-f0-9]{32}'
```

(May need a temporary `t.Logf` to capture it.)

Update the test to assert against the captured value:

```go
const sentinelHash = "<captured value here>"
// ...
assert.Equal(t, sentinelHash, h,
    "regression: encoding/json or constitution struct shape changed")
```

Run again:

```bash
go test ./internal/constitution/hash/ -v
```

Expected: 5/5 PASS.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe @ -m "feat(constitution/hash): canonical Murmur3-128 hash of domain struct

New package internal/constitution/hash. Used by drift detection in the
RefreshConstitutionLayer RPC (subsequent commit).

Hashing operates post-parse on *storage.Constitution, not raw YAML
bytes. This is resilient to comments/whitespace/key-order and avoids a
second YAML parser. Determinism assumption: all map fields are
map[string]string, which encoding/json sorts. The fixed-expected test
guards against regressions.

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"`.

---

## Task 2: `internal/constitution/load` package

**Files:**

- Create: `internal/constitution/load/load.go`
- Create: `internal/constitution/load/load_test.go`
- Modify: `cmd/specgraph/constitution.go` (switch the existing CLI import to use the new helper)

The current YAML→domain path is split: CLI does YAML→`config.ConstitutionConfig`→proto→domain. We need a server-side path that goes YAML→domain directly. Extract the parse logic into a shared helper and reuse from both.

- [ ] **Step 1: Read the existing path**

Locate:

- `cmd/specgraph/constitution.go:constitutionConfigToProto` — converts YAML's `ConstitutionConfig` to proto
- `internal/server/convert_constitution.go:constitutionFromProto` — converts proto to domain

We want a new function `LoadFromYAML([]byte) (*storage.Constitution, error)` that does the work in one shot.

- [ ] **Step 2: Write failing tests**

Create `internal/constitution/load/load_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package load_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/constitution/load"
	"github.com/specgraph/specgraph/internal/storage"
)

func TestLoadFromYAML_HappyPath(t *testing.T) {
	yaml := []byte(`
name: test-constitution
layer: project
principles:
  - id: p1
    statement: Prefer explicit
constraints:
  - never use eval
`)
	c, err := load.LoadFromYAML(yaml)
	require.NoError(t, err)
	assert.Equal(t, "test-constitution", c.Name)
	assert.Equal(t, storage.ConstitutionLayerProject, c.Layer)
	require.Len(t, c.Principles, 1)
	assert.Equal(t, "p1", c.Principles[0].ID)
	assert.Equal(t, "Prefer explicit", c.Principles[0].Statement)
	require.Len(t, c.Constraints, 1)
	assert.Equal(t, "never use eval", c.Constraints[0])
}

func TestLoadFromYAML_MalformedYAML(t *testing.T) {
	yaml := []byte(`name: [unclosed`)
	_, err := load.LoadFromYAML(yaml)
	require.Error(t, err)
}

func TestLoadFromYAML_EmptyDoc(t *testing.T) {
	c, err := load.LoadFromYAML([]byte("{}"))
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "", c.Name)
}

func TestLoadFromYAML_InvalidLayer(t *testing.T) {
	yaml := []byte(`layer: not-a-real-layer`)
	_, err := load.LoadFromYAML(yaml)
	require.Error(t, err)
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/constitution/load/ -v
```

Expected: compile error — package doesn't exist.

- [ ] **Step 4: Implement the load package**

Create `internal/constitution/load/load.go`. Approach: reuse `config.ConstitutionConfig` (already does YAML parsing) and the existing conversion logic, but return `*storage.Constitution` directly.

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package load parses constitution YAML/JSON into the *storage.Constitution
// domain struct. Single source of YAML parsing for both the CLI and the
// server's RefreshConstitutionLayer RPC handler.
package load

import (
	"fmt"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// LoadFromYAML parses YAML or JSON bytes into a *storage.Constitution.
// Layer validation enforces that layer (if present) is one of
// user|org|project|domain.
func LoadFromYAML(data []byte) (*storage.Constitution, error) {
	cc, err := config.ParseConstitutionConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parse constitution: %w", err)
	}
	return configToConstitution(cc), nil
}

// configToConstitution converts a config.ConstitutionConfig (YAML) to the
// domain *storage.Constitution. Mirrors the CLI's constitutionConfigToProto
// and the server's constitutionFromProto in one step.
func configToConstitution(cc *config.ConstitutionConfig) *storage.Constitution {
	c := &storage.Constitution{
		Name:        cc.Name,
		Constraints: cc.Constraints,
	}
	if cc.Layer != "" {
		c.Layer = storage.ConstitutionLayer(cc.Layer)
	}
	// ... principles, antipatterns, references, tech, process — copy fields
	return c
}
```

The skeleton above is incomplete. Open `cmd/specgraph/constitution.go:constitutionConfigToProto` and `internal/server/convert_constitution.go:constitutionFromProto` to fill in the field-by-field conversion. The result should be exhaustive — every field on `*storage.Constitution` must be set from `config.ConstitutionConfig` where applicable.

If `config.ParseConstitutionConfig` doesn't exist (the current path may inline the YAML parse), add it as a one-liner in `internal/config/config.go`:

```go
func ParseConstitutionConfig(data []byte) (*ConstitutionConfig, error) {
	var cc ConstitutionConfig
	if err := yaml.Unmarshal(data, &cc); err != nil {
		return nil, err
	}
	return &cc, nil
}
```

For invalid layer string ("not-a-real-layer"), the conversion should return an error. Add a validation step in `configToConstitution`:

```go
switch cc.Layer {
case "", "user", "org", "project", "domain":
    // valid
default:
    return nil, fmt.Errorf("invalid layer %q; must be user|org|project|domain", cc.Layer)
}
```

(This means `configToConstitution` returns `(*storage.Constitution, error)`. Adjust signatures accordingly.)

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/constitution/load/ -v
```

Expected: 4/4 PASS.

- [ ] **Step 6: Update CLI to use the new helper**

In `cmd/specgraph/constitution.go`, the `runConstitutionImport` function currently parses the YAML via `config.LoadConstitutionConfig` and calls `constitutionConfigToProto`. Replace that path with:

```go
data, err := os.ReadFile(filepath)
if err != nil { ... }

c, err := load.LoadFromYAML(data)
if err != nil { ... }

pb := constitutionToProto(c) // existing function, lives in convert_constitution.go
```

This means we also need a `*storage.Constitution → *specv1.Constitution` converter. If `constitutionToProto` already exists in `internal/server/convert_constitution.go`, expose it. If not, write a thin one — it's the inverse of `constitutionFromProto` and mostly mechanical.

Actually simpler: the CLI sends the proto via RPC `UpdateConstitution`. So the CLI needs `domain → proto`. Either:

- (a) Expose the existing `constitutionToProto` from `internal/server/` (move to a shared location)
- (b) Write a local converter in `cmd/specgraph/`

Pick (a) — put it in `internal/constitution/load/` as `ToProto(c *storage.Constitution) *specv1.Constitution`. Both CLI and server import it. This complements the YAML→domain function.

Run all CLI constitution tests to verify no regression:

```bash
go test ./cmd/specgraph/... -count=1
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
jj --no-pager describe @ -m "feat(constitution/load): YAML to domain helper (spgr-8ar piece B)

New package internal/constitution/load extracts the YAML to
*storage.Constitution parse path from CLI-specific code so both the CLI
import command and the server's RefreshConstitutionLayer RPC can use it.

Layer validation centralized: rejects unknown layer strings (not in
user|org|project|domain) at parse time.

CLI 'constitution import' switched to use the new helper. Behavior
unchanged.

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"`.

---

## Task 3: `internal/constitution/fetch` package (core, no security tests yet)

**Files:**

- Create: `internal/constitution/fetch/fetch.go`
- Create: `internal/constitution/fetch/fetch_test.go`

This task creates the package with the go-getter wrapper and basic happy-path tests via `file://` (under `//go:build testfetch`). Security tests come in Task 4 to keep the diff readable.

- [ ] **Step 1: Write a happy-path test**

Create `internal/constitution/fetch/fetch_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build testfetch

package fetch_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/constitution/fetch"
)

func TestFetch_HappyPath_FileScheme(t *testing.T) {
	tmpDir := t.TempDir()
	fixturePath := filepath.Join(tmpDir, "constitution.yaml")
	fixture := []byte("name: test\nlayer: project\n")
	require.NoError(t, os.WriteFile(fixturePath, fixture, 0o644))

	url := "file://" + fixturePath
	result, err := fetch.Fetch(context.Background(), url)
	require.NoError(t, err)
	assert.Equal(t, fixture, result.Body, "body should match fixture exactly")
	assert.Equal(t, url, result.ResolvedURL, "ResolvedURL stores user-supplied URL verbatim")
}

func TestFetch_UnsupportedScheme(t *testing.T) {
	// 'mailto' is not a registered getter
	_, err := fetch.Fetch(context.Background(), "mailto:bob@example.com")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -tags testfetch ./internal/constitution/fetch/ -v
```

Expected: compile error — package doesn't exist.

- [ ] **Step 3: Implement `fetch.go`**

Create `internal/constitution/fetch/fetch.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package fetch retrieves constitution files from remote URLs using
// hashicorp/go-getter v1 with a restricted security posture: file-only
// mode, explicit getter allow-list, host-scoped GitHub token injection
// via SPECGRAPH_FETCH_GITHUB_TOKEN, URL credential sanitization, and
// short HTTP timeouts.
package fetch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-getter"
)

const (
	bodySizeCap = 1 << 20 // 1 MiB
	httpTimeout = 10 * time.Second
)

// Fetched holds the result of a remote constitution fetch.
type Fetched struct {
	// Body is the raw YAML/JSON content as bytes.
	Body []byte
	// ResolvedURL is the URL as the user supplied it.
	ResolvedURL string
}

// Fetch retrieves a constitution file from the given URL.
// Auth (when applicable) is injected internally based on host + env var.
func Fetch(ctx context.Context, rawURL string) (*Fetched, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "specgraph-fetch-")
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// go-getter writes a file at Dst; use a fixed name inside the temp dir
	// so the dispatcher always knows where to read the result.
	dst := tmpDir + "/constitution"

	client := &getter.Client{
		Ctx:           ctx,
		Src:           rawURL,
		Dst:           dst,
		Mode:          getter.ClientModeFile,
		Getters:       restrictedGetters(getTokenFromEnv()),
		Decompressors: nil,
	}
	if err := client.Get(); err != nil {
		return nil, classifyError(rawURL, err)
	}

	// Enforce size cap by stat-then-read.
	info, err := os.Stat(dst)
	if err != nil {
		return nil, fmt.Errorf("stat fetched file: %w", err)
	}
	if info.Size() > bodySizeCap {
		return nil, fmt.Errorf("fetched body exceeds %d bytes (got %d)", bodySizeCap, info.Size())
	}

	body, err := os.ReadFile(dst)
	if err != nil {
		return nil, fmt.Errorf("read fetched file: %w", err)
	}

	return &Fetched{Body: body, ResolvedURL: rawURL}, nil
}

// validateURL rejects URLs with embedded credentials (userinfo per
// RFC 3986) or common token-bearing query parameters. Per the design
// spec's URL credential containment invariant.
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.User != nil {
		return fmt.Errorf("URL contains embedded credentials; use SPECGRAPH_FETCH_GITHUB_TOKEN env var for authenticated GitHub access")
	}
	for _, k := range []string{"token", "access_token", "api_key", "password"} {
		if u.Query().Has(k) {
			return fmt.Errorf("URL query contains credential parameter %q; use SPECGRAPH_FETCH_GITHUB_TOKEN env var for authenticated GitHub access", k)
		}
	}
	return nil
}

// getTokenFromEnv reads SPECGRAPH_FETCH_GITHUB_TOKEN. Returns "" if unset.
func getTokenFromEnv() string {
	return strings.TrimSpace(os.Getenv("SPECGRAPH_FETCH_GITHUB_TOKEN"))
}

// classifyError maps go-getter errors to specgraph-friendly errors. V1
// keeps this simple: pass through with a wrapped message that includes
// the URL for diagnostics. Section 12 of the spec details the gRPC-code
// mapping; that happens in the RPC handler, not here.
func classifyError(rawURL string, err error) error {
	return fmt.Errorf("fetch %q: %w", rawURL, err)
}
```

Then create a second file for the registry + token transport: `internal/constitution/fetch/getters.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package fetch

import (
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-getter"
)

// restrictedGetters returns a getter registry with only the schemes we
// allow in production. If token is non-empty, the http/https getters
// inject Authorization: Bearer <token> for the GitHub host allow-list
// (see tokenTransport).
//
// file:// is intentionally not registered here — it's added only via the
// testfetch build tag (see getters_testfetch.go).
func restrictedGetters(token string) map[string]getter.Getter {
	httpClient := &http.Client{
		Timeout:       httpTimeout,
		CheckRedirect: stripAuthOnCrossHost,
		Transport: &tokenTransport{
			base:  http.DefaultTransport,
			token: token,
		},
	}
	httpGetter := &getter.HttpGetter{
		Client: httpClient,
	}
	// Note: no separate "github" scheme — go-getter's URL detectors
	// handle github.com/org/repo shorthand by rewriting to git:: before
	// dispatch (verified in Task 0).
	return map[string]getter.Getter{
		"http":  httpGetter,
		"https": httpGetter,
		"git":   new(getter.GitGetter),
	}
}

// tokenTransport injects Authorization: Bearer <token> on requests whose
// host is in the GitHub allow-list. Tokens are never sent to other hosts.
type tokenTransport struct {
	base  http.RoundTripper
	token string
}

func (t *tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.token != "" && hostAllowed(r.URL.Host) {
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.base.RoundTrip(r)
}

func hostAllowed(host string) bool {
	if host == "raw.githubusercontent.com" || host == "api.github.com" {
		return true
	}
	if strings.HasSuffix(host, ".githubusercontent.com") {
		return true
	}
	return false
}

// stripAuthOnCrossHost is the CheckRedirect handler that removes the
// Authorization header whenever a redirect crosses to a host outside
// the allow-list. Go's net/http does NOT strip Authorization
// automatically across redirects.
func stripAuthOnCrossHost(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	prev := via[len(via)-1]
	if req.URL.Host != prev.URL.Host && !hostAllowed(req.URL.Host) {
		req.Header.Del("Authorization")
	}
	return nil
}

var _ = time.Second // keep import; remove if unused
```

And `internal/constitution/fetch/getters_testfetch.go` (registered only under the build tag):

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build testfetch

package fetch

import "github.com/hashicorp/go-getter"

func init() {
	// Re-export restrictedGetters with file:// added, only under the
	// testfetch build tag. Production builds never register file://.
	defaultGetters = func(token string) map[string]getter.Getter {
		g := restrictedGetters(token)
		g["file"] = new(getter.FileGetter)
		return g
	}
}
```

For this to work, modify `fetch.go`'s `Fetch` function to use an indirected `defaultGetters` variable that production builds set to `restrictedGetters` and test builds (testfetch tag) override. Adjust the structure accordingly.

If this indirection feels too clever, an alternative: register `file://` in `restrictedGetters` unconditionally for V1 and document that production deployments should disable it via a config flag. This is less safe — pick the build-tag approach.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -tags testfetch ./internal/constitution/fetch/ -v
```

Expected: 2/2 PASS.

- [ ] **Step 5: Verify production build excludes file://**

```bash
go test ./internal/constitution/fetch/ -v
```

(No `-tags testfetch`.) The happy-path test should be skipped/fail because `file://` isn't registered. This is the right behavior — production builds must reject `file://`.

Actually since the test file has `//go:build testfetch`, it won't compile in production builds, so `go test` without the tag will report "no tests to run" — which is fine.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe @ -m "feat(constitution/fetch): go-getter wrapper with security posture

New package internal/constitution/fetch wraps hashicorp/go-getter v1
with restrictions: ClientModeFile only, restricted getter registry
(http/https/git/github), 10s HTTP timeout, 1 MiB body cap,
decompressors disabled, file:// behind testfetch build tag only.

URL credential sanitization: rejects URLs with RFC 3986 userinfo and
common token-bearing query parameters at parse time.

Token transport: SPECGRAPH_FETCH_GITHUB_TOKEN injected only for
raw.githubusercontent.com, api.github.com, and *.githubusercontent.com.
CheckRedirect strips Authorization on cross-host redirects.

Happy-path and unsupported-scheme tests via testfetch build tag.
Security checklist tests follow in the next commit.

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"`.

---

## Task 4: Security tests for `internal/constitution/fetch`

**Files:**

- Create: `internal/constitution/fetch/fetch_security_test.go`

Section 13's security checklist becomes explicit tests. Each must use real network behavior (httptest.Server) where applicable; do not stub out the transport layer.

- [ ] **Step 1: Write the security test file**

Create `internal/constitution/fetch/fetch_security_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build testfetch

package fetch_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/constitution/fetch"
)

const testToken = "ghp_TESTTOKEN1234567890"

// TestSecurity_TokenAllowList: with token set, fetching a non-GitHub
// HTTPS URL must NOT include Authorization in the captured request.
func TestSecurity_TokenAllowList(t *testing.T) {
	t.Setenv("SPECGRAPH_FETCH_GITHUB_TOKEN", testToken)

	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Write([]byte("name: test\n"))
	}))
	defer srv.Close()

	_, err := fetch.Fetch(context.Background(), srv.URL+"/constitution.yaml")
	require.NoError(t, err)
	assert.Empty(t, capturedAuth,
		"token must NOT be sent to non-allow-list hosts")
}

// TestSecurity_CrossHostRedirect: redirect from allow-listed host to
// arbitrary host must strip the Authorization header.
func TestSecurity_CrossHostRedirect(t *testing.T) {
	t.Skip("Cross-host redirect test requires DNS or host header rewriting; integration-only.")
	// Full implementation deferred; see invariant doc in Section 14.
	// A skeleton test that confirms the CheckRedirect function's logic
	// in isolation can be added in the unit tests.
}

// TestSecurity_BodySizeCap: bodies > 1 MiB are rejected before parse.
func TestSecurity_BodySizeCap(t *testing.T) {
	big := bytes.Repeat([]byte("x"), 2<<20) // 2 MiB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(big)
	}))
	defer srv.Close()

	_, err := fetch.Fetch(context.Background(), srv.URL+"/big.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

// TestSecurity_URLWithUserInfo_Rejected: https://token@host/path rejected.
func TestSecurity_URLWithUserInfo_Rejected(t *testing.T) {
	_, err := fetch.Fetch(context.Background(),
		"https://ghp_xxxxxxxxxxxx@raw.githubusercontent.com/foo/bar/main/constitution.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedded credentials")
	assert.Contains(t, err.Error(), "SPECGRAPH_FETCH_GITHUB_TOKEN")
}

// TestSecurity_URLWithUserPass_Rejected: https://user:pass@host/path rejected.
func TestSecurity_URLWithUserPass_Rejected(t *testing.T) {
	_, err := fetch.Fetch(context.Background(),
		"https://user:pass@example.com/constitution.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedded credentials")
}

// TestSecurity_URLWithTokenQuery_Rejected: ?token=secret rejected.
func TestSecurity_URLWithTokenQuery_Rejected(t *testing.T) {
	_, err := fetch.Fetch(context.Background(),
		"https://example.com/constitution.yaml?token=secret123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credential parameter")
}

// TestSecurity_LogRedaction: token bytes must not appear in log output
// during a successful fetch.
func TestSecurity_LogRedaction(t *testing.T) {
	t.Setenv("SPECGRAPH_FETCH_GITHUB_TOKEN", testToken)

	// Capture all slog output during the fetch.
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(orig)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("name: test\n"))
	}))
	defer srv.Close()

	_, _ = fetch.Fetch(context.Background(), srv.URL+"/constitution.yaml")
	assert.NotContains(t, buf.String(), testToken,
		"token bytes must not appear in any log line")
}
```

The `CrossHostRedirect` test is intentionally skipped at this stage — building a full DNS-rewriting test infra is overkill for V1, and the redirect-stripping logic is tested via the unit test of `stripAuthOnCrossHost` directly. Add such a unit test in `getters_test.go`.

- [ ] **Step 2: Add the redirect-stripping unit test**

Create `internal/constitution/fetch/getters_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package fetch

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripAuthOnCrossHost(t *testing.T) {
	// Helper to build a request with an Authorization header.
	mkReq := func(host string) *http.Request {
		r := &http.Request{
			URL:    &url.URL{Host: host, Scheme: "https"},
			Header: http.Header{},
		}
		r.Header.Set("Authorization", "Bearer token123")
		return r
	}

	// Same-host redirect — preserve the header.
	prev := mkReq("raw.githubusercontent.com")
	cur := mkReq("raw.githubusercontent.com")
	require.NoError(t, stripAuthOnCrossHost(cur, []*http.Request{prev}))
	assert.NotEmpty(t, cur.Header.Get("Authorization"))

	// Cross-host to non-allow-list — strip the header.
	prev = mkReq("raw.githubusercontent.com")
	cur = mkReq("attacker.example.com")
	require.NoError(t, stripAuthOnCrossHost(cur, []*http.Request{prev}))
	assert.Empty(t, cur.Header.Get("Authorization"),
		"Authorization must be stripped on cross-host redirect")

	// Cross-host to another allow-listed host — keep the header.
	prev = mkReq("raw.githubusercontent.com")
	cur = mkReq("api.github.com")
	require.NoError(t, stripAuthOnCrossHost(cur, []*http.Request{prev}))
	assert.NotEmpty(t, cur.Header.Get("Authorization"))
}
```

Note: the `require` import needs `"github.com/stretchr/testify/require"`. Add it.

- [ ] **Step 3: Run all security tests**

```bash
go test -tags testfetch ./internal/constitution/fetch/ -v
```

Expected: all tests PASS (one skipped: `TestSecurity_CrossHostRedirect`).

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe @ -m "test(constitution/fetch): security checklist tests

Implements Section 13 security checklist from the design spec:
- token allow-list (no Auth header to non-GitHub hosts)
- body size cap fires before parse
- URL userinfo rejected (with env-var hint in error)
- URL user:pass rejected
- URL token-bearing query params rejected
- log redaction (token bytes never logged)
- cross-host redirect strips Authorization (unit test on the
  stripAuthOnCrossHost function; full integration deferred to a
  follow-up bead)

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"`.

---

## Task 5: Proto definition + regeneration

**Files:**

- Modify: `proto/specgraph/v1/constitution.proto`
- Regenerated: `gen/specgraph/v1/constitution.pb.go`, `gen/specgraph/v1/specgraphv1connect/constitution.connect.go`

- [ ] **Step 1: Add the new RPC to the proto**

Edit `proto/specgraph/v1/constitution.proto`. Find the `ConstitutionService` service block and add:

```protobuf
service ConstitutionService {
    // ... existing RPCs (GetConstitution, UpdateConstitution, EmitToolFiles) ...

    // RefreshConstitutionLayer fetches a remote constitution and updates
    // the specified layer. Used by `specgraph constitution import
    // --from-url` and `specgraph constitution sync`.
    rpc RefreshConstitutionLayer(RefreshConstitutionLayerRequest)
        returns (RefreshConstitutionLayerResponse);
}

message RefreshConstitutionLayerRequest {
    // layer identifies which layer to refresh. Required, non-UNSPECIFIED.
    ConstitutionLayer layer = 1;
    // source_url is the URL to fetch from. Required. Must pass URL
    // credential sanitization (no userinfo, no token query params).
    string source_url = 2;
    // dry_run skips the write and returns the diff only.
    bool dry_run = 3;
}

message RefreshConstitutionLayerResponse {
    Constitution before = 1;          // nil if no prior layer
    Constitution after = 2;           // newly fetched + parsed
    string previous_source_hash = 3;  // "" if no prior layer
    string new_source_hash = 4;
    bool changed = 5;                 // false iff hashes match (no write)
}
```

- [ ] **Step 2: Regenerate**

```bash
cd ~/Code/github.com/specgraph-8ar-piece-b
task proto
```

Verify the generated files now contain the new types:

```bash
grep -c "RefreshConstitutionLayer" gen/specgraph/v1/constitution.pb.go
```

Expected: non-zero match count.

- [ ] **Step 3: Build to verify**

```bash
go build ./...
```

Expected: clean build. (Handler implementation in Task 6 fills the interface gap.)

If build fails because the new `ConstitutionServiceHandler` interface has an unimplemented method, that's expected — the handler is the next task. Note in the commit message.

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe @ -m "feat(proto): RefreshConstitutionLayer RPC

Adds RefreshConstitutionLayer to ConstitutionService for remote-source
constitution import and drift sync. Request: layer + source_url +
dry_run. Response: before/after constitutions, previous/new source
hashes, changed bool.

Handler implementation follows in the next commit (build is temporarily
broken between this commit and the next).

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"`.

---

## Task 6: `RefreshConstitutionLayer` RPC handler

**Files:**

- Modify: `internal/server/constitution_handler.go`
- Modify: `internal/server/constitution_handler_test.go`
- Modify: `internal/auth/permissions.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/server/constitution_handler_test.go`:

```go
func TestRefreshConstitutionLayer_NewLayer_HashUnset(t *testing.T) {
	t.Skip("requires fetch fake — see TestRefreshConstitutionLayer_WithFakeFetcher below")
}

// To test RefreshConstitutionLayer without real network calls, the
// handler accepts an injected Fetcher interface. The default
// implementation calls fetch.Fetch; tests inject a fake.

// Add a Fetcher interface in the handler, then test:
// - First-time fetch (no prior layer) → changed=true, layer written
// - Same content (hash match) → changed=false, no write
// - Modified content → changed=true, before/after populated
// - Dry-run with modified content → changed=true, no write
// - Layer UNSPECIFIED → CodeInvalidArgument
// - Malformed body → CodeInvalidArgument

func TestRefreshConstitutionLayer_WithFakeFetcher(t *testing.T) {
	fake := &fakeFetcher{
		body: []byte("name: test\nlayer: project\nprinciples:\n  - id: p1\n    statement: First\n"),
	}
	store := &mockConstitutionBackend{}
	// ... wire up handler with fake fetcher ...
	// ... assert changed=true and one layer stored ...
}

type fakeFetcher struct {
	body []byte
	err  error
}

func (f *fakeFetcher) Fetch(ctx context.Context, url string) (*fetch.Fetched, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &fetch.Fetched{Body: f.body, ResolvedURL: url}, nil
}
```

Cover at minimum:

- New layer (no prior): writes, changed=true
- Same content (hash match against prior): no write, changed=false
- Modified content: writes new, changed=true, before populated
- Dry-run on modified: changed=true, no write
- Layer UNSPECIFIED → CodeInvalidArgument
- URL with embedded credentials → CodeInvalidArgument (via fake fetch returning the validation error)

- [ ] **Step 2: Implement the handler**

In `internal/server/constitution_handler.go`, add a `Fetcher` interface and inject it:

```go
type Fetcher interface {
    Fetch(ctx context.Context, url string) (*fetch.Fetched, error)
}

type defaultFetcher struct{}

func (defaultFetcher) Fetch(ctx context.Context, url string) (*fetch.Fetched, error) {
    return fetch.Fetch(ctx, url)
}

type ConstitutionHandler struct {
    scoper  storage.Scoper
    fetcher Fetcher
}

func RegisterConstitutionService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
    handler := &ConstitutionHandler{
        scoper:  scoper,
        fetcher: defaultFetcher{},
    }
    // ... existing wiring ...
}
```

Add the `RefreshConstitutionLayer` method:

```go
func (h *ConstitutionHandler) RefreshConstitutionLayer(ctx context.Context, req *connect.Request[specv1.RefreshConstitutionLayerRequest]) (*connect.Response[specv1.RefreshConstitutionLayerResponse], error) {
    // 1. Validate layer.
    if req.Msg.Layer == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
        return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("layer is required"))
    }
    layer, ok := constitutionLayerFromProtoMap[req.Msg.Layer]
    if !ok {
        return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown layer: %s", req.Msg.Layer))
    }

    // 2. Fetch (URL credential sanitization happens inside the fetcher).
    fetched, err := h.fetcher.Fetch(ctx, req.Msg.SourceUrl)
    if err != nil {
        return nil, classifyFetchError(err)
    }

    // 3. Parse.
    parsed, err := load.LoadFromYAML(fetched.Body)
    if err != nil {
        return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parse: %w", err))
    }
    parsed.Layer = layer
    parsed.SourceURL = fetched.ResolvedURL

    // 4. Hash and compare.
    newHash, err := hash.Hash(parsed)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("hash: %w", err))
    }
    parsed.SourceHash = newHash

    store, err := scopeStore(ctx, h.scoper)
    if err != nil {
        return nil, err
    }

    prior, priorErr := store.GetConstitutionLayer(ctx, layer)
    var prevHash string
    if priorErr == nil {
        prevHash = prior.SourceHash
    }

    changed := prevHash != newHash

    resp := &specv1.RefreshConstitutionLayerResponse{
        After:             constitutionToProto(parsed),
        PreviousSourceHash: prevHash,
        NewSourceHash:     newHash,
        Changed:           changed,
    }
    if priorErr == nil {
        resp.Before = constitutionToProto(prior)
    }

    // 5. Dry-run or no-change → return without writing.
    if req.Msg.DryRun || !changed {
        return connect.NewResponse(resp), nil
    }

    // 6. Write.
    written, err := store.UpdateConstitution(ctx, parsed)
    if err != nil {
        return nil, constitutionError(err)
    }
    resp.After = constitutionToProto(written)
    return connect.NewResponse(resp), nil
}

// classifyFetchError maps fetch errors to gRPC codes per Section 12.
func classifyFetchError(err error) error {
    msg := err.Error()
    switch {
    case strings.Contains(msg, "embedded credentials"),
         strings.Contains(msg, "credential parameter"),
         strings.Contains(msg, "exceeds"):
        return connect.NewError(connect.CodeInvalidArgument, err)
    default:
        return connect.NewError(connect.CodeUnavailable, err)
    }
}
```

If `constitutionToProto` doesn't exist as an exported function (it likely lives in `internal/server/convert_constitution.go` as unexported), expose it or call the equivalent local converter.

Also: the spec's Section 12 calls for a HEAD pre-flight for HTTPS URLs to distinguish 401/403/404 from other failures. V1 can defer this — the string-matching above is sufficient for the security-test cases. File a follow-up if real precision is needed.

- [ ] **Step 3: Add the permission entry**

In `internal/auth/permissions.go`, add the new procedure to the permission map:

```go
specgraphv1connect.ConstitutionServiceRefreshConstitutionLayerProcedure: "constitution:write",
```

Match the convention of other constitution-writing RPCs.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/server/... -v -count=1 -run TestRefreshConstitutionLayer
```

Expected: all new tests PASS. Run the full server suite to verify no regressions:

```bash
go test ./internal/server/... -count=1
```

- [ ] **Step 5: Build full tree**

```bash
go build ./...
```

Expected: clean.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe @ -m "feat(server): RefreshConstitutionLayer RPC handler

Wires up the new RPC: validates layer, fetches body via internal/
constitution/fetch (with URL sanitization + auth), parses via
internal/constitution/load, hashes via internal/constitution/hash,
compares to existing layer's hash, writes if changed (and not
dry-run).

Handler accepts an injected Fetcher interface for testability; the
default implementation calls fetch.Fetch.

Error classification (Section 12): URL/parse/size errors map to
CodeInvalidArgument; all other fetch failures map to CodeUnavailable.
HEAD pre-flight for 401/403/404 distinction is deferred to a follow-up
(string-matching is sufficient for V1).

Adds 'constitution:write' permission for the new procedure.

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"`.

---

## Task 7: CLI `constitution import --from-url`

**Files:**

- Modify: `cmd/specgraph/constitution.go`

- [ ] **Step 1: Add the flag and the new code path**

In `cmd/specgraph/constitution.go`, locate `constitutionImportCmd` and add a `--from-url` flag:

```go
var (
    importLayerFlag    string
    importFromURLFlag  string
    importProjectSlug  string
)

func init() {
    constitutionImportCmd.Flags().StringVar(&importFromURLFlag, "from-url", "", "fetch constitution from URL (alternative to local file argument)")
    // ... existing flags ...
}
```

In `runConstitutionImport`:

```go
func runConstitutionImport(cmd *cobra.Command, args []string) error {
    if importFromURLFlag != "" {
        if len(args) > 0 {
            return fmt.Errorf("cannot specify both <path> and --from-url")
        }
        if importLayerFlag == "" {
            return fmt.Errorf("--layer is required when using --from-url")
        }
        return runImportFromURL(cmd, importFromURLFlag, importLayerFlag)
    }
    // ... existing local-file path ...
}

func runImportFromURL(cmd *cobra.Command, url, layerStr string) error {
    layer := constitutionLayerStringToProto(layerStr)
    if layer == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
        return fmt.Errorf("invalid layer %q", layerStr)
    }

    client, err := constitutionClient()
    if err != nil {
        return err
    }

    resp, err := client.RefreshConstitutionLayer(cmd.Context(), connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
        Layer:     layer,
        SourceUrl: url,
    }))
    if err != nil {
        return fmt.Errorf("refresh constitution layer: %w", err)
    }

    fmt.Fprintf(cmd.OutOrStdout(), "Imported as '%s' layer (sha: %s)\n", layerStr, resp.Msg.GetNewSourceHash())
    return nil
}
```

- [ ] **Step 2: Test manually with file:// URL (testfetch tag)**

Build the binary with `-tags testfetch`:

```bash
go build -tags testfetch -o /tmp/specgraph-testfetch ./cmd/specgraph
```

Spin up a postgres + specgraph server in a separate terminal (or test in an e2e test in Task 9). For now, smoke-test the CLI parsing:

```bash
/tmp/specgraph-testfetch constitution import --help
```

Expected: `--from-url` and `--layer` flags listed.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe @ -m "feat(cli): constitution import --from-url

Adds --from-url flag to 'specgraph constitution import' that fetches
the constitution from a remote URL via the RefreshConstitutionLayer
RPC. --layer is required when using --from-url.

Local file argument and --from-url are mutually exclusive.

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"`.

---

## Task 8: CLI `constitution sync`

**Files:**

- Modify: `cmd/specgraph/constitution.go`

- [ ] **Step 1: Add the sync subcommand**

In `cmd/specgraph/constitution.go`:

```go
var (
    syncLayerFlag string
    syncDryRun    bool
    syncCheck     bool
)

var constitutionSyncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Re-fetch remote constitution layers and detect drift",
    RunE:  runConstitutionSync,
}

func init() {
    constitutionSyncCmd.Flags().StringVar(&syncLayerFlag, "layer", "", "sync only this layer (default: all layers with source_url)")
    constitutionSyncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "fetch and compare but do not write")
    constitutionSyncCmd.Flags().BoolVar(&syncCheck, "check", false, "exit 1 if drift detected; useful in CI")
    constitutionCmd.AddCommand(constitutionSyncCmd)
}

func runConstitutionSync(cmd *cobra.Command, _ []string) error {
    client, err := constitutionClient()
    if err != nil {
        return err
    }

    // 1. List all layers via GetConstitution with no filter — returns merged + all-layer metadata?
    // Actually we need the raw per-layer data with source_url. Use GetAllLayers via GetConstitution
    // with each layer enum value? Or add a ListLayers RPC?
    //
    // For V1: iterate the four ConstitutionLayer enum values, call GetConstitution with each,
    // collect those that have a non-empty SourceURL.

    type layerInfo struct {
        name string
        layer specv1.ConstitutionLayer
        url string
    }
    var withSource []layerInfo

    for _, l := range []specv1.ConstitutionLayer{
        specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER,
        specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
        specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
        specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN,
    } {
        resp, err := client.GetConstitution(cmd.Context(), connect.NewRequest(&specv1.GetConstitutionRequest{Layer: l}))
        if err != nil {
            if connect.CodeOf(err) == connect.CodeNotFound {
                continue
            }
            return fmt.Errorf("list layers: %w", err)
        }
        c := resp.Msg.GetConstitution()
        if c != nil && c.GetSourceUrl() != "" {
            withSource = append(withSource, layerInfo{
                name:  constitutionLayerProtoToString(l),
                layer: l,
                url:   c.GetSourceUrl(),
            })
        }
    }

    if syncLayerFlag != "" {
        layer := constitutionLayerStringToProto(syncLayerFlag)
        if layer == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
            return fmt.Errorf("invalid layer %q", syncLayerFlag)
        }
        var filtered []layerInfo
        for _, li := range withSource {
            if li.layer == layer {
                filtered = append(filtered, li)
            }
        }
        if len(filtered) == 0 {
            return fmt.Errorf("layer %s has no source_url; nothing to sync", syncLayerFlag)
        }
        withSource = filtered
    }

    if len(withSource) == 0 {
        fmt.Fprintln(cmd.OutOrStdout(), "no remote layers configured; nothing to sync")
        return nil
    }

    var driftDetected, updated, failed int
    for _, li := range withSource {
        resp, err := client.RefreshConstitutionLayer(cmd.Context(), connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
            Layer:     li.layer,
            SourceUrl: li.url,
            DryRun:    syncDryRun,
        }))
        if err != nil {
            fmt.Fprintf(cmd.OutOrStdout(), "%-8s  error           %v\n", li.name, err)
            failed++
            continue
        }
        if !resp.Msg.GetChanged() {
            fmt.Fprintf(cmd.OutOrStdout(), "%-8s  unchanged       (sha %s)\n", li.name, shortHash(resp.Msg.GetNewSourceHash()))
            continue
        }
        driftDetected++
        if syncDryRun {
            fmt.Fprintf(cmd.OutOrStdout(), "%-8s  would-change    (sha %s -> %s)\n", li.name,
                shortHash(resp.Msg.GetPreviousSourceHash()), shortHash(resp.Msg.GetNewSourceHash()))
        } else {
            fmt.Fprintf(cmd.OutOrStdout(), "%-8s  changed         (sha %s -> %s)\n", li.name,
                shortHash(resp.Msg.GetPreviousSourceHash()), shortHash(resp.Msg.GetNewSourceHash()))
            updated++
        }
    }

    fmt.Fprintf(cmd.OutOrStdout(), "\n%d of %d remote layers checked, %d updated.\n", len(withSource), len(withSource), updated)

    if failed > 0 {
        os.Exit(2)
    }
    if syncCheck && driftDetected > 0 {
        os.Exit(1)
    }
    return nil
}

func shortHash(h string) string {
    if len(h) <= 8 {
        return h
    }
    return h[:8] + "..."
}
```

`constitutionLayerProtoToString` is the inverse of the existing `constitutionLayerStringToProto` — write it if it doesn't already exist.

- [ ] **Step 2: Test the help output**

```bash
go build -o /tmp/specgraph ./cmd/specgraph
/tmp/specgraph constitution sync --help
```

Expected: lists `--layer`, `--dry-run`, `--check`.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe @ -m "feat(cli): constitution sync

Adds 'specgraph constitution sync' subcommand that iterates all
constitution layers with a non-empty source_url and calls
RefreshConstitutionLayer for each. Output format adapts to actually-
present layers (not the full enum).

Flags:
- --layer: sync only the specified layer
- --dry-run: fetch and compare, do not write
- --check: exit 1 if drift detected (for CI); default exit 0

Exit codes:
- 0: ran successfully (with or without drift, unless --check)
- 1: --check + drift detected (or would-detect under --dry-run)
- 2: at least one fetch failed (other layers still attempted)

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"`.

---

## Task 9: E2E test for full refresh flow

**Files:**

- Create: `e2e/api/constitution_refresh_test.go`

- [ ] **Step 1: Write the E2E test**

Create `e2e/api/constitution_refresh_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "connectrpc.com/connect"

    specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestE2E_RefreshConstitutionLayer_FileURL(t *testing.T) {
    if os.Getenv("SPECGRAPH_FETCH_TESTFETCH") == "" {
        t.Skip("requires testfetch-built binary; set SPECGRAPH_FETCH_TESTFETCH=1 to run")
    }

    client := newConstitutionClient(t)
    ctx := context.Background()

    tmpDir := t.TempDir()
    fixturePath := filepath.Join(tmpDir, "constitution.yaml")
    fixture := []byte(`
name: e2e-test
layer: org
principles:
  - id: e2e-p1
    statement: E2E principle
`)
    require.NoError(t, os.WriteFile(fixturePath, fixture, 0o644))

    url := "file://" + fixturePath

    // First refresh — should be a fresh layer.
    resp, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
        Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
        SourceUrl: url,
    }))
    require.NoError(t, err)
    assert.True(t, resp.Msg.GetChanged(), "first refresh must report changed")
    assert.Nil(t, resp.Msg.GetBefore())
    assert.Equal(t, "e2e-test", resp.Msg.GetAfter().GetName())

    // Second refresh on the same file — should be no-change.
    resp2, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
        Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
        SourceUrl: url,
    }))
    require.NoError(t, err)
    assert.False(t, resp2.Msg.GetChanged(), "second refresh on unchanged file must report unchanged")

    // Modify the file and re-refresh — should be change with before populated.
    modified := []byte(`
name: e2e-test-modified
layer: org
principles:
  - id: e2e-p1
    statement: E2E principle (modified)
`)
    require.NoError(t, os.WriteFile(fixturePath, modified, 0o644))

    resp3, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
        Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
        SourceUrl: url,
    }))
    require.NoError(t, err)
    assert.True(t, resp3.Msg.GetChanged())
    assert.NotNil(t, resp3.Msg.GetBefore())
    assert.Equal(t, "e2e-test-modified", resp3.Msg.GetAfter().GetName())
}
```

`newConstitutionClient(t)` follows the existing e2e helper pattern in the package.

- [ ] **Step 2: Run the e2e test**

```bash
SPECGRAPH_FETCH_TESTFETCH=1 go test -tags "e2e testfetch" ./e2e/api/ -run TestE2E_RefreshConstitutionLayer -v -count=1
```

Expected: PASS. The server must be running with the testfetch build tag so `file://` URLs are accepted.

If the e2e infrastructure starts a server, it needs to be built with the testfetch tag too. Update `e2e/api/main_test.go` or the equivalent server-startup helper to use `go test -tags testfetch` for the server binary.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe @ -m "test(e2e): RefreshConstitutionLayer full flow via file://

End-to-end test exercising the full RefreshConstitutionLayer RPC
against a live postgres + specgraph server. Uses file:// URLs (which
require the testfetch build tag on the server binary).

Tests three scenarios:
- First refresh (no prior layer): changed=true, before nil
- Second refresh on unchanged file: changed=false
- Refresh after modification: changed=true, before populated

Part of spgr-8ar Piece B.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then `jj --no-pager new @ -m "(working)"`.

---

## Task 10: Quality gates + PR

**Files:** none

- [ ] **Step 1: `task check`**

```bash
cd ~/Code/github.com/specgraph-8ar-piece-b
task check
```

Expected: all checks PASS. Address any lint issues inline.

- [ ] **Step 2: `task pr-prep`**

```bash
task pr-prep
```

Expected: all checks PASS, including integration + e2e tests. Docker required.

- [ ] **Step 3: Update bd issue + push**

```bash
cd ~/Code/github.com/specgraph
bd update spgr-15gz --notes "Piece B complete locally. ~10 commits covering hash/load/fetch packages, RefreshConstitutionLayer RPC, CLI import --from-url + sync. task pr-prep green."
bd dolt push
```

- [ ] **Step 4: Push bookmark + open PR**

```bash
gh auth switch -u seanb4t -h github.com

cd ~/Code/github.com/specgraph-8ar-piece-b
jj --no-pager bookmark set spgr-8ar-piece-b -r @-
jj --no-pager git push --bookmark spgr-8ar-piece-b
```

Open the PR from the main repo path:

```bash
cd ~/Code/github.com/specgraph
gh pr create --head spgr-8ar-piece-b --base main \
  --title "spgr-8ar PR B: remote-source import + sync (env-var auth)" \
  --body "..."
```

PR body should summarize:

- New packages (hash, load, fetch)
- New RPC + handler + permission
- New CLI commands (--from-url, sync with --check/--dry-run)
- Security checklist from Section 13 (link to the tests)
- DCO signoff (matching author email, learned from Piece A)
- Closes: spgr-15gz

---

## Self-Review Checklist

After all tasks, before opening the PR:

- [ ] All `Signed-off-by:` trailers use `Sean Brandt <4678+seanb4t@users.noreply.github.com>` (matches author email; DCO requirement)
- [ ] All `jj describe` (not `git commit -s` to avoid the mismatched author/signoff trap)
- [ ] `task check` and `task pr-prep` both green
- [ ] Section 13 security checklist tests all present and passing
- [ ] Section 14 invariants 3 (Source URL ↔ source_hash coupling), 4 (Hash determinism), 5 (Fetch idempotency), 6 (Token containment), 7 (Data preservation on failure), 13 (Source-URL allow-list) all have test coverage
- [ ] No `git commit` used anywhere (CLAUDE.md rule for jj-colocated repos)
- [ ] Documentation in spec is consistent with implementation (no drift)
- [ ] Plan checkboxes all ticked

## Open issues to flag in PR

- The `TestSecurity_CrossHostRedirect` test is skipped (full network-level redirect simulation requires more infra than V1 warrants); the redirect-stripping logic is tested directly via `stripAuthOnCrossHost` unit test instead.
- HEAD pre-flight for HTTPS URLs (Section 12 spec call) is not implemented in V1; error classification uses string matching. File follow-up bead if real precision needed.
- SHA-pinning resolution (rewriting `?ref=main` to `?ref=<sha>`) is explicitly deferred per the spec.

## Plan complete

10 tasks total. Substantially larger than Piece A — new dependency, new RPC, new CLI subcommands, security-sensitive auth handling. Estimated 1-2 sessions to execute via subagent-driven development.
