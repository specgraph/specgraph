# Repository Organization Move Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update all references from `github.com/specgraph/specgraph` to `github.com/specgraph/specgraph` after the GitHub repo transfer.

**Architecture:** Bulk find-replace across Go imports, proto go_package options, site config, and documentation. Regenerate proto-generated code. No logic changes.

**Tech Stack:** Go, protobuf (buf), jj

**Spec:** `docs/superpowers/specs/2026-03-20-repo-org-move-design.md`

**Prerequisites:** Sean must transfer the repo via GitHub Settings → Transfer to `specgraph` org BEFORE executing this plan. After transfer, update local remote:

```bash
jj git remote set-url origin git@github.com:specgraph/specgraph.git
jj git fetch
```

---

## Chunk 1: Module Path and Source Code

### Task 1: Update go.mod

**Files:**

- Modify: `go.mod`

- [ ] **Step 1: Update module declaration**

```text
- module github.com/specgraph/specgraph
+ module github.com/specgraph/specgraph
```

- [ ] **Step 2: Commit**

```bash
jj commit -m "chore: update go.mod module path to specgraph org"
```

### Task 2: Update All Go Import Paths

**Files:**

- Modify: All `.go` files (~165 files) excluding `gen/` (regenerated in Task 4)

- [ ] **Step 1: Bulk find-replace Go imports**

```bash
find . -name '*.go' -not -path './gen/*' -not -path './.jj/*' -exec \
  sed -i '' 's|github.com/specgraph/specgraph|github.com/specgraph/specgraph|g' {} +
```

- [ ] **Step 2: Verify no remaining old references in Go source**

```bash
grep -r "seanb4t/specgraph" --include="*.go" --exclude-dir=gen --exclude-dir=.jj | head -5
```

Expected: No output

- [ ] **Step 3: Verify build (will fail on gen/ mismatch — expected)**

```bash
go build ./cmd/specgraph/...
```

Expected: May fail because `gen/` still has old paths. That's OK — Task 4 fixes it.

- [ ] **Step 4: Commit**

```bash
jj commit -m "chore: update Go import paths to specgraph org"
```

### Task 3: Update Proto go_package Options

**Files:**

- Modify: All `.proto` files in `proto/specgraph/v1/` (10 files)

- [ ] **Step 1: Bulk find-replace proto go_package**

```bash
find proto -name '*.proto' -exec \
  sed -i '' 's|github.com/specgraph/specgraph|github.com/specgraph/specgraph|g' {} +
```

- [ ] **Step 2: Verify**

```bash
grep -r "seanb4t" proto/
```

Expected: No output

- [ ] **Step 3: Commit**

```bash
jj commit -m "chore: update proto go_package paths to specgraph org"
```

### Task 4: Regenerate Proto Code

**Files:**

- Regenerate: All files in `gen/`

- [ ] **Step 1: Clean and regenerate**

```bash
rm -rf gen/
task proto
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

Expected: PASS — all imports and generated code now use the new path.

- [ ] **Step 3: Commit**

```bash
jj commit -m "chore: regenerate proto code with new module path"
```

---

## Chunk 2: Configuration and Documentation

### Task 5: Update Site Configuration

**Files:**

- Modify: `site/zensical.toml`

- [ ] **Step 1: Update site_url and repo URL**

Change `site_url` from `https://specgraph.io/` to `https://specgraph.io/` (or `https://specgraph.github.io/specgraph/` as interim).

Change repo `url` from `https://github.com/specgraph/specgraph` to `https://github.com/specgraph/specgraph`.

- [ ] **Step 2: Commit**

```bash
jj commit -m "chore: update site config URLs for specgraph org"
```

### Task 6: Update Plugin Manifest

**Files:**

- Modify: `plugin/specgraph/plugin.json`

- [ ] **Step 1: Update any repo references**

```bash
sed -i '' 's|seanb4t/specgraph|specgraph/specgraph|g' plugin/specgraph/plugin.json
```

- [ ] **Step 2: Commit**

```bash
jj commit -m "chore: update plugin.json for specgraph org"
```

### Task 7: Update Docker Compose References

**Files:**

- Modify: `docker/docker-compose.memgraph.yaml` (if it references GHCR)
- Modify: `internal/docker/compose.go` (if it references GHCR)

- [ ] **Step 1: Check and update GHCR image references**

```bash
grep -r "seanb4t" docker/ internal/docker/
```

Update any `ghcr.io/seanb4t/specgraph` → `ghcr.io/specgraph/specgraph`.

- [ ] **Step 2: Commit (if changes made)**

```bash
jj commit -m "chore: update Docker image references for specgraph org"
```

### Task 8: Update README and CLAUDE.md

**Files:**

- Modify: `README.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Bulk replace in markdown docs**

```bash
sed -i '' 's|seanb4t/specgraph|specgraph/specgraph|g' README.md CLAUDE.md
sed -i '' 's|specgraph.io|specgraph.io|g' README.md CLAUDE.md
```

- [ ] **Step 2: Verify**

```bash
grep "seanb4t" README.md CLAUDE.md
```

Expected: No output

- [ ] **Step 3: Commit**

```bash
jj commit -m "docs: update README and CLAUDE.md for specgraph org"
```

### Task 9: Update Historical Docs (plans, specs)

**Files:**

- Modify: All files in `docs/plans/` and `docs/superpowers/` referencing `seanb4t`

- [ ] **Step 1: Bulk replace in docs**

```bash
find docs -name '*.md' -exec \
  sed -i '' 's|github.com/specgraph/specgraph|github.com/specgraph/specgraph|g' {} +
find docs -name '*.md' -exec \
  sed -i '' 's|specgraph.io|specgraph.io|g' {} +
```

- [ ] **Step 2: Commit**

```bash
jj commit -m "docs: update historical plan/spec references for specgraph org"
```

---

## Chunk 3: Verification

### Task 10: Full Verification

- [ ] **Step 1: Verify no remaining seanb4t references in source**

```bash
grep -r "seanb4t" --include="*.go" --include="*.proto" --include="*.mod" \
  --include="*.yaml" --include="*.yml" --include="*.json" --include="*.toml" \
  --exclude-dir=.jj | head -10
```

Expected: No output (docs may still have historical references — that's OK).

- [ ] **Step 2: Run task pr-prep**

```bash
task pr-prep
```

Expected: All checks pass (fmt, lint, build, unit tests, integration, e2e).

- [ ] **Step 3: Fix any remaining issues**

If `pr-prep` fails, investigate and fix. Common issues:

- Missed import path in a test file
- Generated code stale (re-run `task proto`)
- YAML formatting (run `yamlfmt`)

- [ ] **Step 4: Final commit if fixups needed**

```bash
jj commit -m "fix: address remaining org-move issues"
```

### Task 11: Post-Merge Housekeeping

After PR is merged:

- [ ] **Step 1: Verify Go module proxy**

```bash
go list -m github.com/specgraph/specgraph@latest
```

- [ ] **Step 2: Update beads external-ref URLs**

Bulk update any beads pointing to the old repo URL.

- [ ] **Step 3: Update local directory name (optional)**

If desired, rename the local clone directory to match the new org.
