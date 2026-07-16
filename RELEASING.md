# Releasing

SpecGraph releases are driven by [release-please](https://github.com/googleapis/release-please).
There is no manual dispatch and no `task release:cut` — cutting a release is a
single action:

**Merge the open release-please release PR.**

## How it works

1. Every push to `main` runs the `release-please` step of
   `.github/workflows/release.yml`. It parses Conventional Commit messages
   (which, on squash-merge, come from the PR title) since the last release
   and opens or updates a release PR containing the computed version bump and
   an updated `CHANGELOG.md`.
2. Merging that release PR makes release-please cut the `vX.Y.Z` tag and
   publish a GitHub Release whose notes body is release-please's generated
   changelog.
3. The same workflow run then re-runs with `release_created` true: it checks
   out the new tag and runs GoReleaser, which builds, signs, and uploads
   artifacts (binaries, Docker images, Homebrew cask, SBOMs, attestations) to
   the release release-please already created. GoReleaser does not create or
   modify the release's notes body — it only uploads artifacts
   (`replace_existing_artifacts: true`).
4. Ordinary pushes/merges to `main` that do not create a release
   (`release_created` is falsy) run none of the build/publish steps.

## Version bump cadence

`release-please-config.json` sets `bump-minor-pre-major: true` and omits
`bump-patch-for-minor-pre-major`, so pre-1.0 `feat:` commits bump **minor**
(e.g. 0.13.0 → 0.14.0), matching SpecGraph's existing milestone-as-minor
history. Breaking changes also bump minor pre-1.0 (release-please semantics).

## Commit-message linting

- **PR titles** are linted as Conventional Commits in CI
  (`amannn/action-semantic-pull-request`) on every pull request. On
  squash-merge, the PR title becomes the commit message on `main` that
  release-please parses — this is the authoritative gate.
- **Local commits** are validated by `cog verify` via the lefthook
  `commit-msg` hook. `cog` (cocogitto) no longer drives releases — it is
  commit-message validation only.

## Out-of-band repo settings (cannot be done in code)

These two settings live in GitHub repo configuration, not in this repo's
YAML, and must be confirmed once (and re-confirmed if branch protection or
merge settings are ever reset):

1. **The release-please GitHub App must be an allowed bypass actor on the
   `main` branch-protection ruleset.** Without this, release-please's PR
   merge (and the version-bump commit it lands) will silently block against
   required status checks / review rules.
2. **Squash-merge "default commit message" must be set to "Pull request title"**
   (Settings → General → Pull Requests). Otherwise the squashed commit that
   lands on `main` won't be the linted PR title, and release-please will
   parse the wrong (or a compound, GitHub-generated) message.
