# Repository Organization Move

**Status:** Approved
**Date:** 2026-03-20
**Bead:** spgr-4ha

## Problem

The repository lives at `github.com/specgraph/specgraph` under a personal account. Before publishing a Go module or a 0.1.0 release, the repo should move to the `specgraph` GitHub organization so the module path is stable and professional.

## Decision

Transfer the repo to `github.com/specgraph/specgraph`. The Go module path becomes `github.com/specgraph/specgraph`. The project site domain is `specgraph.io` (Cloudflare Pages deployment is out of scope for this spec).

## Two-Phase Process

### Phase 1: GitHub Transfer (manual)

Sean transfers the repo via GitHub Settings → Danger Zone → Transfer. This:

- Moves the repo to `github.com/specgraph/specgraph`
- Creates a redirect from `github.com/specgraph/specgraph`
- Preserves all git history, PRs, issues, and settings
- Requires org owner permissions

### Phase 2: Code Changes (automated PR)

A single PR updates all references from `github.com/specgraph/specgraph` to `github.com/specgraph/specgraph`.

## What Changes

### go.mod

```text
- module github.com/specgraph/specgraph
+ module github.com/specgraph/specgraph
```

### Go Import Paths (~165 files)

Every `.go` file with `"github.com/specgraph/specgraph/..."` imports. Bulk find-replace:

```text
github.com/specgraph/specgraph → github.com/specgraph/specgraph
```

### Proto go_package Options (10 files)

Each `.proto` file in `proto/specgraph/v1/`:

```text
- option go_package = "github.com/specgraph/specgraph/gen/specgraph/v1;specgraphv1";
+ option go_package = "github.com/specgraph/specgraph/gen/specgraph/v1;specgraphv1";
```

The proto package name `specgraph.v1` does not change.

After updating, regenerate all `gen/` files with `task proto`. Do NOT manually edit files in `gen/` — they are fully regenerated.

### buf.yaml / buf.gen.yaml

No changes needed — these files use relative paths (`path: proto`, `out: gen`) and do not reference the module path.

### CI Workflows

No `seanb4t` references exist in `.github/workflows/*.yml`. No changes needed.

### Site Configuration

Update `site/zensical.toml`:

- `site_url`: change from `https://specgraph.io/` to `https://specgraph.io/` (or `https://specgraph.github.io/specgraph/` as interim until custom domain is configured)
- `url`: change from `https://github.com/specgraph/specgraph` to `https://github.com/specgraph/specgraph`

**Note:** After transfer, GitHub Pages serves from `specgraph.github.io/specgraph` until the `specgraph.io` custom domain is configured on Cloudflare Pages. The site deploy workflow (`site.yml`) may need updating when the hosting moves — that is out of scope for this spec.

### Docker Compose

Update `internal/docker/compose.go` templates if they reference `ghcr.io/seanb4t/specgraph`. Check any Docker Compose YAML files for GHCR image references.

### plugin.json

The Claude Code plugin manifest at `plugin/specgraph/plugin.json` may reference the repo URL. Update if present.

### CLAUDE.md and Documentation

Update any `github.com/specgraph/specgraph` or `specgraph.io` URLs in:

- `CLAUDE.md`
- `README.md` (includes GitHub Pages URL)
- `docs/` files (plans, specs, ADRs)
- `site/` docs

### License Headers

The SPDX headers say `Copyright 2026 Sean Brandt` with no repo path — no change needed.

## What Does NOT Change

- Proto package name (`specgraph.v1`)
- Directory structure
- Git history
- Any code logic or behavior
- Beads database content (external-ref URLs updated separately as housekeeping)

## Post-Transfer Housekeeping

After the transfer and code PR merge:

1. **Update local git remote:**

   ```bash
   jj git remote set-url origin git@github.com:specgraph/specgraph.git
   ```

2. **Verify Go module proxy:**

   ```bash
   go list -m github.com/specgraph/specgraph@latest
   ```

   Note: The old module path (`github.com/specgraph/specgraph`) has no published tags on `proxy.golang.org`, so there is no cached old-path entry to worry about. If a tag had been published, the old path would remain permanently in the proxy (immutable). This is why we move before any release.

3. **Update beads external-ref URLs** (bulk search-replace in beads DB if needed).

4. **Update any local `.claude/` project paths** that reference the old repo location on disk (if the local directory is renamed).

## Testing

The PR must pass `task pr-prep` (fmt, lint, build, unit tests, integration tests, e2e). Since this is purely a rename with no logic changes, test failures indicate a missed reference.

## Alternatives Considered

### github.com/specgraph/core

Rejected — this repo is the entire project, not a "core" subset. `specgraph/specgraph` follows the standard primary-repo pattern.

### Prepare code changes before transfer

Rejected — GitHub redirects make the brief inconsistency harmless, and the coordination complexity isn't worth it.
