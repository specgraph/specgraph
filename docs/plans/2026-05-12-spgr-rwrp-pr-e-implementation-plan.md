# spgr-rwrp PR E — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Claude Code plugin shim through `internal/config/managedfiles/`'s embed-and-write framework: 6 new manifest entries, 3 migrated entries, 3 new `JSONKeyMerge` modes, a `WholeFile+CommentNone` path for JSON files, and three PR D review fold-ins.

**Architecture:** Mirror PRs C and D. Canonicals live under `internal/config/managedfiles/embedded/claude/`; `plugin/specgraph/` becomes reverse-symlinks. JSON files use no in-file sentinel (hash-only state classification via a unified priors registry). `.claude/settings.json` uses a new declarative `JSONKeys` shape with `KeyManagedValue`, `KeyManagedPresence`, and `KeyManagedArrayUnion` modes. All three existing `JSONKeyMerge` entries (`.mcp.json`, `.cursor/mcp.json`, `opencode.json`) migrate onto `JSONKeys` for framework consistency.

**Tech Stack:** Go 1.24, `github.com/evanphx/json-patch/v5` (already vendored), `embed.FS`, Taskfile.dev (`task check` / `task pr-prep`). Tests use stdlib `testing`. E2E uses Ginkgo/Gomega under `//go:build e2e`.

**Spec:** [`2026-05-12-spgr-rwrp-pr-e-claude-plugin-design.md`](2026-05-12-spgr-rwrp-pr-e-claude-plugin-design.md)

**Bead:** spgr-kir0

---

## File Structure

### New files

- `internal/config/managedfiles/jsonkeys.go` — `JSONKeyMode`, `JSONManagedKey`, JSON Pointer helpers (`jsonPointerSet`, `jsonPointerGet`).
- `internal/config/managedfiles/jsonkeys_test.go` — JSON Pointer round-trip tests.
- `internal/config/managedfiles/priors.go` — `priorsRegistry` type, `priorsFor` accessor, package-init registration.
- `internal/config/managedfiles/priors_test.go` — registry lookup tests.
- `internal/config/managedfiles/embedded/claude/.claude-plugin/plugin.json` — canonical plugin manifest with inlined hooks.
- `internal/config/managedfiles/embedded/claude/.claude-plugin/marketplace.json` — canonical single-plugin marketplace.
- `internal/config/managedfiles/embedded/claude/hooks/specgraph-session-start.sh` — moved + renamed from `plugin/specgraph/hooks/session-start.sh`.
- `internal/config/managedfiles/embedded/claude/hooks/specgraph-post-stage.sh` — moved + renamed.
- `internal/config/managedfiles/embedded/claude/routing-guide.md` — moved from `plugin/specgraph/`.
- `e2e/api/claude_plugin_test.go` — E2E test of the full shim install.

### Modified files

- `internal/config/managedfiles/types.go` — add `JSONKeys []JSONManagedKey` field.
- `internal/config/managedfiles/manifest.go` — migrate 3 entries; add 6 new entries; remove `unionPluginArray`'s path-keyed special case from the closure list.
- `internal/config/managedfiles/manifest_test.go` — flip negative anchor → positive shape test.
- `internal/config/managedfiles/jsonkeymerge.go` — replace `Build`-path canonical with `JSONKeys`-driven canonical; remove `unionPluginArray` hook.
- `internal/config/managedfiles/wholefile.go` — add `CommentNone` path; extract `keepEditsBodyForFrontmatter` / `keepEditsBodyPlain` helpers.
- `internal/config/managedfiles/vestigial_cursor_rules.go` — merge `vestigialCursorRulePriorHash` into `priorsRegistry`.
- `internal/config/managedfiles/supersedes.go` — export drift-detail consts.

### Deleted files

- `plugin/specgraph/hooks/hooks.json` — hooks declaration inlined into `plugin.json`.
- `plugin/specgraph/hooks/session-start.sh`, `…/post-stage.sh` — relocated.
- `plugin/specgraph/routing-guide.md`, `plugin/specgraph/.claude-plugin/plugin.json` — relocated.

### Reverse-symlinks created (under `plugin/specgraph/`)

- `.claude-plugin → ../../internal/config/managedfiles/embedded/claude/.claude-plugin`
- `hooks → ../../internal/config/managedfiles/embedded/claude/hooks`
- `routing-guide.md → ../../internal/config/managedfiles/embedded/claude/routing-guide.md`

---

## Task 1: Define `JSONKeyMode`, `JSONManagedKey`, and the `JSONKeys` field

**Files:**

- Create: `internal/config/managedfiles/jsonkeys.go`
- Modify: `internal/config/managedfiles/types.go`
- Test: `internal/config/managedfiles/jsonkeys_test.go`

- [ ] **Step 1: Write the failing test for JSON Pointer round-trip**

Create `internal/config/managedfiles/jsonkeys_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSONPointerSet_RoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		pointer string
		value   any
		want    string
	}{
		{
			name:    "top-level key",
			base:    `{}`,
			pointer: "/foo",
			value:   "bar",
			want:    `{"foo":"bar"}`,
		},
		{
			name:    "nested key creates intermediate object",
			base:    `{}`,
			pointer: "/a/b/c",
			value:   42.0,
			want:    `{"a":{"b":{"c":42}}}`,
		},
		{
			name:    "key with @ does not need escaping",
			base:    `{}`,
			pointer: "/enabledPlugins/specgraph@specgraph-local",
			value:   true,
			want:    `{"enabledPlugins":{"specgraph@specgraph-local":true}}`,
		},
		{
			name:    "RFC 6901 escape: ~1 means /",
			base:    `{}`,
			pointer: "/path~1with~1slash",
			value:   "v",
			want:    `{"path/with/slash":"v"}`,
		},
		{
			name:    "RFC 6901 escape: ~0 means ~",
			base:    `{}`,
			pointer: "/tilde~0key",
			value:   "v",
			want:    `{"tilde~key":"v"}`,
		},
		{
			name:    "overwrite existing value",
			base:    `{"foo":"old"}`,
			pointer: "/foo",
			value:   "new",
			want:    `{"foo":"new"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal([]byte(tc.base), &doc); err != nil {
				t.Fatalf("unmarshal base: %v", err)
			}
			if err := jsonPointerSet(doc, tc.pointer, tc.value); err != nil {
				t.Fatalf("jsonPointerSet: %v", err)
			}
			got, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var gotAny, wantAny any
			_ = json.Unmarshal(got, &gotAny)
			_ = json.Unmarshal([]byte(tc.want), &wantAny)
			if !reflect.DeepEqual(gotAny, wantAny) {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestJSONPointerGet(t *testing.T) {
	doc := map[string]any{
		"enabledPlugins": map[string]any{
			"specgraph@specgraph-local": true,
		},
	}
	v, ok := jsonPointerGet(doc, "/enabledPlugins/specgraph@specgraph-local")
	if !ok {
		t.Fatal("expected key present, got missing")
	}
	if v != true {
		t.Errorf("got %v, want true", v)
	}
	if _, ok := jsonPointerGet(doc, "/nonexistent"); ok {
		t.Error("expected missing key to report not-present")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/managedfiles/ -run TestJSONPointer -v`
Expected: FAIL — `jsonPointerSet` and `jsonPointerGet` undefined.

- [ ] **Step 3: Implement `jsonkeys.go`**

Create `internal/config/managedfiles/jsonkeys.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"fmt"
	"strings"
)

// JSONKeyMode controls how a JSONKeyMerge strategy treats a managed key.
type JSONKeyMode int

const (
	// KeyManagedValue overwrites the key on every init. For object-valued
	// keys, JSON Merge Patch (RFC 7396) recursively merges; for scalar and
	// array values, the canonical value replaces whatever is on disk.
	KeyManagedValue JSONKeyMode = iota

	// KeyManagedPresence ensures the key exists. On first init the value
	// is written; on subsequent inits an existing value is preserved.
	// Useful for keys whose presence we want to guarantee but whose value
	// belongs to the user (e.g. enabledPlugins entries toggled via
	// /plugin disable).
	KeyManagedPresence

	// KeyManagedArrayUnion treats the key as an array. Canonical elements
	// are unioned with existing elements (set-union by reflect.DeepEqual;
	// duplicates collapse). Formalizes the unionPluginArray hook in
	// jsonkeymerge.go.
	KeyManagedArrayUnion
)

// JSONManagedKey is one managed key inside a JSONKeyMerge file.
type JSONManagedKey struct {
	// Path is a JSON Pointer (RFC 6901) addressing the key. Use
	// slash-separated segments; ~ and / inside a key are escaped as
	// ~0 and ~1 respectively.
	Path string
	// Mode is one of KeyManagedValue, KeyManagedPresence,
	// KeyManagedArrayUnion.
	Mode JSONKeyMode
	// Value computes the canonical value at init time. For static values,
	// the closure ignores ProjectParams.
	Value func(ProjectParams) (any, error)
}

// jsonPointerSet sets the value at `pointer` in `doc`, creating
// intermediate objects as needed. Implements RFC 6901 unescaping.
// `doc` must be a map[string]any (or nested maps); array indices are
// not supported because the framework only addresses object keys.
func jsonPointerSet(doc map[string]any, pointer string, value any) error {
	tokens, err := jsonPointerTokens(pointer)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return fmt.Errorf("jsonPointerSet: empty pointer")
	}
	cur := doc
	for i, tok := range tokens {
		if i == len(tokens)-1 {
			cur[tok] = value
			return nil
		}
		next, ok := cur[tok]
		if !ok {
			fresh := map[string]any{}
			cur[tok] = fresh
			cur = fresh
			continue
		}
		nextMap, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("jsonPointerSet: %s segment %q is non-object %T", pointer, tok, next)
		}
		cur = nextMap
	}
	return nil
}

// jsonPointerGet returns the value at `pointer` in `doc` and a presence flag.
func jsonPointerGet(doc map[string]any, pointer string) (any, bool) {
	tokens, err := jsonPointerTokens(pointer)
	if err != nil || len(tokens) == 0 {
		return nil, false
	}
	var cur any = doc
	for _, tok := range tokens {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[tok]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// jsonPointerTokens splits an RFC 6901 JSON Pointer into segments and
// unescapes ~1 → / and ~0 → ~ (order matters per the RFC: unescape ~1
// before ~0 to avoid double-unescaping).
func jsonPointerTokens(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("jsonPointerTokens: %q must start with /", pointer)
	}
	parts := strings.Split(pointer[1:], "/")
	for i, p := range parts {
		p = strings.ReplaceAll(p, "~1", "/")
		p = strings.ReplaceAll(p, "~0", "~")
		parts[i] = p
	}
	return parts, nil
}
```

- [ ] **Step 4: Add `JSONKeys` field to `ManagedFile`**

In `internal/config/managedfiles/types.go`, after the existing `Build func(ProjectParams) ([]byte, error)` field, add:

```go
	// JSONKeys is the declarative form of managed JSON keys for
	// JSONKeyMerge entries. Mutually exclusive with Build (validator
	// enforces XOR). Only meaningful for StrategyJSONKeyMerge.
	JSONKeys []JSONManagedKey
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/config/managedfiles/ -run TestJSONPointer -v && go build ./...`
Expected: PASS for both pointer tests; build succeeds.

- [ ] **Step 6: Commit**

```bash
jj describe -m "feat(managedfiles): add JSONKeyMode, JSONManagedKey, and JSON Pointer helpers

Foundations for PR E's JSONKeyMerge alignment migration. Defines three
key modes (Value, Presence, ArrayUnion), a JSONManagedKey struct with
a closure-valued Value field, and minimal RFC 6901 JSON Pointer
set/get helpers. No strategy code uses these yet; subsequent tasks
wire them through jsonkeymerge.go.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 2: Implement `KeyManagedValue` mode in `jsonKeyMergeCanonical`

**Files:**

- Modify: `internal/config/managedfiles/jsonkeymerge.go:106-137`
- Test: `internal/config/managedfiles/jsonkeymerge_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/config/managedfiles/jsonkeymerge_test.go`:

```go
func TestJSONKeyMerge_KeyManagedValue_Basic(t *testing.T) {
	mf := ManagedFile{
		Path:     "test.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		JSONKeys: []JSONManagedKey{
			{
				Path: "/managed/value",
				Mode: KeyManagedValue,
				Value: func(_ ProjectParams) (any, error) {
					return "canonical", nil
				},
			},
		},
	}
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	if err := os.WriteFile(full, []byte(`{"unrelated":"keep","managed":{"value":"old"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("got action %v, want Refreshed", res.Action)
	}
	got, _ := os.ReadFile(full)
	var doc map[string]any
	_ = json.Unmarshal(got, &doc)
	if doc["unrelated"] != "keep" {
		t.Errorf("unrelated key clobbered: %v", doc)
	}
	if m, _ := doc["managed"].(map[string]any); m["value"] != "canonical" {
		t.Errorf("managed key not refreshed: %v", doc)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/managedfiles/ -run TestJSONKeyMerge_KeyManagedValue -v`
Expected: FAIL — strategy ignores `JSONKeys` and produces empty patch from nil `Build`.

- [ ] **Step 3: Rewrite `jsonKeyMergeCanonical`**

Replace the body of `jsonKeyMergeCanonical` in `internal/config/managedfiles/jsonkeymerge.go` (currently lines 106-137):

```go
// jsonKeyMergeCanonical computes the canonical disk content for an entry.
// Routes on whether the entry uses the new declarative JSONKeys field or
// the legacy Build closure. JSONKeys path handles KeyManagedValue
// (merge-patch), KeyManagedPresence (preserve existing), and
// KeyManagedArrayUnion (set-union with existing array).
//
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func jsonKeyMergeCanonical(existing []byte, mf ManagedFile, params ProjectParams) ([]byte, error) {
	if len(mf.JSONKeys) > 0 {
		return jsonKeyMergeCanonicalFromKeys(existing, mf, params)
	}
	return jsonKeyMergeCanonicalFromBuild(existing, mf, params)
}

// jsonKeyMergeCanonicalFromBuild is the pre-PR-E legacy path retained
// transitionally until the three remaining Build-style entries migrate.
// Task 7 removes it.
//
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func jsonKeyMergeCanonicalFromBuild(existing []byte, mf ManagedFile, params ProjectParams) ([]byte, error) {
	patch, err := mf.Build(params)
	if err != nil {
		return nil, fmt.Errorf("build patch for %s: %w", mf.Path, err)
	}
	src := existing
	if len(src) == 0 {
		src = []byte(`{}`)
	}
	merged, err := jsonpatch.MergePatch(src, patch)
	if err != nil {
		return nil, fmt.Errorf("merge patch %s: %w", mf.Path, err)
	}
	canonical, err := canonicalize(merged)
	if err != nil {
		return nil, err
	}
	if mf.Path == "opencode.json" {
		canonical, err = unionPluginArray(existing, canonical)
		if err != nil {
			return nil, fmt.Errorf("union plugin array for %s: %w", mf.Path, err)
		}
	}
	return canonical, nil
}

//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func jsonKeyMergeCanonicalFromKeys(existing []byte, mf ManagedFile, params ProjectParams) ([]byte, error) {
	src := existing
	if len(src) == 0 {
		src = []byte(`{}`)
	}
	// Phase 1: build patch from KeyManagedValue keys.
	patch := map[string]any{}
	for _, k := range mf.JSONKeys {
		if k.Mode != KeyManagedValue {
			continue
		}
		v, err := k.Value(params)
		if err != nil {
			return nil, fmt.Errorf("value for %s: %w", k.Path, err)
		}
		if err := jsonPointerSet(patch, k.Path, v); err != nil {
			return nil, fmt.Errorf("set %s: %w", k.Path, err)
		}
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshal patch for %s: %w", mf.Path, err)
	}
	merged, err := jsonpatch.MergePatch(src, patchBytes)
	if err != nil {
		return nil, fmt.Errorf("merge patch %s: %w", mf.Path, err)
	}
	// Subsequent phases (Presence, ArrayUnion) added in Tasks 3 and 4.
	return canonicalize(merged)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/managedfiles/ -run TestJSONKeyMerge -v`
Expected: PASS for the new test; existing `Build`-path tests still PASS.

- [ ] **Step 5: Commit**

```bash
jj describe -m "feat(managedfiles): implement KeyManagedValue mode in jsonKeyMergeCanonical

Routes JSONKeyMerge entries with non-empty JSONKeys through a new
declarative path. KeyManagedValue keys are collected into a merge
patch via jsonPointerSet, then applied with RFC 7396 MergePatch.
The legacy Build path is retained as jsonKeyMergeCanonicalFromBuild
for transitional compatibility; Task 7 removes it after all three
existing entries migrate.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 3: Implement `KeyManagedPresence` mode

**Files:**

- Modify: `internal/config/managedfiles/jsonkeymerge.go` (extend `jsonKeyMergeCanonicalFromKeys`)
- Test: `internal/config/managedfiles/jsonkeymerge_test.go`

- [ ] **Step 1: Write the failing tests**

Append three test cases to `jsonkeymerge_test.go`:

```go
func TestJSONKeyMerge_KeyManagedPresence_WriteIfAbsent(t *testing.T) {
	mf := presenceTestEntry(t, true)
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	_ = os.WriteFile(full, []byte(`{}`), 0o644)
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), `"specgraph@specgraph-local":true`) {
		t.Errorf("expected key written with canonical value; got %s", got)
	}
}

func TestJSONKeyMerge_KeyManagedPresence_PreservesUserFalse(t *testing.T) {
	mf := presenceTestEntry(t, true)
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	_ = os.WriteFile(full, []byte(`{"enabledPlugins":{"specgraph@specgraph-local":false}}`), 0o644)
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), `"specgraph@specgraph-local":false`) {
		t.Errorf("expected user's false to be preserved; got %s", got)
	}
}

func TestJSONKeyMerge_KeyManagedPresence_PreservesUserCustomValue(t *testing.T) {
	mf := presenceTestEntry(t, true)
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	_ = os.WriteFile(full, []byte(`{"enabledPlugins":{"specgraph@specgraph-local":"custom"}}`), 0o644)
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), `"specgraph@specgraph-local":"custom"`) {
		t.Errorf("expected user's custom value to be preserved; got %s", got)
	}
}

func presenceTestEntry(t *testing.T, defaultValue bool) ManagedFile {
	t.Helper()
	return ManagedFile{
		Path:     "test.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		JSONKeys: []JSONManagedKey{
			{
				Path: "/enabledPlugins/specgraph@specgraph-local",
				Mode: KeyManagedPresence,
				Value: func(_ ProjectParams) (any, error) {
					return defaultValue, nil
				},
			},
		},
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/managedfiles/ -run TestJSONKeyMerge_KeyManagedPresence -v`
Expected: FAIL — `KeyManagedPresence` is in the enum but the strategy ignores it.

- [ ] **Step 3: Add presence phase to `jsonKeyMergeCanonicalFromKeys`**

In `jsonkeymerge.go`, before the final `return canonicalize(merged)` of `jsonKeyMergeCanonicalFromKeys`, add:

```go
	// Phase 2: KeyManagedPresence — write if absent, preserve if present.
	var existingDoc map[string]any
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &existingDoc); err != nil {
			return nil, fmt.Errorf("unmarshal existing %s: %w", mf.Path, err)
		}
	}
	var mergedDoc map[string]any
	if err := json.Unmarshal(merged, &mergedDoc); err != nil {
		return nil, fmt.Errorf("unmarshal merged %s: %w", mf.Path, err)
	}
	for _, k := range mf.JSONKeys {
		if k.Mode != KeyManagedPresence {
			continue
		}
		if existingValue, present := jsonPointerGet(existingDoc, k.Path); present {
			if err := jsonPointerSet(mergedDoc, k.Path, existingValue); err != nil {
				return nil, fmt.Errorf("preserve %s: %w", k.Path, err)
			}
			continue
		}
		v, err := k.Value(params)
		if err != nil {
			return nil, fmt.Errorf("value for %s: %w", k.Path, err)
		}
		if err := jsonPointerSet(mergedDoc, k.Path, v); err != nil {
			return nil, fmt.Errorf("set %s: %w", k.Path, err)
		}
	}
	merged, err = json.Marshal(mergedDoc)
	if err != nil {
		return nil, fmt.Errorf("remarshal %s: %w", mf.Path, err)
	}
```

(Replace `return canonicalize(merged)` with this block followed by the original return.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/managedfiles/ -run TestJSONKeyMerge -v`
Expected: PASS for all three presence cases plus the earlier value case.

- [ ] **Step 5: Commit**

```bash
jj describe -m "feat(managedfiles): implement KeyManagedPresence mode

After the merge-patch phase, walk KeyManagedPresence keys: if the
path exists in the pre-merge document, copy that value into the
merged result (preserving user choice); else write the canonical
Value. This is how 'specgraph init' will leave a user's
/plugin disable specgraph alone on subsequent runs.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 4: Implement `KeyManagedArrayUnion` mode

**Files:**

- Modify: `internal/config/managedfiles/jsonkeymerge.go` (extend `jsonKeyMergeCanonicalFromKeys`)
- Test: `internal/config/managedfiles/jsonkeymerge_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `jsonkeymerge_test.go`:

```go
func TestJSONKeyMerge_KeyManagedArrayUnion_AbsentArray(t *testing.T) {
	mf := arrayUnionTestEntry()
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	_ = os.WriteFile(full, []byte(`{}`), 0o644)
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), `"./.specgraph/agents/opencode/specgraph.ts"`) {
		t.Errorf("expected canonical element written; got %s", got)
	}
}

func TestJSONKeyMerge_KeyManagedArrayUnion_DisjointUnion(t *testing.T) {
	mf := arrayUnionTestEntry()
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	_ = os.WriteFile(full, []byte(`{"plugin":["./user-plugin.ts"]}`), 0o644)
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), `"./user-plugin.ts"`) ||
		!strings.Contains(string(got), `"./.specgraph/agents/opencode/specgraph.ts"`) {
		t.Errorf("expected both elements present; got %s", got)
	}
}

func TestJSONKeyMerge_KeyManagedArrayUnion_DedupesOverlap(t *testing.T) {
	mf := arrayUnionTestEntry()
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	_ = os.WriteFile(full,
		[]byte(`{"plugin":["./.specgraph/agents/opencode/specgraph.ts","./user.ts"]}`),
		0o644)
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	var doc struct {
		Plugin []string `json:"plugin"`
	}
	_ = json.Unmarshal(got, &doc)
	if len(doc.Plugin) != 2 {
		t.Errorf("expected 2 unique elements, got %d: %v", len(doc.Plugin), doc.Plugin)
	}
}

func arrayUnionTestEntry() ManagedFile {
	return ManagedFile{
		Path:     "test.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessOpenCode,
		JSONKeys: []JSONManagedKey{
			{
				Path: "/plugin",
				Mode: KeyManagedArrayUnion,
				Value: func(_ ProjectParams) (any, error) {
					return []any{"./.specgraph/agents/opencode/specgraph.ts"}, nil
				},
			},
		},
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/managedfiles/ -run TestJSONKeyMerge_KeyManagedArrayUnion -v`
Expected: FAIL — strategy ignores `KeyManagedArrayUnion`.

- [ ] **Step 3: Add array-union phase**

In `jsonkeymerge.go`, just before the final `merged, err = json.Marshal(mergedDoc)` line added in Task 3, add:

```go
	// Phase 3: KeyManagedArrayUnion — union with existing array (DeepEqual dedupe).
	for _, k := range mf.JSONKeys {
		if k.Mode != KeyManagedArrayUnion {
			continue
		}
		canonicalAny, err := k.Value(params)
		if err != nil {
			return nil, fmt.Errorf("value for %s: %w", k.Path, err)
		}
		canonicalSlice, ok := canonicalAny.([]any)
		if !ok {
			return nil, fmt.Errorf("ArrayUnion value for %s must be []any, got %T", k.Path, canonicalAny)
		}
		var existingSlice []any
		if v, present := jsonPointerGet(existingDoc, k.Path); present {
			if s, ok := v.([]any); ok {
				existingSlice = s
			}
		}
		unioned := append([]any{}, existingSlice...)
		for _, c := range canonicalSlice {
			seen := false
			for _, e := range unioned {
				if reflect.DeepEqual(c, e) {
					seen = true
					break
				}
			}
			if !seen {
				unioned = append(unioned, c)
			}
		}
		if err := jsonPointerSet(mergedDoc, k.Path, unioned); err != nil {
			return nil, fmt.Errorf("set %s: %w", k.Path, err)
		}
	}
```

Add `"reflect"` to the imports at the top of the file if not already present.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/managedfiles/ -run TestJSONKeyMerge -v`
Expected: PASS for all KeyManagedArrayUnion cases plus prior tests.

- [ ] **Step 5: Commit**

```bash
jj describe -m "feat(managedfiles): implement KeyManagedArrayUnion mode

Formalizes the unionPluginArray special case that previously lived
as a path-keyed hook in jsonKeyMergeCanonical. Array elements are
deduplicated with reflect.DeepEqual; canonical elements not already
present are appended after the existing user elements.

The legacy unionPluginArray hook is removed in Task 6 when
opencode.json migrates onto KeyManagedArrayUnion.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 5: Migrate `.mcp.json` and `.cursor/mcp.json` to `JSONKeys`

**Files:**

- Modify: `internal/config/managedfiles/manifest.go` (the two entries + delete `buildClaudeMCPJSON`, `buildCursorMCPJSON` closures)
- Test: new `internal/config/managedfiles/jsonkeymerge_migration_test.go`

- [ ] **Step 1: Write the failing golden tests**

Create `internal/config/managedfiles/jsonkeymerge_migration_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigratedMCPJSONsMatchLegacyOutput(t *testing.T) {
	params := ProjectParams{
		Slug:      "test-project",
		ServerURL: "https://specgraph.example.com",
	}
	cases := []struct {
		path  string
		entry func() ManagedFile
		// goldenBuild is the EXACT bytes the pre-migration Build closure
		// produced for these params. Captured by running the legacy
		// closure once and freezing the output.
		goldenBuild []byte
	}{
		{
			path:        ".mcp.json",
			entry:       func() ManagedFile { return findManifestEntry(t, ".mcp.json") },
			goldenBuild: legacyBuildClaudeMCPJSON(params),
		},
		{
			path:        ".cursor/mcp.json",
			entry:       func() ManagedFile { return findManifestEntry(t, ".cursor/mcp.json") },
			goldenBuild: legacyBuildCursorMCPJSON(params),
		},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			dir := t.TempDir()
			full := filepath.Join(dir, tc.path)
			_ = os.MkdirAll(filepath.Dir(full), 0o755)
			mf := tc.entry()
			res, err := jsonKeyMergeStrategy{}.Sync(dir, mf, params, SyncOptions{})
			if err != nil {
				t.Fatalf("sync: %v", err)
			}
			if res.Err != nil {
				t.Fatalf("sync result error: %v", res.Err)
			}
			got, _ := os.ReadFile(full)
			var gotDoc, wantDoc any
			_ = json.Unmarshal(got, &gotDoc)
			_ = json.Unmarshal(applyLegacyToEmpty(tc.goldenBuild), &wantDoc)
			gotCanon, _ := json.Marshal(gotDoc)
			wantCanon, _ := json.Marshal(wantDoc)
			if !bytes.Equal(gotCanon, wantCanon) {
				t.Errorf("migrated output differs from legacy:\n got:  %s\n want: %s", gotCanon, wantCanon)
			}
		})
	}
}

// legacyBuildClaudeMCPJSON / legacyBuildCursorMCPJSON are exact copies of
// the pre-migration Build closures, kept ONLY in this test file as the
// regression oracle. Delete them once the migration has been merged for
// one release cycle and we're confident in the new output.
func legacyBuildClaudeMCPJSON(p ProjectParams) []byte {
	b, _ := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"type": "http",
				"url":  ensureMCPSuffix(p.ServerURL),
				"headers": map[string]any{
					"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": p.Slug,
				},
			},
		},
	})
	return b
}

func legacyBuildCursorMCPJSON(p ProjectParams) []byte {
	b, _ := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"url": ensureMCPSuffix(p.ServerURL),
				"headers": map[string]any{
					"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": p.Slug,
				},
			},
		},
	})
	return b
}

func applyLegacyToEmpty(patch []byte) []byte {
	// MergePatch({}, patch) → patch itself for top-level-object patches.
	return patch
}

func findManifestEntry(t *testing.T, path string) ManagedFile {
	t.Helper()
	for _, mf := range Manifest() {
		if mf.Path == path {
			return mf
		}
	}
	t.Fatalf("manifest entry %q not found", path)
	return ManagedFile{}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/managedfiles/ -run TestMigratedMCPJSONsMatchLegacyOutput -v`
Expected: FAIL — entries still use `Build`; migrated output is empty/wrong.

- [ ] **Step 3: Migrate the two entries**

In `internal/config/managedfiles/manifest.go`, replace the existing two entries (the `.mcp.json` and `.cursor/mcp.json` blocks around lines 28-50) with:

```go
		{
			Path:     ".mcp.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessClaude,
			JSONKeys: []JSONManagedKey{
				{
					Path: "/mcpServers/specgraph",
					Mode: KeyManagedValue,
					Value: func(p ProjectParams) (any, error) {
						return map[string]any{
							"type": "http",
							"url":  ensureMCPSuffix(p.ServerURL),
							"headers": map[string]any{
								"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
								"X-Specgraph-Project": p.Slug,
							},
						}, nil
					},
				},
			},
		},
		{
			Path:     ".cursor/mcp.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessCursor,
			JSONKeys: []JSONManagedKey{
				{
					Path: "/mcpServers/specgraph",
					Mode: KeyManagedValue,
					Value: func(p ProjectParams) (any, error) {
						return map[string]any{
							"url": ensureMCPSuffix(p.ServerURL),
							"headers": map[string]any{
								"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
								"X-Specgraph-Project": p.Slug,
							},
						}, nil
					},
				},
			},
		},
```

Delete the `buildClaudeMCPJSON` and `buildCursorMCPJSON` functions (lines ~160-195).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/managedfiles/... -v`
Expected: ALL existing JSONKeyMerge tests still PASS; new migration test PASSES; no test fails because the closures were deleted.

- [ ] **Step 5: Commit**

```bash
jj describe -m "refactor(managedfiles): migrate .mcp.json and .cursor/mcp.json onto JSONKeys

Both entries managed exactly one nested key (/mcpServers/specgraph)
via the recursive object-merge semantics of RFC 7396. The declarative
form makes that explicit in the manifest. Golden tests assert
byte-identical output to the pre-migration Build closures across the
ProjectParams matrix.

The two Build closures (buildClaudeMCPJSON, buildCursorMCPJSON) are
deleted. opencode.json still uses Build until Task 6.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 6: Migrate `opencode.json`; remove `unionPluginArray` hook

**Files:**

- Modify: `internal/config/managedfiles/manifest.go` (the entry + delete `buildOpenCodeJSON`)
- Modify: `internal/config/managedfiles/jsonkeymerge.go` (remove `unionPluginArray` from `jsonKeyMergeCanonicalFromBuild`)
- Modify: `internal/config/managedfiles/unionarray.go` (delete or shrink)
- Test: extend `jsonkeymerge_migration_test.go`

- [ ] **Step 1: Add the opencode.json golden test case**

In `jsonkeymerge_migration_test.go`, extend the `cases` slice in `TestMigratedMCPJSONsMatchLegacyOutput`. Also add specific union-tests:

```go
func TestMigratedOpenCodeJSON_PreservesPluginUnion(t *testing.T) {
	params := ProjectParams{Slug: "test", ServerURL: "https://x.example"}
	dir := t.TempDir()
	full := filepath.Join(dir, "opencode.json")
	// Pre-existing opencode.json with a user-added plugin.
	_ = os.WriteFile(full,
		[]byte(`{"plugin":["./user-plugin.ts"]}`),
		0o644)
	mf := findManifestEntry(t, "opencode.json")
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), `"./user-plugin.ts"`) {
		t.Errorf("user plugin entry lost: %s", got)
	}
	if !strings.Contains(string(got), `"./.specgraph/agents/opencode/specgraph.ts"`) {
		t.Errorf("canonical plugin entry not added: %s", got)
	}
}
```

Add the `opencode.json` case to the golden-output slice as well:

```go
{
    path:        "opencode.json",
    entry:       func() ManagedFile { return findManifestEntry(t, "opencode.json") },
    goldenBuild: legacyBuildOpenCodeJSON(params),
},
```

with a `legacyBuildOpenCodeJSON` companion mirroring the pattern.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/config/managedfiles/ -run TestMigratedOpenCodeJSON -v`
Expected: FAIL — entry still uses Build.

- [ ] **Step 3: Migrate the entry**

Replace the `opencode.json` block in `manifest.go`:

```go
		{
			Path:     "opencode.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessOpenCode,
			JSONKeys: []JSONManagedKey{
				{
					Path: "/$schema",
					Mode: KeyManagedValue,
					Value: func(_ ProjectParams) (any, error) {
						return "https://opencode.ai/config.json", nil
					},
				},
				{
					Path: "/mcp/specgraph",
					Mode: KeyManagedValue,
					Value: func(p ProjectParams) (any, error) {
						return map[string]any{
							"type":    "remote",
							"url":     ensureMCPSuffix(p.ServerURL),
							"enabled": true,
							"headers": map[string]any{
								"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
								"X-Specgraph-Project": p.Slug,
							},
						}, nil
					},
				},
				{
					Path: "/plugin",
					Mode: KeyManagedArrayUnion,
					Value: func(_ ProjectParams) (any, error) {
						return []any{"./.specgraph/agents/opencode/specgraph.ts"}, nil
					},
				},
			},
		},
```

Delete the `buildOpenCodeJSON` function.

- [ ] **Step 4: Remove the path-keyed unionPluginArray hook**

In `jsonkeymerge.go`'s `jsonKeyMergeCanonicalFromBuild`, remove the entire block:

```go
	if mf.Path == "opencode.json" {
		canonical, err = unionPluginArray(existing, canonical)
		if err != nil {
			return nil, fmt.Errorf("union plugin array for %s: %w", mf.Path, err)
		}
	}
```

The `unionPluginArray` function in `unionarray.go` is no longer referenced. Either delete the file or keep just the test if it has independent value for ArrayUnion regression coverage. Recommend: delete the function but move its test logic into `TestJSONKeyMerge_KeyManagedArrayUnion_*`.

- [ ] **Step 5: Run all tests**

Run: `task check`
Expected: All managedfiles tests pass; build succeeds; lint clean.

- [ ] **Step 6: Commit**

```bash
jj describe -m "refactor(managedfiles): migrate opencode.json onto JSONKeys; remove unionPluginArray hook

opencode.json manages three keys: /\$schema (Value, static),
/mcp/specgraph (Value, dynamic), /plugin (ArrayUnion, static). The
path-keyed unionPluginArray post-merge hook in jsonKeyMergeCanonical
is no longer reachable and is deleted. Existing union-merge tests
from unionarray_test.go are subsumed by TestJSONKeyMerge_KeyManagedArrayUnion_*.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 7: Tighten validator — `JSONKeyMerge` requires `JSONKeys`

**Files:**

- Modify: `internal/config/managedfiles/manifest.go` (the `validateManifestEntry` function)
- Modify: `internal/config/managedfiles/jsonkeymerge.go` (delete `jsonKeyMergeCanonicalFromBuild`)
- Test: extend `manifest_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `manifest_test.go`:

```go
func TestValidator_JSONKeyMergeRequiresJSONKeys(t *testing.T) {
	mf := ManagedFile{
		Path:     "x.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		Build:    func(_ ProjectParams) ([]byte, error) { return []byte(`{}`), nil },
	}
	if err := validateManifestEntry(mf); err == nil {
		t.Error("expected validator to reject JSONKeyMerge with Build (post-PR-E)")
	}
}

func TestValidator_JSONKeyMergeAcceptsJSONKeys(t *testing.T) {
	mf := ManagedFile{
		Path:     "x.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		JSONKeys: []JSONManagedKey{
			{Path: "/foo", Mode: KeyManagedValue, Value: func(_ ProjectParams) (any, error) { return "bar", nil }},
		},
	}
	if err := validateManifestEntry(mf); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidator_JSONKeysOnWholeFileRejected(t *testing.T) {
	mf := ManagedFile{
		Path:     "x.md",
		Strategy: StrategyWholeFile,
		Comment:  CommentHTML,
		Harness:  HarnessClaude,
		Source:   "embedded/x.md",
		JSONKeys: []JSONManagedKey{{Path: "/x", Mode: KeyManagedValue, Value: func(_ ProjectParams) (any, error) { return "", nil }}},
	}
	if err := validateManifestEntry(mf); err == nil {
		t.Error("expected validator to reject JSONKeys on WholeFile")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/config/managedfiles/ -run TestValidator -v`
Expected: FAIL — current validator accepts `Build` on JSONKeyMerge and tolerates `JSONKeys` on WholeFile.

- [ ] **Step 3: Tighten the validator**

In `manifest.go`'s `validateManifestEntry`, replace the existing strategy-specific block with:

```go
	hasBuild := mf.Build != nil
	hasJSONKeys := len(mf.JSONKeys) > 0
	hasSource := mf.Source != ""
	switch mf.Strategy {
	case StrategyJSONKeyMerge:
		if !hasJSONKeys {
			return fmt.Errorf("manifest entry %q: JSONKeyMerge strategy requires JSONKeys", mf.Path)
		}
		if hasBuild {
			return fmt.Errorf("manifest entry %q: JSONKeyMerge strategy must not set Build (use JSONKeys)", mf.Path)
		}
		if hasSource {
			return fmt.Errorf("manifest entry %q: JSONKeyMerge strategy must not set Source", mf.Path)
		}
	case StrategyMarkdownBlock:
		if !hasBuild {
			return fmt.Errorf("manifest entry %q: MarkdownBlock strategy requires Build", mf.Path)
		}
		if hasJSONKeys {
			return fmt.Errorf("manifest entry %q: MarkdownBlock strategy must not set JSONKeys", mf.Path)
		}
	case StrategyWholeFile:
		if !hasSource {
			return fmt.Errorf("manifest entry %q: WholeFile strategy requires Source", mf.Path)
		}
		if hasBuild || hasJSONKeys {
			return fmt.Errorf("manifest entry %q: WholeFile strategy must not set Build or JSONKeys", mf.Path)
		}
	}
```

- [ ] **Step 4: Delete `jsonKeyMergeCanonicalFromBuild`**

Now that no entry can use Build with JSONKeyMerge, the legacy function in `jsonkeymerge.go` is unreachable. Delete `jsonKeyMergeCanonicalFromBuild`, and rewrite `jsonKeyMergeCanonical` as:

```go
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func jsonKeyMergeCanonical(existing []byte, mf ManagedFile, params ProjectParams) ([]byte, error) {
	return jsonKeyMergeCanonicalFromKeys(existing, mf, params)
}
```

(Or inline `jsonKeyMergeCanonicalFromKeys` into `jsonKeyMergeCanonical` directly — the wrapper-rename will read cleaner.)

- [ ] **Step 5: Run all tests**

Run: `task check`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
jj describe -m "refactor(managedfiles): tighten validator — JSONKeyMerge requires JSONKeys

With all three existing JSONKeyMerge entries migrated, the legacy
Build path is unreachable. The validator now rejects Build on
JSONKeyMerge entries and JSONKeys on non-JSONKeyMerge entries.
The transitional jsonKeyMergeCanonicalFromBuild function is deleted.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 8: Allow `WholeFile + CommentNone`; update validator to the 5-combo positive shape

**Files:**

- Modify: `internal/config/managedfiles/manifest.go` (validator)
- Modify: `internal/config/managedfiles/manifest_test.go` (flip negative anchor → positive shape)
- Modify: `internal/config/managedfiles/wholefile.go` (handle CommentNone in Inspect/Sync; add `keepEditsBody*` helpers)
- Test: new `internal/config/managedfiles/wholefile_json_test.go`

- [ ] **Step 1: Write the failing tests — CommentNone WholeFile path**

Create `internal/config/managedfiles/wholefile_json_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func wholeFileJSONTestEntry(canonical []byte) ManagedFile {
	src := registerTestSource(canonical, "test.json")
	return ManagedFile{
		Path:     "test.json",
		Strategy: StrategyWholeFile,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		Source:   src,
	}
}

func TestWholeFileJSON_Missing(t *testing.T) {
	canonical := []byte(`{"hello":"world"}`)
	mf := wholeFileJSONTestEntry(canonical)
	dir := t.TempDir()
	res, err := wholeFileStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("got action %v, want Created", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "test.json"))
	if string(got) != string(canonical) {
		t.Errorf("got %q, want %q", got, canonical)
	}
}

func TestWholeFileJSON_Synced(t *testing.T) {
	canonical := []byte(`{"hello":"world"}`)
	mf := wholeFileJSONTestEntry(canonical)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "test.json"), canonical, 0o644)
	state, err := wholeFileStrategy{}.Inspect(dir, mf, ProjectParams{})
	if err != nil {
		t.Fatal(err)
	}
	if state.State != StateSynced {
		t.Errorf("got state %v, want Synced", state.State)
	}
}

func TestWholeFileJSON_DriftedUserowned(t *testing.T) {
	canonical := []byte(`{"hello":"world"}`)
	mf := wholeFileJSONTestEntry(canonical)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "test.json"), []byte(`{"hello":"hacked"}`), 0o644)
	res, err := wholeFileStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionSkipped {
		t.Errorf("got action %v, want Skipped (drifted, user-owned)", res.Action)
	}
	if !strings.Contains(res.Detail, DriftDetailNoSentinel) {
		t.Errorf("expected detail to contain %q, got %q", DriftDetailNoSentinel, res.Detail)
	}
}

// registerTestSource registers `body` in the test source registry and
// returns the source key. Implemented as a tiny in-memory fs.FS wired
// through the existing test source mechanism.
func registerTestSource(body []byte, name string) string {
	// Implementation note: source.go has a test-only registry; reuse it.
	// If absent, add a memberTestFS that satisfies the source loader.
	// See existing wholefile_test.go for the pattern.
	return registerInMemorySource(body, name) // see helper in source_test.go
}
```

If `registerInMemorySource` doesn't exist yet, add it as a `testing.T`-scoped helper in `source_test.go` that registers bytes into an in-memory FS used by `wholeFileStrategy`. Pattern: similar to how `wholefile_test.go` already constructs source entries.

- [ ] **Step 2: Write the failing test — 5-combo positive shape**

Replace the negative-anchor test in `manifest_test.go:194-202`:

```go
func TestManifestValidator_WholeFileSupportedCombinations(t *testing.T) {
	type combo struct {
		comment       CommentSyntax
		hasFm         bool
		name          string
	}
	supported := map[combo]bool{
		{CommentNone, false, "JSON"}:                   true,
		{CommentHash, false, "shell/Python/YAML"}:      true,
		{CommentSlash, false, "TypeScript/JavaScript"}: true,
		{CommentHTML, false, "plain Markdown"}:         true,
		{CommentHTML, true, "Markdown w/ frontmatter"}: true,
	}
	for _, mf := range Manifest() {
		if mf.Strategy != StrategyWholeFile {
			continue
		}
		k := combo{comment: mf.Comment, hasFm: mf.HasFrontmatter}
		k.name = "" // ignore name when looking up
		var matched bool
		for sk := range supported {
			if sk.comment == k.comment && sk.hasFm == k.hasFm {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("entry %q: unsupported WholeFile combo Comment=%v HasFrontmatter=%v",
				mf.Path, mf.Comment, mf.HasFrontmatter)
		}
	}
}
```

- [ ] **Step 3: Run tests to verify failure**

Run: `go test ./internal/config/managedfiles/ -run 'TestWholeFileJSON|TestManifestValidator_WholeFileSupportedCombinations' -v`
Expected: FAIL — `WholeFile+CommentNone` rejected; new positive shape test passes vacuously (no JSON WholeFile entries yet) but `wholefile.go` doesn't handle CommentNone.

- [ ] **Step 4: Update validator — keep only the HasFm+CommentNone rejection**

In `manifest.go`'s `validateManifestEntry`, replace the `if mf.HasFrontmatter { ... }` block with:

```go
	if mf.Strategy == StrategyWholeFile {
		if mf.HasFrontmatter && mf.Comment == CommentNone {
			return fmt.Errorf("manifest entry %q: HasFrontmatter requires a non-None comment style", mf.Path)
		}
		// Supported combinations:
		//   CommentNone  + !HasFrontmatter → JSON files (no in-file sentinel)   [PR E]
		//   CommentHash  + !HasFrontmatter → shell / Python / YAML scripts
		//   CommentSlash + !HasFrontmatter → TypeScript / JS plugin source      [PR C]
		//   CommentHTML  + !HasFrontmatter → plain Markdown                     [PR E]
		//   CommentHTML  +  HasFrontmatter → Markdown with leading frontmatter  [PR D]
	}
	if mf.HasFrontmatter && mf.Strategy != StrategyWholeFile {
		return fmt.Errorf("manifest entry %q: HasFrontmatter requires WholeFile strategy, got %s",
			mf.Path, mf.Strategy)
	}
```

The PR D anchor for `CommentHTML+!HasFm` is removed — that combo is now first-class (`routing-guide.md`).

- [ ] **Step 5: Extract `keepEditsBodyForFrontmatter` / `keepEditsBodyPlain` helpers**

In `wholefile.go`, find the two body-extraction branches in `Sync` (introduced in PR D for frontmatter handling). Pull each into its own function:

```go
// keepEditsBodyForFrontmatter extracts the user-owned body from a file
// that carries a YAML frontmatter block followed by a sentinel line.
// Returns the body bytes (everything after the sentinel) so the caller
// can re-emit them after rewriting the sentinel hash.
func keepEditsBodyForFrontmatter(existing []byte, sentinelIdx int) []byte {
	// ... existing inline logic ...
}

// keepEditsBodyPlain extracts the user-owned body from a file that
// carries a leading sentinel line (no frontmatter). Returns everything
// after the sentinel.
func keepEditsBodyPlain(existing []byte, sentinelIdx int) []byte {
	// ... existing inline logic ...
}
```

The Sync function calls the appropriate helper based on `mf.HasFrontmatter`. Pure refactor — no behavior change.

- [ ] **Step 6: Add `CommentNone` path in `wholeFileStrategy.Sync` and `.Inspect`**

In `wholefile.go`, near the top of both `Sync` and `Inspect`, after reading the canonical and the existing file, route on `mf.Comment == CommentNone`:

```go
	if mf.Comment == CommentNone {
		// JSON-files path: no in-file sentinel; state classification via
		// byte-hash comparison against canonical + priors registry.
		return wholeFileSyncNoSentinel(cwd, mf, canonical, existing, fileExisted)
	}
```

Implement `wholeFileSyncNoSentinel` (for `Sync`) and a sibling `wholeFileInspectNoSentinel` (for `Inspect`). Sync logic:

```go
func wholeFileSyncNoSentinel(cwd string, mf ManagedFile, canonical, existing []byte, fileExisted bool) (SyncResult, error) {
	full := filepath.Join(cwd, mf.Path)
	if !fileExisted {
		if err := writeAtomic(full, canonical, 0o644); err != nil {
			return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil //nolint:nilerr
		}
		return SyncResult{Path: mf.Path, Action: ActionCreated}, nil
	}
	existingHash := hex.EncodeToString(sha256Sum(existing))
	canonicalHash := hex.EncodeToString(sha256Sum(canonical))
	if existingHash == canonicalHash {
		return SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: "synced"}, nil
	}
	for _, prior := range priorsFor(mf.Path) {
		if existingHash == prior {
			if err := writeAtomic(full, canonical, 0o644); err != nil {
				return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil //nolint:nilerr
			}
			return SyncResult{Path: mf.Path, Action: ActionRefreshed}, nil
		}
	}
	return SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: DriftDetailNoSentinel}, nil
}
```

`priorsFor` is referenced here but not yet defined; Task 9 provides the full registry. For now, add a minimal stub at the bottom of `wholefile.go` (or a new `priors_stub.go` deleted by Task 9):

```go
// priorsFor returns canonical hashes for `path` that should classify the
// on-disk file as Stale-managed rather than Drifted-userowned. Stubbed
// in this task; replaced by the full registry in Task 9.
func priorsFor(_ string) []string { return nil }
```

(Task 9 deletes this stub and replaces with the registered version.)

- [ ] **Step 7: Run all tests**

Run: `task check`
Expected: All tests pass including the new `TestWholeFileJSON_*` family.

- [ ] **Step 8: Commit**

```bash
jj describe -m "feat(managedfiles): support WholeFile + CommentNone (no-sentinel JSON path)

JSON files cannot carry sentinel comments. WholeFile + CommentNone is
now a supported combination: state classification compares byte
hashes against the canonical and a priors registry (stubbed in this
commit; populated in Task 9).

Also lifts the PR D anchor rejecting WholeFile + CommentHTML +
!HasFrontmatter — that combo is now first-class for plain Markdown
files like routing-guide.md.

The validator's WholeFile rules collapse to a single positive shape
check; the manifest test flips from negative anchor to positive
five-combo enumeration. Extracts keepEditsBodyForFrontmatter and
keepEditsBodyPlain helpers from Sync (pure refactor).

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 9: Unified priors registry

**Files:**

- Create: `internal/config/managedfiles/priors.go`
- Create: `internal/config/managedfiles/priors_test.go`
- Modify: `internal/config/managedfiles/vestigial_cursor_rules.go` (deprecate the standalone map; populate registry)
- Modify: `internal/config/managedfiles/wholefile.go` (use `priorsFor` in CommentNone path)
- Modify any other call sites of `vestigialCursorRulePriorHash`.

- [ ] **Step 1: Write the failing test**

Create `internal/config/managedfiles/priors_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "testing"

func TestPriorsRegistry_CursorRulesMigrated(t *testing.T) {
	// PR D's vestigial cursor-rule priors must remain accessible
	// through priorsFor after the unification.
	hashes := priorsFor(".cursor/rules/specgraph.mdc")
	if len(hashes) == 0 {
		t.Error("expected at least one prior hash for .cursor/rules/specgraph.mdc")
	}
}

func TestPriorsRegistry_UnknownPathEmpty(t *testing.T) {
	if priors := priorsFor("nonexistent/path"); len(priors) != 0 {
		t.Errorf("expected empty slice for unknown path, got %v", priors)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/config/managedfiles/ -run TestPriorsRegistry -v`
Expected: FAIL — `priorsFor` undefined (it's a stub from Task 8 if you inlined it; if not, this test discovers that).

- [ ] **Step 3: Implement the registry**

Create `internal/config/managedfiles/priors.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "sync"

// priorsRegistry maps managed-file path → list of canonical SHA256 hashes
// (hex-encoded) that should classify the on-disk file as Stale-managed
// rather than Drifted-userowned. Each entry corresponds to a canonical
// version that has shipped at some point in SpecGraph's history.
//
// Population: package-level init (in priors_init.go) registers all
// known priors. Both PR D's cursor-rule priors and PR E's JSON priors
// route through this registry.
type priorsRegistry struct {
	mu sync.RWMutex
	m  map[string][]string
}

var globalPriors = &priorsRegistry{m: map[string][]string{}}

// priorsFor returns the priors hashes registered for `path`. Empty slice
// if the path has no known priors. Always safe to call from any
// goroutine after package init.
func priorsFor(path string) []string {
	globalPriors.mu.RLock()
	defer globalPriors.mu.RUnlock()
	out := make([]string, len(globalPriors.m[path]))
	copy(out, globalPriors.m[path])
	return out
}

// registerPrior adds a prior hash for a managed-file path. Called from
// package init (priors_init.go) and the vestigial-cursor-rules
// translation. Idempotent: duplicate hashes are silently deduped.
func registerPrior(path, hashHex string) {
	globalPriors.mu.Lock()
	defer globalPriors.mu.Unlock()
	for _, h := range globalPriors.m[path] {
		if h == hashHex {
			return
		}
	}
	globalPriors.m[path] = append(globalPriors.m[path], hashHex)
}
```

- [ ] **Step 4: Migrate PR D's `vestigialCursorRulePriorHash` map**

In `vestigial_cursor_rules.go`, find the existing `vestigialCursorRulePriorHash` map. **First, read the current values out of that map verbatim** — they are SHA256 hex strings of the pre-rename canonical content. Replace the map with an `init()` function that registers each entry into `globalPriors`:

```go
func init() {
	// Hash values copied verbatim from the pre-migration
	// vestigialCursorRulePriorHash map. DO NOT recompute — those are
	// the exact hashes that classify the .md → .mdc rename's old
	// canonical content as Stale-managed. Changing them would
	// regress drift detection on existing user installs.
	priors := map[string]string{
		".cursor/rules/specgraph.mdc":            "<hash from PR D map>",
		".cursor/rules/specgraph-post-stage.mdc": "<hash from PR D map>",
	}
	for path, hashHex := range priors {
		registerPrior(path, hashHex)
	}
}
```

Replace each `"<hash from PR D map>"` placeholder with the exact hex string from the deleted map's corresponding entry. Then delete the old `vestigialCursorRulePriorHash` variable declaration entirely. Also delete the stub `priorsFor` added in Task 8 — this file's `priors.go` now provides the real one.

Update any code that read the old map directly to call `priorsFor(path)` instead.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/config/managedfiles/... -v`
Expected: All priors tests PASS; all PR D cursor-rule drift tests still PASS (they read through `priorsFor` now, transparently).

- [ ] **Step 6: Commit**

```bash
jj describe -m "refactor(managedfiles): unify priors lookups behind priorsRegistry

PR C's computePriorCanonical results and PR D's
vestigialCursorRulePriorHash map both addressed the same need:
'if the on-disk file matches THIS known older canonical hash,
classify as Stale-managed rather than Drifted-userowned'. PR E adds
a third user (JSON files with no in-file sentinel).

Unify into a single sync.RWMutex-guarded registry with priorsFor()
accessor. PR D's map migrates via package-level init. PR E's
upcoming JSON entries register through the same path.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 10: Export drift-detail consts

**Files:**

- Modify: `internal/config/managedfiles/supersedes.go` (or wherever the magic strings live today)
- Modify producers: `wholefile.go`, `markdownblock.go`

- [ ] **Step 1: Add the exported consts**

In `supersedes.go` (or create `internal/config/managedfiles/drift_details.go` if cleaner):

```go
// Detail strings emitted in SyncResult.Detail and FileState.Detail.
// PR G's `specgraph doctor` regex-matches these prefixes to render
// user-facing classifications; keep them stable and exported.
const (
	// DriftDetailNoSentinel is emitted when WholeFile + CommentNone
	// finds an on-disk file whose hash matches neither the canonical
	// nor any registered prior — i.e. user-owned content at a path the
	// framework manages.
	DriftDetailNoSentinel = "no sentinel"

	// DriftDetailFrontmatterBrokenPrefix prefixes detail strings emitted
	// when WholeFile + HasFrontmatter finds malformed frontmatter
	// surrounding the sentinel line.
	DriftDetailFrontmatterBrokenPrefix = "frontmatter broken: "

	// DriftDetailSupersedesPath prefixes detail strings emitted when a
	// superseded path is left in place because its content has drifted
	// from the prior canonical.
	DriftDetailSupersedesPath = "supersedes path "
)
```

- [ ] **Step 2: Replace string literals in producers**

In each producer, replace bare string literals with the exported consts:

- `wholefile.go`: `"no sentinel"` → `DriftDetailNoSentinel`
- `wholefile.go`: `"frontmatter broken: " + ...` → `DriftDetailFrontmatterBrokenPrefix + ...`
- `supersedes.go` / `vestigial_cursor_rules.go`: `"supersedes path "` → `DriftDetailSupersedesPath`

- [ ] **Step 3: Run tests**

Run: `task check`
Expected: All tests pass (producers and assertions both use the same const, so any string-equality tests still work).

- [ ] **Step 4: Commit**

```bash
jj describe -m "refactor(managedfiles): export drift-detail const prefixes for PR G

DriftDetailNoSentinel, DriftDetailFrontmatterBrokenPrefix, and
DriftDetailSupersedesPath are now exported consts. PR G's
'specgraph doctor' will regex-match these to render user-facing
classifications; making them exported decouples the doctor regex
from a hand-maintained mirror.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 11: Relocate canonicals to `embedded/claude/`; create reverse-symlinks

**Files:**

- Move: `plugin/specgraph/hooks/session-start.sh` → `internal/config/managedfiles/embedded/claude/hooks/specgraph-session-start.sh`
- Move: `plugin/specgraph/hooks/post-stage.sh` → `internal/config/managedfiles/embedded/claude/hooks/specgraph-post-stage.sh`
- Move: `plugin/specgraph/routing-guide.md` → `internal/config/managedfiles/embedded/claude/routing-guide.md`
- Delete: `plugin/specgraph/hooks/hooks.json` (content inlined into `plugin.json`)
- Move: `plugin/specgraph/.claude-plugin/plugin.json` → `internal/config/managedfiles/embedded/claude/.claude-plugin/plugin.json` (with content updates per below)
- Create: `internal/config/managedfiles/embedded/claude/.claude-plugin/marketplace.json`
- Create reverse-symlinks under `plugin/specgraph/`

- [ ] **Step 1: Move and rename hook scripts via `jj` (preserves history)**

Run:

```bash
mkdir -p internal/config/managedfiles/embedded/claude/hooks
mkdir -p internal/config/managedfiles/embedded/claude/.claude-plugin
jj file move plugin/specgraph/hooks/session-start.sh \
  internal/config/managedfiles/embedded/claude/hooks/specgraph-session-start.sh
jj file move plugin/specgraph/hooks/post-stage.sh \
  internal/config/managedfiles/embedded/claude/hooks/specgraph-post-stage.sh
jj file move plugin/specgraph/routing-guide.md \
  internal/config/managedfiles/embedded/claude/routing-guide.md
jj file move plugin/specgraph/.claude-plugin/plugin.json \
  internal/config/managedfiles/embedded/claude/.claude-plugin/plugin.json
```

If `jj file move` isn't available in your jj version, fall back to `mv` + `jj add` (jj auto-detects the move from content similarity).

- [ ] **Step 2: Rewrite `plugin.json` with inlined hooks**

Open `internal/config/managedfiles/embedded/claude/.claude-plugin/plugin.json` and replace its content with:

```json
{
  "name": "specgraph",
  "description": "Thin Claude Code client for SpecGraph. Rich authoring workflow guidance is delivered from the SpecGraph MCP server.",
  "version": "0.4.0",
  "hooks": [
    {
      "event": "SessionStart",
      "type": "command",
      "command": "${CLAUDE_PLUGIN_ROOT}/hooks/specgraph-session-start.sh"
    },
    {
      "event": "PostToolUse",
      "matcher": "mcp__specgraph__author",
      "type": "command",
      "command": "${CLAUDE_PLUGIN_ROOT}/hooks/specgraph-post-stage.sh"
    }
  ]
}
```

- [ ] **Step 3: Create `marketplace.json`**

Create `internal/config/managedfiles/embedded/claude/.claude-plugin/marketplace.json`:

```json
{
  "name": "specgraph-local",
  "owner": { "name": "SpecGraph" },
  "plugins": [
    {
      "name": "specgraph",
      "source": "./",
      "description": "Thin Claude Code client for SpecGraph. Rich authoring workflow guidance is delivered from the SpecGraph MCP server.",
      "version": "0.4.0"
    }
  ]
}
```

- [ ] **Step 4: Delete `hooks.json`**

```bash
jj file untrack plugin/specgraph/hooks/hooks.json
rm plugin/specgraph/hooks/hooks.json
```

- [ ] **Step 5: Verify hook script exec permissions survived the move**

```bash
ls -l internal/config/managedfiles/embedded/claude/hooks/
```

Both `.sh` files should show `-rwxr-xr-x`. If not, restore:

```bash
chmod +x internal/config/managedfiles/embedded/claude/hooks/specgraph-{session-start,post-stage}.sh
```

- [ ] **Step 6: Create reverse-symlinks under `plugin/specgraph/`**

```bash
cd plugin/specgraph
rmdir hooks 2>/dev/null || rm -rf hooks
rmdir .claude-plugin 2>/dev/null || rm -rf .claude-plugin
ln -s ../../internal/config/managedfiles/embedded/claude/hooks hooks
ln -s ../../internal/config/managedfiles/embedded/claude/.claude-plugin .claude-plugin
ln -s ../../internal/config/managedfiles/embedded/claude/routing-guide.md routing-guide.md
cd ../..
```

Verify with `ls -la plugin/specgraph/` — three symlinks plus `README.md` and `skills/`.

- [ ] **Step 7: Run task check**

Run: `task check`
Expected: PASS (embed.FS picks up the relocated files; the symlinks are documentation-only for authors).

- [ ] **Step 8: Commit**

```bash
jj describe -m "refactor: relocate Claude plugin canonicals under embedded/claude/

Mirrors the PR C OpenCode and PR D Cursor patterns. plugin/specgraph/
hosts reverse-symlinks pointing at embedded/claude/ so authors editing
under plugin/ still feel like they're editing the authoring source
while the binary embeds the canonical bytes.

Hook scripts gain a 'specgraph-' prefix (session-start.sh →
specgraph-session-start.sh, post-stage.sh → specgraph-post-stage.sh)
to avoid collisions with user-added hooks.

hooks/hooks.json is removed — Claude Code accepts the hooks array
inlined into plugin.json, so we collapse to one fewer file.

plugin.json gains the inlined hooks array and is bumped to 0.4.0;
marketplace.json is new (single-plugin local marketplace with
plugins[].source = './').

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 12: Add the 5 new `WholeFile` manifest entries

**Files:**

- Modify: `internal/config/managedfiles/manifest.go`
- Test: integration in existing manifest-iteration tests

- [ ] **Step 1: Write the failing test**

Append to `manifest_test.go`:

```go
func TestManifest_ClaudePluginShimEntries(t *testing.T) {
	wantPaths := []string{
		".specgraph/agents/claude/.claude-plugin/plugin.json",
		".specgraph/agents/claude/.claude-plugin/marketplace.json",
		".specgraph/agents/claude/hooks/specgraph-session-start.sh",
		".specgraph/agents/claude/hooks/specgraph-post-stage.sh",
		".specgraph/agents/claude/routing-guide.md",
	}
	present := map[string]bool{}
	for _, mf := range Manifest() {
		present[mf.Path] = true
	}
	for _, p := range wantPaths {
		if !present[p] {
			t.Errorf("manifest missing entry %q", p)
		}
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/config/managedfiles/ -run TestManifest_ClaudePluginShimEntries -v`
Expected: FAIL — entries not yet in manifest.

- [ ] **Step 3: Add the entries**

In `manifest.go`, append to the manifest slice (before the closing `}`):

```go
		{
			Path:     ".specgraph/agents/claude/.claude-plugin/plugin.json",
			Strategy: StrategyWholeFile,
			Comment:  CommentNone,
			Harness:  HarnessClaude,
			Source:   "embedded/claude/.claude-plugin/plugin.json",
		},
		{
			Path:     ".specgraph/agents/claude/.claude-plugin/marketplace.json",
			Strategy: StrategyWholeFile,
			Comment:  CommentNone,
			Harness:  HarnessClaude,
			Source:   "embedded/claude/.claude-plugin/marketplace.json",
		},
		{
			Path:     ".specgraph/agents/claude/hooks/specgraph-session-start.sh",
			Strategy: StrategyWholeFile,
			Comment:  CommentHash,
			Harness:  HarnessClaude,
			Source:   "embedded/claude/hooks/specgraph-session-start.sh",
		},
		{
			Path:     ".specgraph/agents/claude/hooks/specgraph-post-stage.sh",
			Strategy: StrategyWholeFile,
			Comment:  CommentHash,
			Harness:  HarnessClaude,
			Source:   "embedded/claude/hooks/specgraph-post-stage.sh",
		},
		{
			Path:           ".specgraph/agents/claude/routing-guide.md",
			Strategy:       StrategyWholeFile,
			Comment:        CommentHTML,
			HasFrontmatter: false,
			Harness:        HarnessClaude,
			Source:         "embedded/claude/routing-guide.md",
		},
```

- [ ] **Step 4: Run tests**

Run: `task check`
Expected: PASS for the new entries; existing tests still PASS.

- [ ] **Step 5: Commit**

```bash
jj describe -m "feat(managedfiles): add 5 WholeFile entries for Claude plugin shim

Manifest grows by 5 entries under .specgraph/agents/claude/:
plugin.json + marketplace.json (CommentNone, JSON), two hook scripts
(CommentHash), and routing-guide.md (CommentHTML, !HasFrontmatter —
the newly-supported plain-Markdown combo from Task 8).

All five Source paths resolve through the existing embedded/ FS
loader, pointing at the canonicals relocated in Task 11.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 13: Add the `.claude/settings.json` JSONKeyMerge entry

**Files:**

- Modify: `internal/config/managedfiles/manifest.go`
- Test: `internal/config/managedfiles/jsonkeymerge_test.go`

- [ ] **Step 1: Write the failing integration test**

Append to `jsonkeymerge_test.go`:

```go
func TestClaudeSettingsJSON_FreshInit(t *testing.T) {
	dir := t.TempDir()
	mf := findManifestEntry(t, ".claude/settings.json")
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{Slug: "x"}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, ".claude/settings.json"))
	if !strings.Contains(string(got), `"specgraph-local"`) {
		t.Errorf("marketplace entry not written: %s", got)
	}
	if !strings.Contains(string(got), `"./.specgraph/agents/claude"`) {
		t.Errorf("marketplace path not set correctly: %s", got)
	}
	if !strings.Contains(string(got), `"specgraph@specgraph-local":true`) {
		t.Errorf("enabledPlugin not written on fresh init: %s", got)
	}
}

func TestClaudeSettingsJSON_PreservesUserDisable(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	_ = os.MkdirAll(settingsDir, 0o755)
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"),
		[]byte(`{"enabledPlugins":{"specgraph@specgraph-local":false}}`),
		0o644)
	mf := findManifestEntry(t, ".claude/settings.json")
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{Slug: "x"}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, ".claude/settings.json"))
	if !strings.Contains(string(got), `"specgraph@specgraph-local":false`) {
		t.Errorf("user's disable was overwritten: %s", got)
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/config/managedfiles/ -run TestClaudeSettingsJSON -v`
Expected: FAIL — entry missing.

- [ ] **Step 3: Add the entry to the manifest**

Append to the manifest slice in `manifest.go`:

```go
		{
			Path:     ".claude/settings.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessClaude,
			JSONKeys: []JSONManagedKey{
				{
					Path: "/extraKnownMarketplaces/specgraph-local",
					Mode: KeyManagedValue,
					Value: func(_ ProjectParams) (any, error) {
						return map[string]any{
							"source": map[string]any{
								"type": "directory",
								"path": "./.specgraph/agents/claude",
							},
							"autoUpdate": false,
						}, nil
					},
				},
				{
					Path: "/enabledPlugins/specgraph@specgraph-local",
					Mode: KeyManagedPresence,
					Value: func(_ ProjectParams) (any, error) { return true, nil },
				},
			},
		},
```

- [ ] **Step 4: Run tests**

Run: `task check`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
jj describe -m "feat(managedfiles): add .claude/settings.json JSONKeyMerge entry

Two managed keys:
  /extraKnownMarketplaces/specgraph-local — KeyManagedValue, the
    local marketplace registration with source.path pointing at the
    marketplace ROOT (.specgraph/agents/claude), NOT .claude-plugin/
    itself. autoUpdate: false per PR 0 verification.
  /enabledPlugins/specgraph@specgraph-local — KeyManagedPresence,
    written true on first init; a user's /plugin disable specgraph
    survives subsequent inits.

Manifest is now at 14 entries (8 + 6 new), matching the parent
epic's table.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 14: Integration + E2E tests

**Files:**

- Create: `e2e/api/claude_plugin_test.go`
- Possibly extend existing integration tests in `internal/config/managedfiles/integration_test.go`

- [ ] **Step 1: Write the E2E test**

Create `e2e/api/claude_plugin_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Claude plugin shim install", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "spgr-claude-e2e-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	It("writes all 6 Claude-owned managed files on fresh init", func() {
		runInit(tmpDir)
		expected := []string{
			".claude/settings.json",
			".specgraph/agents/claude/.claude-plugin/plugin.json",
			".specgraph/agents/claude/.claude-plugin/marketplace.json",
			".specgraph/agents/claude/hooks/specgraph-session-start.sh",
			".specgraph/agents/claude/hooks/specgraph-post-stage.sh",
			".specgraph/agents/claude/routing-guide.md",
		}
		for _, p := range expected {
			_, err := os.Stat(filepath.Join(tmpDir, p))
			Expect(err).NotTo(HaveOccurred(), "expected %s to exist", p)
		}
	})

	It("preserves /plugin disable across re-init", func() {
		runInit(tmpDir)
		settingsPath := filepath.Join(tmpDir, ".claude/settings.json")
		body, _ := os.ReadFile(settingsPath)
		// Flip the enabled state to false.
		body = []byte(strings.ReplaceAll(string(body),
			`"specgraph@specgraph-local":true`,
			`"specgraph@specgraph-local":false`))
		Expect(os.WriteFile(settingsPath, body, 0o644)).To(Succeed())
		runInit(tmpDir)
		body2, _ := os.ReadFile(settingsPath)
		var doc map[string]any
		_ = json.Unmarshal(body2, &doc)
		enabled := doc["enabledPlugins"].(map[string]any)
		Expect(enabled["specgraph@specgraph-local"]).To(BeFalse(),
			"expected user's disable to survive init")
	})

	It("registers the marketplace at the directory root, not .claude-plugin/", func() {
		runInit(tmpDir)
		body, _ := os.ReadFile(filepath.Join(tmpDir, ".claude/settings.json"))
		var doc map[string]any
		_ = json.Unmarshal(body, &doc)
		ekm := doc["extraKnownMarketplaces"].(map[string]any)
		entry := ekm["specgraph-local"].(map[string]any)
		source := entry["source"].(map[string]any)
		Expect(source["path"]).To(Equal("./.specgraph/agents/claude"))
	})
})

// runInit shells out to the specgraph binary to execute `specgraph init`
// against tmpDir. Pattern matches existing e2e helpers in this package;
// see existing e2e/api tests for the canonical RunInit form.
func runInit(tmpDir string) {
	// Implementation copies existing pattern; do not duplicate here.
}

func TestClaudePluginShim(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Claude plugin shim install")
}
```

- [ ] **Step 2: Run E2E**

Run: `task test:e2e` (requires Docker for the broader e2e suite, but this test is filesystem-only). Expected: PASS.

If `task test:e2e` is too heavy for iteration, run just this file:

```bash
go test -tags e2e ./e2e/api/ -run TestClaudePluginShim -v
```

- [ ] **Step 3: Commit**

```bash
jj describe -m "test(e2e): Claude plugin shim install end-to-end

Three E2E scenarios:
  - all 6 Claude-owned managed files exist after fresh init
  - /plugin disable survives subsequent init (presence-only key)
  - marketplace source.path is the directory ROOT, not .claude-plugin/

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 15: Documentation + final `task check`

**Files:**

- Modify: `CLAUDE.md` (Plugin shims paragraph)
- Modify: `plugin/specgraph/README.md` (reverse-symlink layout)
- Modify: `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md` (mark PR E merged — DO NOT commit this in PR E's commits; this happens after merge)

- [ ] **Step 1: Update `CLAUDE.md`**

Find the "Plugin shims" paragraph and update to reflect Claude joining Cursor/OpenCode. Specifically:

- Drop references to `hooks.json` (deleted).
- Mention `.claude/settings.json` is now init-managed.
- Note hook script renames.

Concrete edit (locate and replace):

````markdown
- **Plugin shims** — `plugin/specgraph/` (Claude Code), `plugin/cursor/` (Cursor), and `plugin/opencode/` (OpenCode) are thin per-harness shims that consume a single shared `skills/` tree at the repo root. After PR E, all three harnesses' canonical content lives under `internal/config/managedfiles/embedded/<harness>/` and is written to end-user projects by `specgraph init` (Cursor → `.cursor/rules/`, OpenCode → `.specgraph/agents/opencode/`, Claude → `.specgraph/agents/claude/` plus `.claude/settings.json`). The `plugin/<harness>/` directories are reverse-symlinks for author convenience. Refresh after editing canonicals: `task plugin:sync`.
````

- [ ] **Step 2: Update `plugin/specgraph/README.md`**

Replace its body with text describing the new reverse-symlink layout. Mention that hook scripts now carry a `specgraph-` prefix.

- [ ] **Step 3: Run full pre-PR pipeline**

```bash
task pr-prep
```

Expected: All checks pass (fmt, license, lint, build, unit tests, integration, e2e).

- [ ] **Step 4: Commit docs**

```bash
jj describe -m "docs: update CLAUDE.md and plugin/specgraph/README.md for PR E

Reflects: Claude joins Cursor/OpenCode under the embed-and-write
pattern; .claude/settings.json is init-managed; hook scripts gain
specgraph- prefix to avoid user collisions; plugin/specgraph/ hosts
reverse-symlinks.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 5: Set bookmark + push**

```bash
gh auth switch --user seanb4t
jj bookmark set spgr-rwrp-pr-e -r @
jj git push --bookmark spgr-rwrp-pr-e
gh pr create --title "spgr-rwrp PR E: Claude plugin shim via embed-and-write" \
  --body-file <(cat <<'EOF'
## Summary

Ships the Claude Code plugin shim through the `internal/config/managedfiles/`
embed-and-write framework. Closes spgr-kir0; parent epic spgr-rwrp.

- 6 new manifest entries (5 WholeFile under `.specgraph/agents/claude/`, plus `.claude/settings.json` JSONKeyMerge)
- 3 migrated entries (`.mcp.json`, `.cursor/mcp.json`, `opencode.json` move onto declarative `JSONKeys`)
- New framework features:
  - `WholeFile + CommentNone` for JSON files (hash-only state classification)
  - `JSONKeys []JSONManagedKey` with `KeyManagedValue`/`KeyManagedPresence`/`KeyManagedArrayUnion` modes
  - Plain-Markdown `WholeFile + CommentHTML + !HasFrontmatter` combo (PR D anchor lifted)
- PR D review fold-ins:
  - Unified `priorsRegistry` (PR C/D/E priors lookups consolidated)
  - Extracted `keepEditsBody*` helpers from `Sync`
  - Exported `DriftDetail*` consts for PR G's doctor regex

Design: `docs/plans/2026-05-12-spgr-rwrp-pr-e-claude-plugin-design.md`
Plan:   `docs/plans/2026-05-12-spgr-rwrp-pr-e-implementation-plan.md`

## Test plan

- [ ] task pr-prep passes locally
- [ ] CI green
- [ ] Manual smoke: fresh `claude` session in a tempdir post-init, `/plugin list` shows specgraph@specgraph-local, `/plugin disable specgraph` followed by re-init preserves disabled state, SessionStart hook fires and primes
EOF
)
```

---

## Self-review checklist

After implementing all 15 tasks, run:

- [ ] `task pr-prep` (full pipeline including e2e)
- [ ] `bd update spgr-kir0 --close`
- [ ] Verify the cross-PR review (Opus subagent against the diff) flags no critical/important issues
- [ ] Verify CodeRabbit comments addressed
- [ ] PR squashed and merged via the standard workflow
