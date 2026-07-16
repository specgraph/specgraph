# Quick Task 260716-neg: Switch releases to release-please — Research

**Researched:** 2026-07-16
**Domain:** GitHub release automation (release-please + GoReleaser split)
**Confidence:** HIGH (canonical engram reference read live + current action versions/docs verified)

## Summary

The engram pattern is directly transplantable. Every design question in the focus
resolves cleanly: GoReleaser uploads to release-please's pre-created release using
only `replace_existing_artifacts: true` (its `mode` default `keep-existing` preserves
release-please's notes body); `release-type: go` is manifest-driven with no version
file; the existing GitHub App token covers PR-authoring + bypass + GoReleaser upload;
`amannn/action-semantic-pull-request@v6.1.1` is the authoritative squash-merge gate.
The real work is not the release-please wiring (small) but **preserving SpecGraph's
richer build under the `release_created` gate** and **rewiring version references** off
the deleted `steps.version` step onto `steps.release.outputs.*`.

**Primary recommendation:** Mirror engram's `release.yaml` structure verbatim for the
release-please + token + gate scaffolding, then graft SpecGraph's existing web-build →
GoReleaser → Trivy → attestation steps inside the `if: steps.release.outputs.release_created`
gate, replacing `steps.version.outputs.version` with `steps.release.outputs.version`.

## Verified facts (pins + versions)

| Thing | Verified value | Note |
|-------|---------------|------|
| `googleapis/release-please-action` | v5.0.0 = SHA `45996ed1f6d02564a971a2fa1b5860e934307cf7` | latest (2026-04-22); matches engram [VERIFIED: gh api] |
| `amannn/action-semantic-pull-request` | v6.1.1 = SHA `48f256284bd46cdaab1048c3721360e808335d50` | latest (2025-08-22); matches engram [VERIFIED: gh api] |
| `actions/create-github-app-token` | v3.2.0 = SHA `bcd2ba49218906704ab6c1aa796996da409d3eb1` | latest; SpecGraph currently pins **v2** (`fee1f7d…`) — bump to v3 [VERIFIED: gh api] |
| GoReleaser `release.mode` default | `keep-existing` | preserves release-please's notes body — do NOT set `mode: append` [CITED: goreleaser.com/customization/release] |
| release-please root `.` outputs | unprefixed: `release_created`, `tag_name`, `version` (no `v`), `major/minor/patch`, `sha`, `upload_url`, `html_url`, `body` | root component gets unprefixed outputs [CITED: release-please-action v5 README] |
| `v0.13.0` git tag | exists (latest) | anchors release-please; no `bootstrap-sha` needed [VERIFIED: git tag] |
| git-cliff / cliff.toml residue | none | nothing to retire (Claude's-discretion item is moot) [VERIFIED: rg] |

---

## Focus Q1 — GoReleaser uploading to a pre-existing release

**Confirmed working with `replace_existing_artifacts: true` alone; `mode: append` is wrong.**

- When release-please has already created the tag-keyed, **published** (non-draft)
  GitHub Release, `goreleaser release --clean` finds it by tag and **uploads artifacts
  to it** rather than creating a second release. engram proves this in production with a
  `release:` block that is literally just `replace_existing_artifacts: true`. [VERIFIED: engram .goreleaser.yaml]
- `mode` governs the **release-notes body**, not artifacts. Default is `keep-existing`
  → GoReleaser leaves release-please's changelog body untouched. `mode: append` would
  **append GoReleaser's own notes onto release-please's body** — exactly the double-notes
  problem you're eliminating. Leave `mode` unset. [CITED: goreleaser.com/customization/release]
- `replace_existing_artifacts: true` makes re-runs idempotent: on a 422 "asset already
  exists", GoReleaser deletes the existing asset and re-uploads. Needed because the job
  can be retried. [CITED: goreleaser.com/customization/release]
- **Remove from SpecGraph's current `release:` block:** `github: {owner,name}` (inferred
  from the checkout), `draft: false` (default is already false), `prerelease: auto`
  (release-please owns pre-release designation), and **`target_commitish: "{{ .Commit }}"`**.
  `target_commitish` was only needed because *cog created a local-only tag* and GoReleaser
  had to create the remote tag itself. Under release-please the **tag already exists remotely**
  (release-please pushes it) and the job checks out at that tag, so `target_commitish` is
  not just unnecessary — leaving it risks GoReleaser trying to re-point/re-create the tag.
- Net `release:` block: `replace_existing_artifacts: true` (and nothing else). Also delete
  the `changelog:` block — release-please owns the changelog; GoReleaser's `changelog:`
  filters become dead config.

## Focus Q2 — release-please-action@v5 for `release-type: go`

- **Manifest-driven, no version file.** The Go strategy has **no canonical version file to
  stamp** (version is injected at build via GoReleaser ldflags `-X main.version`), so the
  `.release-please-manifest.json` **is** the source of truth. engram runs exactly this: a
  Go root package with manifest `{".": "0.8.7"}` and no version file. [VERIFIED: engram]
- **Seed `.release-please-manifest.json` to `{".": "0.13.0"}`** — matches latest tag `v0.13.0`. Correct.
- **No `bootstrap-sha` / `last-release-sha` needed.** release-please anchors on the existing
  `v0.13.0` tag (its default `tag_name` format is `v${version}`, matching SpecGraph's history)
  and parses Conventional Commits since it to compute the next release PR. `bootstrap-sha` is
  only required when adopting mid-history with **no** matching tag/manifest anchor — not this case.
- **Config** (`release-please-config.json`, mirror engram, drop chart-specific extra-files):
  `release-type: go`, `bump-minor-pre-major: true`, `bump-patch-for-minor-pre-major: true`,
  `always-update: true` (keeps the release PR fresh on every push). [VERIFIED: engram]
- **Action inputs** (v5): `token`, `config-file: release-please-config.json`,
  `manifest-file: .release-please-manifest.json`. **No checkout step before it** — the action
  works via the API (engram has none). [CITED: release-please-action v5 README + engram]
- ⚠️ **Versioning-cadence change (LOCKED but flag to user):** `bump-patch-for-minor-pre-major`
  means while `<1.0`, `feat:` commits bump **patch**, and breaking changes bump **minor** (not
  major). Under the outgoing cog config, `feat:` bumped minor. So post-migration the release
  cadence slows (features → patch). This is the engram-parity choice; call it out so a 0.14.0
  "minor" expectation isn't surprised by a 0.13.1.

## Focus Q3 — Token / branch-protection

The default `GITHUB_TOKEN` is unusable for the release-please role for **two** independent
reasons, both fixed by the GitHub App token:

1. **Anti-recursion:** a release PR opened (and commits pushed) by `GITHUB_TOKEN` does **not**
   trigger `pull_request` / `push` workflows, so required status checks never run and the PR
   sticks on "Expected" forever — unmergeable. An App installation token re-triggers them.
   (engram hit and documented this exact failure mode in its `ui-drift` self-heal.) [VERIFIED: engram ci.yaml]
2. **Branch protection:** release-please pushes the version-bump commit to the release branch
   and the merge lands on protected `main`; the App must be a **bypass actor** on the `main`
   ruleset.

**Wiring (reuse existing `RELEASE_APP_ID` / `RELEASE_APP_PRIVATE_KEY` — do NOT rename to
engram's `RELEASE_APP`):**

| Token | Used for | Required permissions |
|-------|----------|---------------------|
| **App token** (`steps.app-token.outputs.token`) | release-please PR/commit/tag/Release write **and** passed to GoReleaser as `GITHUB_TOKEN` for the Release upload | `contents: write` + `pull-requests: write` (App installation perms). SpecGraph's current step scopes `permission-contents: write, permission-packages: write` — **add `permission-pull-requests: write`** |
| **Default `GITHUB_TOKEN`** (job `permissions:`) | GHCR docker login/push; SLSA attestations OIDC | `packages: write`, `id-token: write`, `attestations: write`, `contents: read` |

- GoReleaser's env `GITHUB_TOKEN: ${{ steps.app-token.outputs.token }}` — so the Release it
  writes to is authored by the same App that created it (consistent, non-blocked). Docker push
  still authenticates via the separate `docker/login-action` using default `GITHUB_TOKEN`
  (has `packages: write`) — unchanged from today.
- **Action item for the user (out of code):** confirm the release-please App is an allowed
  bypass actor on the `main` branch-protection ruleset. This is repo config, not in-repo YAML.

## Focus Q4 — PR-title lint

- **`amannn/action-semantic-pull-request@v6.1.1`** (SHA `48f256284bd46cdaab1048c3721360e808335d50`)
  is current and matches engram. [VERIFIED: gh api]
- **Minimal job** (add to `.github/workflows/ci.yml`), mirror engram:
  ```yaml
  commit-lint:
    name: commit-lint
    if: github.event_name == 'pull_request'
    runs-on: namespace-profile-linux-amd64-4x8   # SpecGraph runner, not ubuntu-latest
    permissions:
      pull-requests: read
    steps:
      - uses: amannn/action-semantic-pull-request@48f256284bd46cdaab1048c3721360e808335d50 # v6.1.1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  ```
- The action reads PR metadata via `GITHUB_TOKEN`; `pull-requests: read` suffices. It runs on
  `pull_request` types `opened`, `edited`, `synchronize`, `reopened` — no explicit `on:` types
  needed if the workflow already triggers on `pull_request` (SpecGraph's ci.yml does).
- **Squash-merge chain:** with squash merge, the **PR title becomes the squashed commit subject**
  on `main`, which is the message release-please parses. So the PR-title lint is the authoritative
  Conventional-Commit gate; the local `cog verify` commit-msg hook is belt-and-suspenders for
  individual commits (which are discarded on squash). ⚠️ **Repo setting dependency:** the repo's
  squash-merge "default commit message" must be set to **"Pull request title"** (Settings →
  General → Pull Requests) or the squashed subject won't be the linted title. Verify this.

## Focus Q5 — SpecGraph-specific migration hazards

The current `release.yml` is a single always-runs job. Post-migration it becomes one job where
**every build/publish step is gated on `if: ${{ steps.release.outputs.release_created }}`** and
checked out at `steps.release.outputs.tag_name`. Hazards:

1. **`steps.version` step is deleted → dangling references.** The current Trivy step uses
   `image-ref: ghcr.io/specgraph/specgraph:${{ steps.version.outputs.version }}`. There is no
   more `version` step. Replace with **`${{ steps.release.outputs.version }}`** (already
   `v`-stripped, identical to the old `${TAG#v}`). Any other `steps.version.outputs.*` usage
   must move to `steps.release.outputs.*`. Drop the whole "Get version" + tag-validation block.
2. **Two checkouts, order matters.** release-please runs first with **no checkout** on `main`
   HEAD; then a **gated `actions/checkout` with `ref: ${{ steps.release.outputs.tag_name }}`,
   `fetch-depth: 0`, `persist-credentials: false`** must precede web-build/setup-go/GoReleaser.
   The current top-of-job checkout must be **removed from before** release-please and **re-added
   gated at the tag**. Without `fetch-depth: 0` GoReleaser's changelog/version templating can break.
3. **Every existing rich step moves under the gate**, in order: setup-node/pnpm → `pnpm install
   --frozen-lockfile && pnpm build` in `web/` (required for `go:embed`) → setup-go → cosign
   installer → syft → buildx → GHCR docker login → GoReleaser → Trivy → 2× attest-build-provenance.
   All get `if: ${{ steps.release.outputs.release_created }}`. Miss one and it runs on **every**
   push to main (release-please's own bump commits included) → wasted/failing builds.
4. **Attestations + Trivy stay on default `GITHUB_TOKEN`/OIDC** — keep job-level `id-token: write`
   + `attestations: write` + `packages: write`. Only GoReleaser's env `GITHUB_TOKEN` switches to
   the app token. Don't collapse job permissions to engram's minimal `packages: write`.
5. **`concurrency: group: release` cannot stay `cancel-in-progress: false` on a main-push trigger.**
   The release workflow now shares the `on: push: branches:[main]` trigger with normal merges.
   Keep a `concurrency` group but scope it so a stuck release doesn't serialize unrelated pushes;
   simplest is `group: release-${{ github.ref }}`. (Design call for planner.)
6. **CHANGELOG.md reconciliation (two issues):**
   - The top blockquote says "frozen as of v0.7.0 … published on GitHub Releases." Un-freeze:
     reword to note that **v0.8.0–v0.13.0 exist only on GitHub Releases** (release-please will
     **not** backfill them) and that **v0.14.0+** is bot-maintained here. Expect a visible
     **0.7.0 → 0.14.0 gap** in CHANGELOG.md — acceptable, but state it.
   - release-please prepends new sections after the file's top matter. Its first PR's CHANGELOG
     diff **must be reviewed** to ensure the new `## [0.14.0]` lands **above** the frozen note /
     historical block cleanly and doesn't interleave. Keep the historical block intact.
7. **Removals (LOCKED):** delete `workflow_dispatch` + `expected_increment` guard, the cog
   install/`cog bump`/git-identity/preview-guard steps, and `task release:cut`. **Keep**
   `cog.toml` (commit-msg validation only — update its header comment: cog no longer drives
   releases), the lefthook `commit-msg` hook, `task release:check`, `task release:snapshot`, and
   `cocogitto` in `task tools`.
8. **`extra-files` stamping (Claude's discretion — recommend DEFER):** SpecGraph has no
   `charts/`. Only `plugin/opencode/package.json` carries a `version`; verify whether
   `plugin/*/.claude-plugin/plugin.json` has one before adding an `extra-files` entry. Adding
   version stamping increases the first-PR blast radius for little gain — ship the core migration
   first, add `extra-files` in a follow-up if desired.

---

## Concrete file-by-file change list

| File | Action |
|------|--------|
| `release-please-config.json` | **CREATE.** `release-type: go`, `bump-minor-pre-major: true`, `bump-patch-for-minor-pre-major: true`, `always-update: true`, package `.` only. No `extra-files` (defer). |
| `.release-please-manifest.json` | **CREATE.** `{ ".": "0.13.0" }`. |
| `.github/workflows/release.yml` | **REWRITE.** `on: push: branches:[main]`. App-token step → `release-please-action@45996ed…` (v5.0.0, with `token`/`config-file`/`manifest-file`) → all existing web-build/GoReleaser/Trivy/attestation steps gated on `steps.release.outputs.release_created`, checkout `ref: steps.release.outputs.tag_name` + `fetch-depth: 0`. GoReleaser env `GITHUB_TOKEN: ${{ steps.app-token.outputs.token }}` (+ keep `HOMEBREW_TAP_TOKEN`). Trivy image tag → `steps.release.outputs.version`. Bump app-token action v2→v3 (`bcd2ba4…`). Add `permission-pull-requests: write` to app-token step. Remove `workflow_dispatch`, cog steps, "Get version" block. |
| `.goreleaser.yaml` | **EDIT.** Replace entire `release:` block with just `replace_existing_artifacts: true`. Delete the `changelog:` block. |
| `.github/workflows/ci.yml` | **ADD** `commit-lint` job (`amannn/action-semantic-pull-request@48f2562…` v6.1.1, `if: pull_request`, `permissions: pull-requests: read`). Optionally add `if: ${{ !startsWith(github.head_ref, 'release-please--') }}` to heavy jobs so release PRs (docs-only) don't run the full Go/e2e matrix — but skipped-job-satisfies-required-check semantics must be verified against SpecGraph's ruleset. |
| `cog.toml` | **EDIT.** Header comment: cog is validation-only, no longer the release engine. Keep `disable_changelog`, `tag_prefix`, `branch_whitelist`. |
| `CHANGELOG.md` | **EDIT.** Reword frozen note (un-freeze; note 0.8.0–0.13.0 live only on GitHub Releases; 0.14.0+ bot-maintained). Keep historical block. |
| `Taskfile.yml` | **EDIT.** Remove `release:cut` (lines ~413–416). Keep `release:check`, `release:snapshot`. Keep `cocogitto` in the `tools` brew list (line ~358). |
| `RELEASING.md` | **OPTIONAL (discretion).** Short doc: releasing = merge the release-please PR. engram has one. |
| `lefthook.yaml` | **NO CHANGE.** `commit-msg` cog+DCO hook stays as-is. |

## Pitfalls / gotchas

1. **Do not set GoReleaser `mode: append`** — it duplicates release-please's notes body. Default
   `keep-existing` is what you want; omit `mode` entirely.
2. **Do not leave `target_commitish` in `.goreleaser.yaml`** — the tag already exists remotely
   under release-please; keeping it invites tag re-creation conflicts.
3. **Every rich build step needs the `release_created` gate** — an ungated step runs on release-
   please's own bump-commit pushes to main and on ordinary merges, burning CI and possibly failing.
4. **`steps.version` no longer exists** — the Trivy `image-ref` (and any other `steps.version.*`)
   must switch to `steps.release.outputs.version`. Easy to miss; breaks the scan silently or hard.
5. **App token needs `pull-requests: write`** in addition to `contents: write`; the current step
   only grants contents+packages. Without PR write, release-please can't open the release PR.
6. **Branch-protection bypass is out-of-band** — the App must be a bypass actor on the `main`
   ruleset; this is repo settings, not YAML, and will silently block release-PR merges if missed.
7. **Squash "default commit message" must be "Pull request title"** or the linted PR title never
   reaches `main` for release-please to parse.
8. **release-please won't backfill v0.8.0–v0.13.0** — expect a CHANGELOG gap; word the un-freeze
   note accordingly and review the first release PR's CHANGELOG placement by hand.
9. **Bump semantics change (pre-1.0):** `feat:` → patch, breaking → minor under the engram flags.
   Confirm this is the intended cadence before the first release PR merges.

## Sources

- **HIGH:** `seanb4t/engram` @ main — `release-please-config.json`, `.release-please-manifest.json`,
  `.github/workflows/release.yaml`, `.goreleaser.yaml`, `ci.yaml` (read live via `gh api` 2026-07-16).
- **HIGH:** `gh api` version/SHA resolution for release-please-action, action-semantic-pull-request,
  create-github-app-token (2026-07-16).
- **MEDIUM:** goreleaser.com/customization/release (mode default `keep-existing`,
  `replace_existing_artifacts`, `target_commitish` semantics).
- **MEDIUM:** release-please-action v5.0.0 README (root-package unprefixed outputs, inputs).
