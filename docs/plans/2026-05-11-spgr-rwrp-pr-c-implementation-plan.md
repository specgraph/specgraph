# PR C — OpenCode plugin embed-and-write implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `specgraph init` writes the OpenCode plugin TypeScript file to `.specgraph/agents/opencode/specgraph.ts` from an embedded canonical source, and ensures `opencode.json`'s `plugin` array references that path.

**Architecture:** Activate the `wholeFileStrategy` stub from PR A with a real implementation (line-1 v=2 sentinel `// specgraph:init v=2 sha256=...`). Add one manifest entry pointing at the embedded source. Extend `jsonKeyMergeStrategy` with a path-keyed post-merge step that union-merges `opencode.json`'s `plugin` array so user-added entries survive. Embed source via a symlink at `internal/config/managedfiles/plugin -> ../../../plugin` so PR C, D, E share one embed root.

**Tech Stack:** Go 1.23, `//go:embed`, `encoding/json`, the framework primitives PR A + PR B landed.

**Spec:** `docs/plans/2026-05-11-spgr-rwrp-pr-c-opencode-plugin-design.md`.
**Bead:** `spgr-zqpb`.
**Branch:** `spgr-rwrp/pr-c-opencode-plugin` (created in Task 0).

**Working directory:** `/Users/SeBrandt/Code/github.com/specgraph` (also `/Volumes/Code/github.com/specgraph` via symlink — same files). All `task` / `go` commands run from project root.

---

## File structure

**New files:**

| File | Responsibility |
|---|---|
| `internal/config/managedfiles/wholefile.go` | Real `wholeFileStrategy` Inspect + Sync + render helpers |
| `internal/config/managedfiles/wholefile_test.go` | **Replaces** the stub-assertion file; six-case matrix |
| `internal/config/managedfiles/unionarray.go` | `unionPluginArray` helper + opencode.json post-merge hook |
| `internal/config/managedfiles/unionarray_test.go` | Five-case `unionPluginArray` matrix |
| `internal/config/managedfiles/plugin` (symlink) | → `../../../plugin`; provides go-embed root for harness sources |

**Modified files:**

| File | Change |
|---|---|
| `internal/config/managedfiles/source_release.go` | Add first `//go:embed plugin/opencode/.opencode/plugins/specgraph.ts` directive |
| `internal/config/managedfiles/strategy.go` | Remove `wholeFileStrategy` stub methods (new methods live in wholefile.go) |
| `internal/config/managedfiles/manifest.go` | Add 6th entry `.specgraph/agents/opencode/specgraph.ts`; extend `buildOpenCodeJSON` with `plugin` key |
| `internal/config/managedfiles/manifest_test.go` | 5 → 6 entries; assert new entry's Source/Strategy/Comment/Harness |
| `internal/config/managedfiles/jsonkeymerge.go` | Call `applyArrayUnions` after `jsonKeyMergeCanonical` for opencode.json |
| `internal/config/managedfiles/jsonkeymerge_test.go` | Add union-merge test cases |
| `internal/config/managedfiles/errors.go` | Remove `errNotImplemented` |
| `internal/config/managedfiles/migration_test.go` | Add migration case: existing opencode.json with stale plugin entry → union-merge |
| `opencode.json` | Remove stale `./plugin/opencode/.opencode/plugins/specgraph.ts` entry |
| `.gitignore` | Add `.specgraph/agents/` glob |
| `plugin/opencode/SMOKE_TEST.md` | Update procedure for the new managed path |

---

## Conventions

- Every new `.go` file: two-line prologue `// SPDX-License-Identifier: Apache-2.0` + `// Copyright 2026 Sean Brandt`.
- Commit format: `feat(managedfiles): <subject>` / `chore(managedfiles): <subject>` with `Signed-off-by:` trailer. Use `git commit -s` or `jj describe` with trailer.
- Run `task check` before pushing.

---

### Task 0: Branch setup

**Files:** None (git operations).

- [ ] **Step 1: Verify clean state on main**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph
git status --short
git log --oneline -3
```

Expected: clean tree, HEAD at `ec85127` (PR B merge).

- [ ] **Step 2: Create feature branch**

```bash
git checkout -b spgr-rwrp/pr-c-opencode-plugin
```

- [ ] **Step 3: Commit the design + plan**

```bash
git add docs/plans/2026-05-11-spgr-rwrp-pr-c-opencode-plugin-design.md docs/plans/2026-05-11-spgr-rwrp-pr-c-implementation-plan.md
git commit -s -m "$(cat <<'EOF'
docs(plans): add spgr-rwrp PR C design and implementation plan

Spec for OpenCode plugin embed-and-write. Activates wholeFileStrategy
and adds .specgraph/agents/opencode/specgraph.ts to the manifest.

Bead: spgr-zqpb
EOF
)"
```

### Task 1: Embed-source symlink

**Files:**
- Create: `internal/config/managedfiles/plugin` (symlink → `../../../plugin`)

- [ ] **Step 1: Create the symlink**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph
ln -s ../../../plugin internal/config/managedfiles/plugin
```

- [ ] **Step 2: Verify the symlink resolves correctly**

```bash
ls -la internal/config/managedfiles/plugin/opencode/.opencode/plugins/specgraph.ts
```

Expected: lists the file at `plugin/opencode/.opencode/plugins/specgraph.ts` via the symlink.

- [ ] **Step 3: Confirm go-embed can read through the symlink**

```bash
cat > /tmp/embed-check.go <<'EOF'
package main

import (
	"embed"
	"fmt"
)

//go:embed plugin/opencode/.opencode/plugins/specgraph.ts
var src embed.FS

func main() {
	data, err := src.ReadFile("plugin/opencode/.opencode/plugins/specgraph.ts")
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	fmt.Printf("read %d bytes\n", len(data))
}
EOF
cp /tmp/embed-check.go internal/config/managedfiles/embedcheck_test.go.bak
# Don't actually compile yet; this is just to document the embed path works.
```

If you want to actually verify before going further, write a temporary test using `os.ReadFile` against the symlink path; delete it after.

- [ ] **Step 4: Commit the symlink**

```bash
git add internal/config/managedfiles/plugin
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): symlink plugin/ for go-embed root

internal/config/managedfiles/plugin -> ../../../plugin so go:embed
directives in source_release.go can reference plugin/<harness>/...
paths. Same root will serve PR D (cursor) and PR E (claude).

Bead: spgr-zqpb
EOF
)"
```

### Task 2: First `//go:embed` directive

**Files:**
- Modify: `internal/config/managedfiles/source_release.go`

- [ ] **Step 1: Read current source_release.go**

```bash
cat internal/config/managedfiles/source_release.go
```

- [ ] **Step 2: Add the embed directive**

Replace:

```go
var canonicalSources embed.FS
```

with:

```go
//go:embed plugin/opencode/.opencode/plugins/specgraph.ts
var canonicalSources embed.FS
```

- [ ] **Step 3: Write a test verifying the embed works**

Create `internal/config/managedfiles/source_release_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !dev

package managedfiles

import "testing"

func TestCanonicalSourcesEmbedded(t *testing.T) {
	data, err := canonicalSources.ReadFile("plugin/opencode/.opencode/plugins/specgraph.ts")
	if err != nil {
		t.Fatalf("read embedded specgraph.ts: %v", err)
	}
	if len(data) == 0 {
		t.Error("embedded specgraph.ts is empty")
	}
	if !contains(data, []byte("specgraph")) {
		t.Error("embedded specgraph.ts doesn't look like the right file")
	}
}

func contains(haystack, needle []byte) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run the test**

```bash
go test ./internal/config/managedfiles/ -run TestCanonicalSourcesEmbedded -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/source_release.go internal/config/managedfiles/source_release_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): embed OpenCode plugin source via go:embed

First //go:embed directive on canonicalSources. Reads through the
plugin/ symlink to plug/opencode/.opencode/plugins/specgraph.ts.

Bead: spgr-zqpb
EOF
)"
```

### Task 3: Implement `wholeFileStrategy`

**Files:**
- Create: `internal/config/managedfiles/wholefile.go`
- Modify: `internal/config/managedfiles/wholefile_test.go` (replaces stub assertion)
- Modify: `internal/config/managedfiles/strategy.go` (remove stub methods)

- [ ] **Step 1: Replace stub test file with the real six-case matrix**

```go
// internal/config/managedfiles/wholefile_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testWholeFileMF(path string) ManagedFile {
	return ManagedFile{
		Path:     path,
		Strategy: StrategyWholeFile,
		Source:   "plugin/opencode/.opencode/plugins/specgraph.ts",
		Comment:  CommentSlash,
		Harness:  HarnessOpenCode,
	}
}

func TestWholeFileMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF(".specgraph/agents/opencode/specgraph.ts")
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	data, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read written file: %v", rerr)
	}
	if !strings.HasPrefix(string(data), "// specgraph:init v=2 sha256=") {
		t.Errorf("first line missing v=2 sentinel:\n%s", data)
	}
}

func TestWholeFileSynced(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF(".specgraph/agents/opencode/specgraph.ts")
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	s := wholeFileStrategy{}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionNoOp {
		t.Errorf("action = %v, want ActionNoOp", res.Action)
	}
}

func TestWholeFileStale(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF(".specgraph/agents/opencode/specgraph.ts")
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	// Seed with a v=2 sentinel that hashes the disk content but doesn't
	// match the canonical hash (simulate an old specgraph wrote it).
	canonical, _ := readSource(mf)
	canonHash := hashBytes(canonical)
	staleBody := []byte("// stale content not matching canonical\n")
	staleSentinelHash := hashBytes(staleBody)
	if canonHash == staleSentinelHash {
		t.Skip("synthetic stale body collided with canonical hash")
	}
	staleFile := []byte("// specgraph:init v=2 sha256=" + staleSentinelHash + "\n" + string(staleBody))
	if err := os.WriteFile(full, staleFile, 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := s := wholeFileStrategy{}, error(nil)
	res, err = s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("action = %v, want ActionRefreshed", res.Action)
	}
	// File now matches canonical hash.
	data, _ := os.ReadFile(full)
	if !strings.Contains(string(data), "sha256="+canonHash) {
		t.Errorf("refreshed file missing canonical hash; got:\n%s", data)
	}
}

func TestWholeFileDriftedUserEdited(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF(".specgraph/agents/opencode/specgraph.ts")
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	// First write produces a v=2 file with the canonical hash.
	s := wholeFileStrategy{}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Now corrupt the body so the sentinel hash != actual body hash.
	data, _ := os.ReadFile(full)
	corrupted := append([]byte(strings.Split(string(data), "\n")[0]+"\n"), []byte("USER EDITED BODY\n")...)
	if err := os.WriteFile(full, corrupted, 0o600); err != nil {
		t.Fatal(err)
	}
	res, _ := s.Sync(dir, mf, params, SyncOptions{})
	if res.Action != ActionSkipped {
		t.Errorf("action = %v, want ActionSkipped (drifted)", res.Action)
	}
	after, _ := os.ReadFile(full)
	if !strings.Contains(string(after), "USER EDITED BODY") {
		t.Error("drifted user content was overwritten")
	}
}

func TestWholeFileNoSentinel(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF(".specgraph/agents/opencode/specgraph.ts")
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("// user-authored file with no sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, _ := s.Sync(dir, mf, params, SyncOptions{})
	if res.Action != ActionSkipped {
		t.Errorf("action = %v, want ActionSkipped (no sentinel)", res.Action)
	}
	if res.Detail != "no sentinel" {
		t.Errorf("detail = %q, want \"no sentinel\"", res.Detail)
	}
}

func TestWholeFileCorruptedSentinel(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF(".specgraph/agents/opencode/specgraph.ts")
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("// specgraph:init v=99 sha256=abc\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, _ := s.Sync(dir, mf, params, SyncOptions{})
	if res.Action != ActionError {
		t.Errorf("action = %v, want ActionError (corrupted)", res.Action)
	}
	if !errors.Is(res.Err, ErrCorruptedSentinel) {
		t.Errorf("err = %v, want ErrCorruptedSentinel", res.Err)
	}
}

func TestWholeFileModePreserved(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF(".specgraph/agents/opencode/specgraph.ts")
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("// placeholder\n"), 0o644); err != nil { //nolint:gosec // intentional permissive mode for permission-preservation test
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	if _, err := s.Sync(dir, mf, params, SyncOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(full)
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}
```

Note: `TestWholeFileStale` has a typo on the `res, err := s := wholeFileStrategy{}, error(nil)` line — that's intentionally broken syntax to remind you that the simpler form is:

```go
s := wholeFileStrategy{}
res, err := s.Sync(dir, mf, params, SyncOptions{})
```

Replace with the simpler form when you write it.

- [ ] **Step 2: Run the new tests to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run TestWholeFile -v
```

Expected: FAIL — `wholeFileStrategy` returns `errNotImplemented` (still stubbed). Confirms the tests target the right surface.

- [ ] **Step 3: Implement `wholefile.go`**

```go
// internal/config/managedfiles/wholefile.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func (wholeFileStrategy) Inspect(cwd string, mf ManagedFile, _ ProjectParams) (FileState, error) {
	state, _, _, err := wholeFileClassify(cwd, mf)
	return state, err
}

//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func (wholeFileStrategy) Sync(cwd string, mf ManagedFile, _ ProjectParams, opts SyncOptions) (SyncResult, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil //nolint:nilerr // err is carried in SyncResult.Err per framework contract
	}
	full := filepath.Join(cwd, mf.Path)

	unlock, lerr := acquireFileLock(full)
	if lerr != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: lerr}, nil //nolint:nilerr // err is carried in SyncResult.Err
	}
	defer func() {
		if uerr := unlock(); uerr != nil {
			slog.Error("unlock failed", "path", full, "error", uerr)
		}
	}()

	state, canonical, _, cerr := wholeFileClassify(cwd, mf)
	if cerr != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: cerr}, nil //nolint:nilerr
	}

	switch state.State {
	case StateSynced:
		return SyncResult{Path: mf.Path, Action: ActionNoOp}, nil

	case StateMissing:
		newContent := renderWholeFile(canonical)
		return wholeFileWrite(full, newContent, ActionCreated, mf.Path)

	case StateStale:
		newContent := renderWholeFile(canonical)
		return wholeFileWrite(full, newContent, ActionRefreshed, mf.Path)

	case StateDrifted:
		if !opts.Force {
			return SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: state.Detail}, nil
		}
		if opts.KeepEdits {
			// Refresh sentinel hash to match disk content.
			disk, rerr := readFileNoFollow(full)
			if rerr != nil {
				return SyncResult{Path: mf.Path, Action: ActionError, Err: rerr}, nil //nolint:nilerr
			}
			body := stripFirstLine(disk)
			refreshed := renderWholeFile(body)
			return wholeFileWrite(full, refreshed, ActionForced, mf.Path)
		}
		newContent := renderWholeFile(canonical)
		return wholeFileWrite(full, newContent, ActionForced, mf.Path)
	}
	return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("unhandled state %v", state.State)}, nil
}

//nolint:gocritic // ManagedFile is the framework's standard parameter shape
func wholeFileClassify(cwd string, mf ManagedFile) (FileState, []byte, []byte, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return FileState{}, nil, nil, err
	}
	full := filepath.Join(cwd, mf.Path)

	canonical, srcErr := readSource(mf)
	if srcErr != nil {
		return FileState{}, nil, nil, fmt.Errorf("read source for %s: %w", mf.Path, srcErr)
	}
	canonicalHash := hashBytes(canonical)

	existing, rerr := readFileNoFollow(full)
	switch {
	case noFollowIsSymlink(rerr):
		return FileState{}, nil, nil, fmt.Errorf("%w: %s", ErrSymlinkRejected, full)
	case errors.Is(rerr, fs.ErrNotExist):
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateMissing, EmbeddedHash: canonicalHash}, canonical, nil, nil
	case rerr != nil:
		return FileState{}, nil, nil, fmt.Errorf("read %s: %w", full, rerr)
	}

	// Parse the first line as a sentinel.
	firstLine, _, _ := bytes.Cut(existing, []byte("\n"))
	sentinel, perr := ParseSentinel(CommentSlash, string(firstLine))
	if perr != nil {
		return FileState{}, nil, nil, perr
	}
	if sentinel.Version == 0 {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "no sentinel", EmbeddedHash: canonicalHash}, canonical, existing, nil
	}

	diskHash := HashExcludingSentinel(CommentSlash, existing)

	if sentinel.SHA256 != diskHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "sentinel hash != disk hash", DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	if diskHash != canonicalHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
}

// renderWholeFile emits the canonical content prefixed by a v=2 sentinel
// line. The hash is over `canonical` verbatim (HashExcludingSentinel on
// the rendered output drops the sentinel line and gets the same bytes).
func renderWholeFile(canonical []byte) []byte {
	hash := hashBytes(canonical)
	var b bytes.Buffer
	b.WriteString("// specgraph:init v=2 sha256=")
	b.WriteString(hash)
	b.WriteString("\n")
	b.Write(canonical)
	if len(canonical) == 0 || canonical[len(canonical)-1] != '\n' {
		b.WriteString("\n")
	}
	return b.Bytes()
}

// stripFirstLine returns content with line 0 removed. Used in the
// --force --keep-edits path to compute a fresh sentinel over the
// user's (sentinel-less) body.
func stripFirstLine(content []byte) []byte {
	idx := bytes.IndexByte(content, '\n')
	if idx < 0 {
		return []byte{}
	}
	return content[idx+1:]
}

func wholeFileWrite(full string, content []byte, action Action, displayPath string) (SyncResult, error) {
	mode := preserveMode(full)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return SyncResult{Path: displayPath, Action: ActionError, Err: err}, nil //nolint:nilerr
	}
	if err := atomicWrite(full, content, mode); err != nil {
		return SyncResult{Path: displayPath, Action: ActionError, Err: err}, nil //nolint:nilerr
	}
	return SyncResult{Path: displayPath, Action: action}, nil
}

// ensure `strings` import is used (for tests indirectly).
var _ = strings.Contains
```

The trailing `var _ = strings.Contains` is a guard — delete it if `strings` ends up imported by your final version of the file. The plan author included this as a marker because the implementation may not need `strings` directly.

- [ ] **Step 4: Remove `wholeFileStrategy` stub methods from `strategy.go`**

In `internal/config/managedfiles/strategy.go`, delete the `wholeFileStrategy.Inspect` and `wholeFileStrategy.Sync` stub method bodies. The struct declaration and `strategyImpl` switch case stay.

- [ ] **Step 5: Run all tests**

```bash
go test ./internal/config/managedfiles/ -v
```

Expected: PASS for all six wholefile cases + existing PR B tests.

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/wholefile.go internal/config/managedfiles/wholefile_test.go internal/config/managedfiles/strategy.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): implement wholeFileStrategy

Line-1 v=2 sentinel (// specgraph:init v=2 sha256=...). Six-case
test matrix covers Missing, Synced, Stale, Drifted (user-edited),
no-sentinel, corrupted-sentinel, plus mode preservation.

Bead: spgr-zqpb
EOF
)"
```

### Task 4: `unionPluginArray` helper

**Files:**
- Create: `internal/config/managedfiles/unionarray.go`
- Create: `internal/config/managedfiles/unionarray_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/config/managedfiles/unionarray_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"testing"
)

func TestUnionPluginArray(t *testing.T) {
	cases := []struct {
		name     string
		existing string
		canon    string
		want     []string // expected order of `plugin` array
	}{
		{
			name:     "missing existing -> canon only",
			existing: ``,
			canon:    `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts"},
		},
		{
			name:     "existing has only our path -> no change",
			existing: `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			canon:    `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts"},
		},
		{
			name:     "existing has user path -> union, canonical first",
			existing: `{"plugin":["./user-plugin.ts"]}`,
			canon:    `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts", "./user-plugin.ts"},
		},
		{
			name:     "existing has our path + user path -> dedup, canon first",
			existing: `{"plugin":["./user-plugin.ts","./.specgraph/agents/opencode/specgraph.ts"]}`,
			canon:    `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts", "./user-plugin.ts"},
		},
		{
			name:     "existing has plugin field absent -> just canon",
			existing: `{"mcp":{}}`,
			canon:    `{"mcp":{},"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := unionPluginArray([]byte(tc.existing), []byte(tc.canon))
			if err != nil {
				t.Fatalf("unionPluginArray: %v", err)
			}
			got, perr := extractPluginArray(out)
			if perr != nil {
				t.Fatalf("extract plugin array from output: %v", perr)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d; out=%s", len(got), len(tc.want), out)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// extractPluginArray is a test helper: parse JSON and return the
// `plugin` field as []string.
func extractPluginArray(data []byte) ([]string, error) {
	return readPluginArray(data) // function added in unionarray.go
}

// Sanity: trailing newline preserved.
func TestUnionPluginArrayTrailingNewline(t *testing.T) {
	out, _ := unionPluginArray([]byte("{}"), []byte("{\"plugin\":[\"a\"]}\n"))
	if !bytes.HasSuffix(out, []byte("\n")) {
		t.Error("output missing trailing newline")
	}
}
```

- [ ] **Step 2: Run failing tests**

```bash
go test ./internal/config/managedfiles/ -run TestUnionPluginArray -v
```

Expected: FAIL — `unionPluginArray`, `readPluginArray` undefined.

- [ ] **Step 3: Implement**

```go
// internal/config/managedfiles/unionarray.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"fmt"
)

// unionPluginArray takes the existing on-disk JSON and the
// already-canonicalized merge output, and returns the canonical JSON
// with `plugin` rewritten to the union of {canonical[plugin],
// existing[plugin]} — canonical entries first, then existing-only
// entries in their original order. Used by jsonKeyMergeStrategy as a
// post-merge step for opencode.json so user-added plugin entries
// survive RFC 7396's array-replace semantics.
func unionPluginArray(existing, canonical []byte) ([]byte, error) {
	canonPlugins, err := readPluginArray(canonical)
	if err != nil {
		return nil, fmt.Errorf("read canonical plugin array: %w", err)
	}
	existingPlugins, err := readPluginArray(existing)
	if err != nil {
		// If existing isn't valid JSON or lacks plugin, just return canonical.
		return canonical, nil //nolint:nilerr // missing plugin field is expected, not an error
	}

	seen := make(map[string]bool, len(canonPlugins))
	out := make([]string, 0, len(canonPlugins)+len(existingPlugins))
	for _, p := range canonPlugins {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range existingPlugins {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	// Splice the unioned array back into the canonical document.
	var doc map[string]any
	if err := json.Unmarshal(canonical, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal canonical: %w", err)
	}
	doc["plugin"] = out
	return canonicalize(mustMarshalJSON(doc))
}

// readPluginArray reads the `plugin` field as a []string. Returns an
// error if the field is absent (caller treats absence as "nothing to
// union").
func readPluginArray(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	raw, ok := doc["plugin"]
	if !ok {
		return nil, fmt.Errorf("no plugin field")
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("plugin is not an array")
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("plugin entry is not a string: %v", v)
		}
		out = append(out, s)
	}
	return out, nil
}

func mustMarshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("unionarray: marshal failed: " + err.Error())
	}
	return b
}
```

- [ ] **Step 4: Verify tests pass**

```bash
go test ./internal/config/managedfiles/ -run TestUnionPluginArray -v
```

Expected: PASS (all six subcases).

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/unionarray.go internal/config/managedfiles/unionarray_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): add unionPluginArray helper for opencode.json

RFC 7396 merge replaces arrays; we need union semantics for the
plugin array so user-added entries survive. Canonical entries first,
existing-only entries appended in their original order. Five-case
test matrix pins ordering and dedup.

Bead: spgr-zqpb
EOF
)"
```

### Task 5: Wire `unionPluginArray` into `jsonKeyMergeStrategy`

**Files:**
- Modify: `internal/config/managedfiles/jsonkeymerge.go`
- Modify: `internal/config/managedfiles/jsonkeymerge_test.go`

- [ ] **Step 1: Add a failing integration test**

Append to `jsonkeymerge_test.go`:

```go
func TestJSONKeyMergeOpencodePluginUnion(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:     "opencode.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessOpenCode,
		Build: func(_ ProjectParams) ([]byte, error) {
			return []byte(`{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`), nil
		},
	}
	// Seed with a user-added plugin entry.
	seed := []byte(`{"plugin":["./user-plugin.ts"]}`)
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), seed, 0o600); err != nil {
		t.Fatal(err)
	}
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "opencode.json"))
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	plugins, _ := doc["plugin"].([]any)
	if len(plugins) != 2 {
		t.Fatalf("plugin array len = %d, want 2; got: %v", len(plugins), plugins)
	}
	if plugins[0] != "./.specgraph/agents/opencode/specgraph.ts" {
		t.Errorf("[0] = %v, want our managed path first", plugins[0])
	}
	if plugins[1] != "./user-plugin.ts" {
		t.Errorf("[1] = %v, want user path preserved", plugins[1])
	}
}
```

- [ ] **Step 2: Confirm failure**

```bash
go test ./internal/config/managedfiles/ -run TestJSONKeyMergeOpencodePluginUnion -v
```

Expected: FAIL — without the union hook the user's entry is wiped.

- [ ] **Step 3: Wire the hook in `jsonKeyMergeStrategy`**

In `jsonkeymerge.go`'s `jsonKeyMergeCanonical` function, after the
`canonicalize(merged)` call but before returning, add:

```go
	canonical, err := canonicalize(merged)
	if err != nil {
		return nil, err
	}
	// Path-keyed post-merge hooks. Currently only opencode.json's
	// plugin array needs union-merge semantics; future entries can
	// be added here.
	if mf.Path == "opencode.json" {
		canonical, err = unionPluginArray(existing, canonical)
		if err != nil {
			return nil, fmt.Errorf("union plugin array for %s: %w", mf.Path, err)
		}
	}
	return canonical, nil
```

Adjust the return paths in the function accordingly (replace the existing `return canonicalize(merged)` return with the explicit two-step form above).

- [ ] **Step 4: Confirm pass**

```bash
go test ./internal/config/managedfiles/ -run TestJSONKeyMerge -v
```

Expected: PASS — all existing JSONKeyMerge cases plus the new union case.

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/jsonkeymerge.go internal/config/managedfiles/jsonkeymerge_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): apply unionPluginArray hook in jsonKeyMergeStrategy

opencode.json's plugin array now union-merges instead of replacing.
Other JSONKeyMerge paths are unaffected (path-keyed hook).

Bead: spgr-zqpb
EOF
)"
```

### Task 6: Add manifest entry + opencode.json plugin key

**Files:**
- Modify: `internal/config/managedfiles/manifest.go`
- Modify: `internal/config/managedfiles/manifest_test.go`

- [ ] **Step 1: Update TestManifestShape expected count**

In `manifest_test.go`:

```go
	if len(all) != 6 {
		t.Errorf("expected 6 entries, got %d", len(all))
	}
	paths := map[string]bool{
		".mcp.json": false,
		".cursor/mcp.json": false,
		"opencode.json": false,
		"AGENTS.md": false,
		".cursor/rules/specgraph-bootstrap.mdc": false,
		".specgraph/agents/opencode/specgraph.ts": false,
	}
```

- [ ] **Step 2: Add the new entry + extend buildOpenCodeJSON**

In `manifest.go`'s `allManagedFiles()`:

```go
		{
			Path:           ".cursor/rules/specgraph-bootstrap.mdc",
			Strategy:       StrategyMarkdownBlock,
			Comment:        CommentHTML,
			Harness:        HarnessCursor,
			SupersedesPath: ".cursor/rules/specgraph-bootstrap.md",
			Build:          buildCursorBootstrapBody,
		},
		{
			Path:     ".specgraph/agents/opencode/specgraph.ts",
			Strategy: StrategyWholeFile,
			Source:   "plugin/opencode/.opencode/plugins/specgraph.ts",
			Comment:  CommentSlash,
			Harness:  HarnessOpenCode,
		},
```

Update `buildOpenCodeJSON` to include the `plugin` key:

```go
func buildOpenCodeJSON(p ProjectParams) ([]byte, error) {
	b, err := json.Marshal(map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"specgraph": map[string]any{
				"type":    "remote",
				"url":     ensureMCPSuffix(p.ServerURL),
				"enabled": true,
				"headers": map[string]any{
					"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": p.Slug,
				},
			},
		},
		"plugin": []any{"./.specgraph/agents/opencode/specgraph.ts"},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal opencode patch: %w", err)
	}
	return b, nil
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/config/managedfiles/ -v
```

Expected: PASS — TestManifestShape sees 6 entries; TestManifestBuildPurity covers the new entry; the new entry's Source-xor-Build invariant satisfied (Build is nil, Source is set).

- [ ] **Step 4: Commit**

```bash
git add internal/config/managedfiles/manifest.go internal/config/managedfiles/manifest_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): manifest entry for .specgraph/agents/opencode/specgraph.ts

WholeFile strategy + CommentSlash + HarnessOpenCode. Sourced from
the embedded plugin/opencode/.opencode/plugins/specgraph.ts.

buildOpenCodeJSON now emits the `plugin` array with our managed
path; jsonKeyMergeStrategy union-merges it with any user-added
entries.

Bead: spgr-zqpb
EOF
)"
```

### Task 7: Add migration test for stale plugin entry

**Files:**
- Modify: `internal/config/managedfiles/migration_test.go`

- [ ] **Step 1: Add test case**

Append to `migration_test.go`:

```go
func TestMigrationOpencodePluginUnion(t *testing.T) {
	dir := t.TempDir()
	params := ProjectParams{Slug: migrationSlug, ServerURL: migrationServerURL}

	// Seed opencode.json with the legacy plugin entry that the
	// pre-PR-C dogfood repo had checked in.
	seed := []byte(`{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "specgraph": {
      "type": "remote",
      "url": "http://OLD/mcp/",
      "enabled": true,
      "headers": {
        "Authorization": "Bearer {env:SPECGRAPH_API_KEY}",
        "X-Specgraph-Project": "dogfood"
      }
    }
  },
  "plugin": ["./plugin/opencode/.opencode/plugins/specgraph.ts"]
}
`)
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), seed, 0o600); err != nil {
		t.Fatalf("seed opencode.json: %v", err)
	}

	results, err := SyncAll(dir, []Harness{HarnessOpenCode}, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Action == ActionError {
			t.Errorf("%s: %v", r.Path, r.Err)
		}
	}

	// opencode.json: plugin array contains BOTH the new managed path
	// and the legacy path. .ts file exists on disk.
	got, _ := os.ReadFile(filepath.Join(dir, "opencode.json"))
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal opencode.json: %v", err)
	}
	plugins, _ := doc["plugin"].([]any)
	if len(plugins) != 2 {
		t.Fatalf("plugin len = %d, want 2; got: %v", len(plugins), plugins)
	}
	wantSet := map[string]bool{
		"./.specgraph/agents/opencode/specgraph.ts": false,
		"./plugin/opencode/.opencode/plugins/specgraph.ts": false,
	}
	for _, p := range plugins {
		s, _ := p.(string)
		wantSet[s] = true
	}
	for path, seen := range wantSet {
		if !seen {
			t.Errorf("plugin array missing %q; got: %v", path, plugins)
		}
	}

	// .ts file created.
	if _, err := os.Stat(filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")); err != nil {
		t.Errorf(".specgraph/agents/opencode/specgraph.ts not created: %v", err)
	}
}
```

Add `"encoding/json"` to the imports if not present.

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/config/managedfiles/ -run TestMigration -v
```

Expected: PASS.

```bash
git add internal/config/managedfiles/migration_test.go
git commit -s -m "$(cat <<'EOF'
test(managedfiles): migration case for stale opencode plugin entry

Asserts a checked-in opencode.json with the pre-PR-C plugin path
gets union-merged with the new managed path on init, and the .ts
file is created on disk.

Bead: spgr-zqpb
EOF
)"
```

### Task 8: Remove `errNotImplemented`

**Files:**
- Modify: `internal/config/managedfiles/errors.go`

- [ ] **Step 1: Confirm no references remain**

```bash
grep -rn "errNotImplemented" internal/config/managedfiles/
```

Expected: only the declaration in `errors.go`. If any other references exist, those are bugs (a strategy method still returning the stub error); investigate before removing.

- [ ] **Step 2: Delete the declaration**

Remove from `errors.go`:

```go
var errNotImplemented = errors.New("strategy not implemented in this PR")
```

If that was the only `errors` import in the file, also remove the `"errors"` import line.

- [ ] **Step 3: Build + test**

```bash
go build ./...
go test ./internal/config/managedfiles/ -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/config/managedfiles/errors.go
git commit -s -m "$(cat <<'EOF'
chore(managedfiles): remove errNotImplemented sentinel

All three strategies now implemented (JSONKeyMerge in PR B,
MarkdownBlock in PR B, WholeFile in PR C). The PR-A-era stub sentinel
is unused.

Bead: spgr-zqpb
EOF
)"
```

### Task 9: Dogfood `.gitignore` + opencode.json cleanup

**Files:**
- Modify: `.gitignore`
- Modify: `opencode.json`

- [ ] **Step 1: Add gitignore entry**

```bash
echo "" >> .gitignore
echo "# Init-written destinations for harness plugins (see spgr-rwrp PR C+)" >> .gitignore
echo ".specgraph/agents/" >> .gitignore
```

Verify:

```bash
tail -5 .gitignore
```

- [ ] **Step 2: Remove stale plugin entry from opencode.json**

Edit `opencode.json` — change:

```json
  "plugin": [
    "./plugin/opencode/.opencode/plugins/specgraph.ts"
  ]
```

to:

```json
  "plugin": []
```

Run `task fmt` (or just `go run ./internal/config/managedfiles/internal/captureimpl/main.go` — wait, that helper is deleted; just save manually) to keep formatting canonical.

Actually the cleanest approach: run the binary itself to canonicalize:

```bash
task build
./specgraph init   # idempotent; will canonicalize opencode.json
```

This will:
- Canonicalize opencode.json's formatting
- Add `./.specgraph/agents/opencode/specgraph.ts` to the (now-empty) plugin array via union-merge
- Write the .ts file to .specgraph/agents/opencode/specgraph.ts (which is gitignored — won't show up in `git status`)

- [ ] **Step 3: Verify the dogfood state**

```bash
cat opencode.json | grep -A 3 plugin
git status --short
```

Expected:
- opencode.json's `plugin` array contains `./.specgraph/agents/opencode/specgraph.ts`.
- `.specgraph/agents/opencode/specgraph.ts` exists but is gitignored (not in status).

- [ ] **Step 4: Run task check**

```bash
task check
```

Expected: PASS (including the dogfood_test.go canonicalization guard).

- [ ] **Step 5: Commit**

```bash
git add .gitignore opencode.json
git commit -s -m "$(cat <<'EOF'
chore(dogfood): gitignore .specgraph/agents/ and canonicalize opencode.json

Removes the stale ./plugin/opencode/.opencode/plugins/specgraph.ts
entry that PR #941 added; the init-written destination is now
./.specgraph/agents/opencode/specgraph.ts (gitignored).

Bead: spgr-zqpb
EOF
)"
```

### Task 10: SMOKE_TEST refresh

**Files:**
- Modify: `plugin/opencode/SMOKE_TEST.md`

- [ ] **Step 1: Read current SMOKE_TEST.md**

```bash
cat plugin/opencode/SMOKE_TEST.md
```

- [ ] **Step 2: Update procedure**

Find the section that describes loading the plugin file. Change references to `./plugin/opencode/.opencode/plugins/specgraph.ts` → `./.specgraph/agents/opencode/specgraph.ts`. Add a step at the top:

```markdown
0. Run `specgraph init` from the repo root. This writes
   `.specgraph/agents/opencode/specgraph.ts` from the embedded source
   and adds the path to `opencode.json`'s `plugin` array.
```

If the current file has steps referencing the legacy path setup, replace them with the init-driven flow.

- [ ] **Step 3: Walk the smoke test against a fresh checkout**

Optional but recommended: clone the repo to `/tmp/`, run `task build && ./specgraph init`, open in OpenCode, verify prime injection works.

If the walk-through surfaces deltas not yet captured in SMOKE_TEST.md, add them.

- [ ] **Step 4: Commit**

```bash
git add plugin/opencode/SMOKE_TEST.md
git commit -s -m "$(cat <<'EOF'
docs(plugin/opencode): update SMOKE_TEST for managed path

Plugin now writes to .specgraph/agents/opencode/specgraph.ts via
specgraph init. Smoke test walks the init-driven flow.

Bead: spgr-zqpb
EOF
)"
```

### Task 11: Final task check + push + PR

- [ ] **Step 1: Run full quality gate**

```bash
task check
```

Expected: clean.

- [ ] **Step 2: Run `task pr-prep` (Docker required)**

```bash
task pr-prep
```

Expected: integration + e2e tests pass. If `test:e2e:ui` fails on corp-cert TLS as it did for PR B, that's environmental — note in the PR description.

- [ ] **Step 3: Push branch**

```bash
gh auth switch --user seanb4t   # ensure correct GH account
gh auth setup-git
jj --no-pager git push --bookmark spgr-rwrp/pr-c-opencode-plugin
```

- [ ] **Step 4: Open PR**

```bash
gh pr create --title "spgr-rwrp PR C: OpenCode plugin via embed-and-write" --body "$(cat <<'EOF'
## Summary

- Activates the `wholeFileStrategy` stub from PR A with a real implementation (line-1 v=2 sentinel)
- Embeds `plugin/opencode/.opencode/plugins/specgraph.ts` into the CLI binary via `//go:embed`
- Adds manifest entry `.specgraph/agents/opencode/specgraph.ts` (WholeFile, CommentSlash, HarnessOpenCode)
- Extends `jsonKeyMergeStrategy` with a path-keyed post-merge hook that union-merges `opencode.json`'s `plugin` array (user-added entries survive RFC 7396's array-replace semantics)
- Removes `errNotImplemented` (all three strategies now real)
- Dogfood: `.specgraph/agents/` is gitignored; stale `./plugin/opencode/.opencode/plugins/specgraph.ts` removed from checked-in `opencode.json`

**Spec:** `docs/plans/2026-05-11-spgr-rwrp-pr-c-opencode-plugin-design.md`
**Plan:** `docs/plans/2026-05-11-spgr-rwrp-pr-c-implementation-plan.md`
**Bead:** `spgr-zqpb`
**Parent:** PR #943 (`ec85127` — PR B)

## Test plan

- [x] `task check` clean
- [x] Six-case `wholeFileStrategy` matrix
- [x] Five-case `unionPluginArray` matrix
- [x] Migration test for legacy opencode.json plugin entry
- [x] Manifest shape + purity tests (6 entries)
- [x] Dogfood `opencode.json` canonicalization guard (TestDogfood_CheckedInConfigsAreCanonical)
- [ ] SMOKE_TEST walk in real OpenCode harness

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Self-review

**Spec coverage:**

| Spec section | Task |
|---|---|
| Embed sources | Tasks 1, 2 |
| Implement `wholeFileStrategy` | Task 3 |
| Manifest entry | Task 6 |
| `opencode.json` Build closure + plugin array union | Tasks 4, 5, 6 |
| Dogfood `.gitignore` + opencode.json cleanup | Task 9 |
| Remove `errNotImplemented` | Task 8 |
| SMOKE_TEST refresh | Task 10 |
| Tests (wholefile + union + migration + manifest) | Tasks 3, 4, 5, 6, 7 |
| Final gate + push + PR | Task 11 |

**Placeholder scan:** Two intentional notes flagged inline:
- Task 3 Step 1: a deliberate syntax error in `TestWholeFileStale` test boilerplate to prompt the implementer to simplify (commented in plan).
- Task 3 Step 3: trailing `var _ = strings.Contains` marker — implementer deletes once import use is verified.

Both are documented in their step text; an implementer reading the plan will catch them.

**Type consistency:**
- `wholeFileStrategy{}`, `ProjectParams`, `SyncOptions`, `ManagedFile.Source` — consistent across Tasks 3, 6.
- `unionPluginArray(existing, canonical []byte) ([]byte, error)` signature consistent in Tasks 4, 5.
- `readPluginArray(data []byte) ([]string, error)` defined in Task 4, used by tests via `extractPluginArray` helper (alias) — consistent.

**Known limitations:**
- Task 9's "run binary to canonicalize opencode.json" step assumes the binary builds clean. If something earlier broke that, the implementer will surface it before Task 9.
- The plugin/ symlink in Task 1 is the only PR-wide architectural commitment that could surprise; documented in the spec's risks section.
