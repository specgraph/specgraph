---
phase: quick-260716-neg
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - release-please-config.json
  - .release-please-manifest.json
  - .github/workflows/release.yml
  - .goreleaser.yaml
  - .github/workflows/ci.yml
  - cog.toml
  - CHANGELOG.md
  - Taskfile.yml
  - RELEASING.md
autonomous: true
requirements: [QUICK-260716-neg]

must_haves:
  truths:
    - "Merging a release-please release PR into main cuts the vN.N.N tag, publishes a GitHub Release whose notes body is release-please's changelog, and GoReleaser uploads artifacts to that same release."
    - "Ordinary pushes/merges to main that do NOT create a release run zero build/publish steps (no wasted GoReleaser/Trivy/attestation runs)."
    - "PR titles are linted as Conventional Commits in CI on every pull_request."
    - "cog remains only a local commit-message validator; no cog install/bump/tag step runs in the release workflow."
    - "CHANGELOG.md is un-frozen: v0.14.0+ is bot-maintained by release-please above the intact historical block."
  artifacts:
    - release-please-config.json
    - .release-please-manifest.json
    - .github/workflows/release.yml
    - .goreleaser.yaml
    - .github/workflows/ci.yml
    - cog.toml
    - CHANGELOG.md
    - Taskfile.yml
    - RELEASING.md
  key_links:
    - "release-please-action reads config-file + manifest-file; manifest seeded to 0.13.0 anchors the next bump off the existing v0.13.0 tag."
    - "GitHub App token (contents + pull-requests + packages write) authors the release-please PR/tag/Release AND is GoReleaser's GITHUB_TOKEN, so GoReleaser can upload to the pre-created release."
    - "Every rich build/publish step is gated on steps.release.outputs.release_created; the gated checkout uses ref: steps.release.outputs.tag_name with fetch-depth: 0."
    - "Trivy image-ref consumes steps.release.outputs.version (replaces the deleted steps.version step)."
    - ".goreleaser.yaml release: block collapses to replace_existing_artifacts: true so GoReleaser's default keep-existing mode preserves release-please's notes body (no double changelog)."
---

<objective>
Switch SpecGraph's release pipeline from cog + GoReleaser (single always-running job) to release-please + GoReleaser (split ownership), mirroring seanb4t/engram. release-please owns version bump, CHANGELOG, tag, and GitHub Release creation; GoReleaser runs only when a release is created and uploads artifacts to the pre-existing release.

Purpose: Give exactly one owner of the Release notes body (release-please) and one owner of the artifacts (GoReleaser via replace_existing_artifacts), eliminating the draft/publish race that produced empty release notes, and gain automated release PRs plus an in-repo bot-maintained CHANGELOG.

Output: New release-please config/manifest, a rewritten gated release workflow, a reduced GoReleaser release block, a PR-title lint job, cog demoted to commit-message validation only, an un-frozen CHANGELOG, a cleaned Taskfile, and a RELEASING.md documenting the new flow plus the out-of-band repo settings.

All decisions in 260716-neg-CONTEXT.md are LOCKED. The 260716-neg-RESEARCH.md file-by-file change list and verified pins are authoritative.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/quick/260716-neg-switch-releases-to-release-please-revers/260716-neg-CONTEXT.md
@.planning/quick/260716-neg-switch-releases-to-release-please-revers/260716-neg-RESEARCH.md
@.github/workflows/release.yml
@.goreleaser.yaml
@.github/workflows/ci.yml
@cog.toml
@CHANGELOG.md
@Taskfile.yml

Verified pins (from RESEARCH — use exactly):
- googleapis/release-please-action v5.0.0 = 45996ed1f6d02564a971a2fa1b5860e934307cf7
- amannn/action-semantic-pull-request v6.1.1 = 48f256284bd46cdaab1048c3721360e808335d50
- actions/create-github-app-token v3.2.0 = bcd2ba49218906704ab6c1aa796996da409d3eb1
- Existing SHAs to preserve for other actions are already in release.yml/ci.yml.

Conventions: SHA-pin every GitHub Action (with a trailing `# vX` comment). DCO sign-off on every commit (`git commit -s`). Conventional-commit messages. .json files cannot carry SPDX comment headers — do NOT add license headers to the two new .json files; addlicense/license:check only processes comment-capable extensions and skips .json.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create release-please config and manifest</name>
  <files>release-please-config.json, .release-please-manifest.json</files>
  <action>
    Create release-please-config.json for a single Go root package. Top-level keys: bump-minor-pre-major set true, always-update set true. Under a packages object, key "." maps to an object with release-type set to "go". OMIT bump-patch-for-minor-pre-major entirely — this preserves SpecGraph's minor-per-feature cadence (feat: -> minor, e.g. 0.13.0 -> 0.14.0) per D-cadence in CONTEXT. Do NOT add any extra-files entry (deferred; only plugin/opencode/package.json carries a version and version stamping is out of scope for this migration). Optionally include a "$schema" pointing at the release-please config schema for editor support.

    Create .release-please-manifest.json containing a single mapping of "." to "0.13.0" (the latest tag). No bootstrap-sha — the existing v0.13.0 tag anchors release-please.

    Neither JSON file gets an SPDX/license header (JSON has no comment syntax).
  </action>
  <verify>
    <automated>jq -e '.packages["."]["release-type"]=="go" and .["bump-minor-pre-major"]==true and .["always-update"]==true and ((.["bump-patch-for-minor-pre-major"]//false)==false) and ((.["extra-files"]//null)==null)' release-please-config.json && jq -e '.["."]=="0.13.0"' .release-please-manifest.json</automated>
  </verify>
  <done>release-please-config.json parses, declares release-type go for ".", sets bump-minor-pre-major + always-update, and omits both bump-patch-for-minor-pre-major and extra-files; manifest maps "." to 0.13.0.</done>
</task>

<task type="auto">
  <name>Task 2: Rewrite the release workflow as gated release-please + GoReleaser</name>
  <files>.github/workflows/release.yml</files>
  <action>
    Rewrite .github/workflows/release.yml. Preserve the existing two-line SPDX/Copyright header.

    Trigger: change to run only on push to the main branch (a single push trigger, branches list of main). Remove the manual-dispatch trigger and its expected_increment input entirely.

    Top-level permissions: contents read. Concurrency: keep a group but scope it to the ref so a stuck release does not serialize unrelated pushes — use group release-${{ github.ref }} with cancel-in-progress false.

    Single job (keep runner namespace-profile-linux-amd64-4x8). Job permissions: contents write is NOT needed at job level for the App-token path, but KEEP packages write, id-token write, attestations write, and contents read (SpecGraph's GHCR push, SLSA attestations, and Trivy need them). Remove the job-level outputs block.

    Step order:
    1. Mint the App token: actions/create-github-app-token pinned to bcd2ba49218906704ab6c1aa796996da409d3eb1 (v3), id app-token, with app-id from secrets.RELEASE_APP_ID and private-key from secrets.RELEASE_APP_PRIVATE_KEY. Keep permission-contents write and permission-packages write, and ADD permission-pull-requests write (release-please needs PR write).
    2. release-please: googleapis/release-please-action pinned to 45996ed1f6d02564a971a2fa1b5860e934307cf7 (v5.0.0), id release, inputs token set to steps.app-token.outputs.token, config-file release-please-config.json, manifest-file .release-please-manifest.json. There is NO checkout step before this — the action works via the API.
    3. Gated checkout: actions/checkout at the existing pinned v6 SHA, with an if condition of steps.release.outputs.release_created, ref steps.release.outputs.tag_name, fetch-depth 0, persist-credentials false.
    4. Then EVERY remaining build/publish step from the current workflow, preserving their existing pinned SHAs and existing relative order, each carrying the same if condition steps.release.outputs.release_created: cosign-installer, sbom-action download-syft, nscloud-setup-buildx-action, docker/login-action (ghcr, username github.actor, password secrets.GITHUB_TOKEN), setup-node v4 (node 22), pnpm/action-setup v4, the Build web UI step (working-directory web, running pnpm install --frozen-lockfile then pnpm build), setup-go v6 (go-version-file go.mod, cache false), nscloud-cache-action (cache go), goreleaser-action v7 (version "~> v2", args release --clean), the Trivy scan, and the two attest-build-provenance steps.

    GoReleaser env: GITHUB_TOKEN set to steps.app-token.outputs.token (so the Release GoReleaser uploads to is authored by the same App that created it), and KEEP HOMEBREW_TAP_TOKEN from secrets.

    Trivy image-ref: set the tag portion to steps.release.outputs.version (this is already v-stripped and replaces the removed version step).

    Remove entirely: the top-of-job checkout that preceded release-please, all cocogitto steps (install, git-identity configuration, preview/guard, and tag creation), and the version-extraction step ("Get version"). Repoint any consumer of that removed version step to steps.release.outputs.version.
  </action>
  <verify>
    <automated>actionlint .github/workflows/release.yml && grep -q '45996ed1f6d02564a971a2fa1b5860e934307cf7' .github/workflows/release.yml && grep -q 'bcd2ba49218906704ab6c1aa796996da409d3eb1' .github/workflows/release.yml && grep -q 'permission-pull-requests: write' .github/workflows/release.yml && grep -q 'steps.release.outputs.tag_name' .github/workflows/release.yml && grep -q 'steps.release.outputs.version' .github/workflows/release.yml && test "$(grep -c 'steps.release.outputs.release_created' .github/workflows/release.yml)" -ge 13 && test "$(grep -v '^[[:space:]]*#' .github/workflows/release.yml | grep -c 'workflow_dispatch')" -eq 0 && test "$(grep -v '^[[:space:]]*#' .github/workflows/release.yml | grep -c 'steps.version.outputs')" -eq 0</automated>
  </verify>
  <done>release.yml is actionlint-clean; triggers only on push to main; mints the App token (v3 SHA, with pull-requests write) before release-please (v5 SHA); a tag-pinned gated checkout precedes every build step; all 10+ build/publish steps carry the release_created gate; GoReleaser uses the app token; Trivy uses steps.release.outputs.version; no manual dispatch, no cog steps, no dangling steps.version references remain.</done>
</task>

<task type="auto">
  <name>Task 3: Reduce the GoReleaser release block and delete its changelog block</name>
  <files>.goreleaser.yaml</files>
  <action>
    Edit .goreleaser.yaml. Replace the entire existing release: block (and its explanatory comment lines) with a release: block whose only key is replace_existing_artifacts set true. Do NOT set mode (default keep-existing preserves release-please's notes body; mode append would duplicate the changelog). Delete the github owner/name sub-block, the draft key, the pre-release designation key, and the tag-creation commit override key described below — the tag already exists remotely under release-please and the job checks out at it, so re-pointing the tag is both unnecessary and risky.
<!-- planner-discipline-allow: target_commitish -->
    The tag-creation commit override key to delete is target_commitish.

    Delete the entire changelog: block (sort/filters) — release-please owns the changelog now, making GoReleaser's changelog filters dead config.

    Leave all other sections (builds, archives, checksum, homebrew_casks, dockers_v2, source, sboms, signs, docker_signs) unchanged.
  </action>
  <verify>
    <automated>goreleaser check && test "$(grep -c 'replace_existing_artifacts: true' .goreleaser.yaml)" -eq 1 && test "$(grep -v '^[[:space:]]*#' .goreleaser.yaml | grep -c 'target_commitish')" -eq 0 && test "$(grep -v '^[[:space:]]*#' .goreleaser.yaml | grep -c 'sort: asc')" -eq 0</automated>
  </verify>
  <done>goreleaser check passes; the release: block is exactly replace_existing_artifacts: true; the changelog block, target_commitish, prerelease/draft, and github owner/name are gone.</done>
</task>

<task type="auto">
  <name>Task 4: Add a PR-title Conventional-Commit lint job to CI</name>
  <files>.github/workflows/ci.yml</files>
  <action>
    Add a new job named commit-lint to the jobs map in .github/workflows/ci.yml. Guard it with an if condition of github.event_name == 'pull_request'. Runner namespace-profile-linux-amd64-4x8 (the SpecGraph runner, not ubuntu-latest). Job-scoped permissions: pull-requests read. Single step using amannn/action-semantic-pull-request pinned to 48f256284bd46cdaab1048c3721360e808335d50 (v6.1.1), with an env of GITHUB_TOKEN set to secrets.GITHUB_TOKEN. No explicit on: types are needed — the workflow already triggers on pull_request. Do not modify the existing jobs; only append this one.
  </action>
  <verify>
    <automated>actionlint .github/workflows/ci.yml && grep -q '48f256284bd46cdaab1048c3721360e808335d50' .github/workflows/ci.yml && grep -q 'commit-lint:' .github/workflows/ci.yml && grep -q "github.event_name == 'pull_request'" .github/workflows/ci.yml</automated>
  </verify>
  <done>ci.yml is actionlint-clean and contains a commit-lint job (SHA-pinned amannn action, pull_request-only, pull-requests: read) without altering existing jobs.</done>
</task>

<task type="auto">
  <name>Task 5: Demote cog to validation-only (cog.toml comment + Taskfile release:cut removal)</name>
  <files>cog.toml, Taskfile.yml</files>
  <action>
    Edit cog.toml: update the header comment block so it states cog is now used ONLY for commit-message validation (via the lefthook commit-msg hook) and NO LONGER drives releases (release-please owns version derivation, tagging, CHANGELOG, and GitHub Release creation). You MUST remove the stale line that currently begins "cog is the release engine: it derives the next semver..." (it is now false). Keep all functional keys unchanged: tag_prefix, disable_changelog, branch_whitelist, ignore_merge_commits. Keep the SPDX header.

    Edit Taskfile.yml: remove the release-cutting task described below (it invoked the workflow via gh) and its two-line body. Keep release:check and release:snapshot exactly as-is, and keep cocogitto in the tools brew-install list (cog verify still runs locally).
<!-- planner-discipline-allow: release:cut -->
    The release-cutting task to remove is release:cut.
  </action>
  <verify>
    <automated>test "$(grep -v '^[[:space:]]*#' Taskfile.yml | grep -c 'release:cut')" -eq 0 && grep -q 'release:check:' Taskfile.yml && grep -q 'release:snapshot:' Taskfile.yml && grep -q 'cocogitto' Taskfile.yml && test "$(grep -c 'cog is the release engine' cog.toml)" -eq 0 && grep -qi 'validation' cog.toml && grep -q 'tag_prefix = "v"' cog.toml</automated>
  </verify>
  <done>Taskfile.yml no longer defines the release-cut task but retains release:check, release:snapshot, and cocogitto in tools; cog.toml header states validation-only while functional keys are unchanged.</done>
</task>

<task type="auto">
  <name>Task 6: Un-freeze CHANGELOG and add RELEASING.md with the out-of-band checklist</name>
  <files>CHANGELOG.md, RELEASING.md</files>
  <action>
    Edit CHANGELOG.md: replace the top blockquote note (the one referencing the v0.7.0 freeze and the GoReleaser design doc) with a new note that states: v0.7.0 and earlier are documented in the historical block below; v0.8.0 through v0.13.0 were published on GitHub Releases only and release-please will NOT backfill them (so a visible 0.7.0 -> 0.14.0 gap in this file is expected); and v0.14.0 onward is bot-maintained here by release-please. Leave the entire historical changelog block (v0.7.0 and earlier) intact below the note.

    Create RELEASING.md documenting the new flow: releasing = merge the open release-please release PR (no manual dispatch); release-please computes the bump from Conventional-Commit PR titles, opens/updates a release PR, and on merge cuts the tag + publishes the Release; GoReleaser then uploads artifacts to it. Include an explicit OUT-OF-BAND repo-settings checklist that CANNOT be done in code and MUST be confirmed once: (1) the release-please GitHub App must be an allowed bypass actor on the main branch-protection ruleset (else release-PR merges silently block); (2) squash-merge "default commit message" must be set to "Pull request title" (Settings -> General -> Pull Requests) so the linted PR title is what lands on main for release-please to parse. RELEASING.md is markdown — no SPDX header.
  </action>
  <verify>
    <automated>test -f RELEASING.md && grep -q 'release-please' RELEASING.md && grep -qi 'bypass' RELEASING.md && grep -qi 'Pull request title' RELEASING.md && grep -q '0.14.0' CHANGELOG.md && test "$(grep -c 'frozen as of v0.7.0' CHANGELOG.md)" -eq 0 && grep -q '## \[0.7.0\]' CHANGELOG.md</automated>
  </verify>
  <done>CHANGELOG.md top note is un-frozen (mentions 0.14.0 bot-maintenance, drops the old freeze wording) with the historical block intact; RELEASING.md exists and documents the merge-the-release-PR flow plus the two out-of-band repo settings.</done>
</task>

<task type="auto">
  <name>Task 7: Full validation gate</name>
  <files>.github/workflows/release.yml, .goreleaser.yaml, .github/workflows/ci.yml</files>
  <action>
    Run the repository quality gate and GoReleaser config validation to prove the migration is internally consistent. Run task check (fmt:check, license:check, lint including actionlint on both workflows, build, unit tests) and goreleaser check. Fix any lint/format/license findings surfaced (e.g. yamlfmt/dprint formatting of the edited YAML/JSON, or a missing license header on a comment-capable file). Do NOT add SPDX headers to the two .json files. If license:check flags the .json files, that indicates a real config surprise — surface it rather than suppressing.

    Note for the PR description (human follow-up, not code): the two out-of-band repo settings from RELEASING.md (App bypass actor on the main ruleset; squash default commit message = Pull request title) must be confirmed before the first release PR is merged.
  </action>
  <verify>
    <automated>task check && goreleaser check</automated>
  </verify>
  <done>task check passes end-to-end and goreleaser check passes; all edited YAML/JSON is formatted and license-clean.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| main push -> release workflow | A push/merge to protected main triggers a privileged build/publish job with write tokens |
| release workflow -> GHCR / GitHub Release | Token-authenticated publish of artifacts, images, signatures, and attestations |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-neg-01 | Tampering | New third-party actions (release-please v5, semantic-pull-request v6.1.1, create-github-app-token v3) | high | mitigate | SHA-pin every action to the RESEARCH-verified commit; versions resolved via gh api on 2026-07-16 |
| T-neg-02 | Elevation of Privilege | App token gains permission-pull-requests: write | medium | mitigate | Minimal added scope; token is a short-lived installation token minted per-run by create-github-app-token, not a long-lived PAT |
| T-neg-03 | Tampering | GoReleaser re-creating/re-pointing the release tag | medium | mitigate | Remove target_commitish and github owner/name; checkout pinned to steps.release.outputs.tag_name so GoReleaser only uploads to the existing release |
| T-neg-04 | Denial of Service | Build/publish steps running on every main push | low | mitigate | Every rich step gated on steps.release.outputs.release_created; non-release pushes run nothing |
| T-neg-SC | Tampering | npm/pip/cargo installs | low | accept | No package-manager installs added by this change; all new dependencies are SHA-pinned GitHub Actions, not registry packages |
</threat_model>

<verification>
- actionlint clean on both .github/workflows/release.yml and .github/workflows/ci.yml (run via task check's lint stage or directly).
- goreleaser check passes with the reduced release block.
- release.yml: release-please (45996ed…) and app-token (bcd2ba4…) SHAs present; permission-pull-requests: write present; 10+ steps gated on release_created; Trivy uses steps.release.outputs.version; no workflow_dispatch, no cog steps, no steps.version references.
- release-please-config.json + .release-please-manifest.json parse and carry the locked values (release-type go, bump-minor-pre-major, always-update, no bump-patch-for-minor-pre-major, no extra-files, manifest 0.13.0).
- task check passes end-to-end.
</verification>

<success_criteria>
- A merge to main that release-please deems a release cuts the tag, publishes a Release with release-please notes, and GoReleaser uploads artifacts to it; a non-release merge runs no build steps.
- PR titles are Conventional-Commit-linted in CI; cog validates local commit messages only.
- CHANGELOG.md is un-frozen with the historical block intact; RELEASING.md documents the flow and the two out-of-band repo settings.
- task check and goreleaser check both pass.
</success_criteria>

<output>
Create `.planning/quick/260716-neg-switch-releases-to-release-please-revers/260716-neg-SUMMARY.md` when done.
</output>
