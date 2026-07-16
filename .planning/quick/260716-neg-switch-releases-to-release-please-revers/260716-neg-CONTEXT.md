# Quick Task 260716-neg: Switch releases to release-please - Context

**Gathered:** 2026-07-16
**Status:** Ready for planning

<domain>
## Task Boundary

Switch SpecGraph's release pipeline from the current **cog + GoReleaser single-job**
model to a **release-please + GoReleaser** model, mirroring the proven pattern in
`seanb4t/engram`. This reverses the prior decision recorded in engram memory
`n5brshcef6` (PR #981 / spgr-7r6g, design doc
`docs/superpowers/specs/2026-06-05-release-single-job-goreleaser-design.md`).

**Motivation:** engram's split gives exactly one owner of the GitHub Release *body*
(release-please) and one owner of the *artifacts* (GoReleaser via
`replace_existing_artifacts`), eliminating the draft/publish race that caused
empty release notes v0.3.7–v0.7.0. Adopting release-please also brings automated
release PRs and an in-repo, bot-maintained CHANGELOG.

</domain>

<decisions>
## Implementation Decisions

### Reference pattern
- Mirror `seanb4t/engram`'s release-please + GoReleaser wiring (verified 2026-07-16):
  - `release-please-config.json` (`release-type: go`, `bump-minor-pre-major`,
    `bump-patch-for-minor-pre-major`, optional `extra-files` version stamping)
  - `.release-please-manifest.json` seeded to the current version
  - `.github/workflows/release.yaml`: `on: push: branches:[main]` → mint GitHub
    App token → `googleapis/release-please-action@v5` → **gate all build/publish
    steps on `steps.release.outputs.release_created`**, checking out at
    `steps.release.outputs.tag_name`.

### GoReleaser role — KEEP as tag-triggered builder (LOCKED)
- release-please owns version bump, CHANGELOG, tag, and GitHub Release creation.
- GoReleaser runs **only when `release_created == true`**, checked out at the cut
  tag, and **uploads artifacts to the already-created release**.
- `.goreleaser.yaml` `release:` block collapses to **just
  `replace_existing_artifacts: true`** (idempotent re-runs). **Drop**
  `target_commitish`, `draft`, `prerelease`, and `github: {owner,name}`.
  **Do NOT set `mode: append`** — GoReleaser's default `keep-existing` preserves
  release-please's notes body; `append` would DUPLICATE the changelog (research
  correction). Also **delete the `changelog:` block** (release-please owns it).
  GoReleaser must NOT create the tag or the release.
- **Preserve all of SpecGraph's richer build steps** inside the `release_created`
  gate: web UI build (`pnpm` in `web/`), Node/pnpm setup, Trivy image scan,
  cosign signing, SBOM, SLSA build-provenance attestations, Homebrew cask,
  multi-arch Docker (`dockers_v2`). This is NOT a verbatim copy of engram's
  smaller workflow — keep SpecGraph's existing security/build steps.

### Commit linting — KEEP cog locally + ADD PR-title lint (LOCKED, "option 2")
- **Retain** the lefthook `commit-msg` hook (`cog verify --file {1}` + DCO
  sign-off check). It is already present and now fires under plain git (it was
  dormant under jj, which bypasses git hooks). No re-introduction of lefthook
  infra needed — it exists; it just needs to keep working.
- **Retain** `cog.toml` for commit-message validation ONLY. Update its header
  comments to state cog no longer drives releases.
- **Remove cog's RELEASE role**: no `cog bump` in CI, no cog install step in the
  release workflow, remove/retire the `release:cut` Task.
- **Add** `amannn/action-semantic-pull-request@v6` to CI (`.github/workflows/ci.yml`)
  to lint the **PR title** — on squash-merge the PR title becomes the commit
  release-please parses, so this is the authoritative conventional-commit gate.
- Keep `cocogitto` in `task tools` (still needed for `cog verify`).

### CHANGELOG.md — UN-FREEZE, bot-maintained (LOCKED)
- release-please resumes maintaining `CHANGELOG.md` in its standard format from
  the next release forward. **Keep the existing frozen historical block**
  (≤ v0.7.0) intact; release-please appends above it.
- Reconcile the "frozen at v0.7.0" note at the top of `CHANGELOG.md` — the freeze
  is being lifted.

### Version / manifest seed + cadence (LOCKED)
- `.release-please-manifest.json` seeded to `{".": "0.13.0"}` (latest tag `v0.13.0`).
  No `bootstrap-sha` needed — the `v0.13.0` tag anchors release-please.
- `release-please-config.json`: `release-type: go`, `always-update: true`,
  `bump-minor-pre-major: true` (breaking → minor pre-1.0).
- **Cadence: preserve minor-per-feature.** OMIT `bump-patch-for-minor-pre-major`
  so `feat:` → **minor** (0.13.0 → 0.14.0), matching SpecGraph's milestone-as-minor
  history (v0.12.0, v0.14.0). This is a deliberate one-flag divergence from engram
  (which sets that flag → feat:→patch). Do NOT set it.
- No `extra-files` version stamping in the first PR (defer — SpecGraph has no
  `charts/`; add later if a plugin.json version stamp is wanted).
- release-please continues Go-style versioning; binary version still injected via
  GoReleaser ldflags `-X main.version={{.Version}}` at the cut tag.

### Secrets / token
- Reuse SpecGraph's **existing** GitHub App secrets `RELEASE_APP_ID` /
  `RELEASE_APP_PRIVATE_KEY` (do NOT rename to engram's `RELEASE_APP`).
- The App token must be the release-PR / tag / release write actor AND the
  `GITHUB_TOKEN` passed to GoReleaser, so the tag-keyed release created by the
  App is writable by GoReleaser and downstream triggers fire (the default
  `GITHUB_TOKEN` does not trigger downstream workflows / is not the bypass actor).
- **App-token step needs `permission-pull-requests: write`** ADDED to its current
  `permission-contents: write` (+ `permission-packages: write`) so release-please
  can open the release PR (research correction).
- Bump `actions/create-github-app-token` v2 → v3 (SHA `bcd2ba4…`).
- Default `GITHUB_TOKEN` (job `permissions:`) keeps `packages: write` (GHCR),
  `id-token: write` + `attestations: write` (SLSA), `contents: read`. Do NOT
  collapse to engram's minimal perms — SpecGraph's attestations/Trivy need them.

### Out-of-band repo settings (CANNOT be done in code — PR checklist)
- Confirm the release-please App is an allowed **bypass actor** on the `main`
  branch-protection ruleset (else release-PR merges silently block).
- Set squash-merge **"default commit message" = "Pull request title"** (Settings →
  General → Pull Requests) so the linted PR title is what lands on `main` for
  release-please to parse.

### Trigger UX
- Releasing = **merging the release-please release PR** (no manual dispatch).
- Remove the `workflow_dispatch` + `expected_increment` guard and `task release:cut`.
- Keep `task release:check` (goreleaser check) and `task release:snapshot`.

### Delivery — Feature branch + PR (LOCKED)
- Work lands on branch `release-please-migration` (already created off
  `origin/main`); open a PR for review before anything touches `main`.

### Claude's Discretion
- Whether to add `extra-files` stamping for `plugin/*/.claude-plugin/plugin.json`
  version fields (nice-to-have; only if those files carry a `version`).
- Exact retirement of `git-cliff`/`cliff.toml` residue if any remains.
- Whether to add a short `RELEASING.md` (engram has one) documenting the new flow.

</decisions>

<specifics>
## Specific Ideas

- Canonical reference implementation: `seanb4t/engram` @ main (files read
  2026-07-16): `release-please-config.json`, `.release-please-manifest.json`,
  `.github/workflows/release.yaml`, `.goreleaser.yaml` (`release:` block =
  `replace_existing_artifacts: true`), `ci.yaml`
  (`amannn/action-semantic-pull-request@v6`), `CHANGELOG.md`.
- Pin all new GitHub Actions to commit SHAs (SpecGraph convention — see existing
  `release.yml`).

</specifics>

<canonical_refs>
## Canonical References

- Prior decision being reversed: engram memory `n5brshcef6`
  (`repo:github.com/specgraph/specgraph`) and
  `docs/superpowers/specs/2026-06-05-release-single-job-goreleaser-design.md`.
- SpecGraph current release surface: `.github/workflows/release.yml`, `cog.toml`,
  `.goreleaser.yaml`, `CHANGELOG.md` (frozen), `Taskfile.yml` (`release:*`),
  `lefthook.yaml` (`commit-msg` → `cog verify` + DCO).

</canonical_refs>
