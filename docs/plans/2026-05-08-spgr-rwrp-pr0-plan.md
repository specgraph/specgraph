# PR 0 — Claude API verification spike implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Empirically verify three Claude Code plugin-loading claims that the spgr-rwrp design depends on, before PR A locks the framework's design assumptions.

**Architecture:** Build a minimal scratch project with a hand-written Claude plugin and local-directory marketplace. Run `claude` against it, capture observed behaviour, and write a verification report that greens or reds each claim.

**Tech Stack:** Bash, Claude Code CLI (`claude` v2.1.133, installed at `/opt/homebrew/bin/claude`), JSON config files.

**Spec:** `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md` — see PR 0 section.

**Bead:** new `spgr-rwrp-pr0` (file as part of Task 0).

---

## Claims to verify

1. **Relative path acceptance:** `extraKnownMarketplaces` in `.claude/settings.json` accepts a `source: { type: "directory", path: "<relative path>" }` entry, where the path is project-relative (e.g. `./test-plugin/.claude-plugin`), and Claude resolves it correctly so the plugin appears in `/plugin list`.

2. **`autoUpdate: false` honoured:** Setting `autoUpdate: false` on the marketplace entry is accepted by Claude (no validation error) and is honoured semantically — Claude does not auto-update the plugin from the directory source.

3. **`${CLAUDE_PLUGIN_ROOT}` resolution:** When a hook script in the plugin runs (e.g., a session-start hook), `${CLAUDE_PLUGIN_ROOT}` resolves to the plugin's root directory (the dir containing `.claude-plugin/plugin.json`), **not** the marketplace root and **not** the `.claude-plugin/` subdirectory.

If any claim fails, the spec's PR 0 section documents the design fallout (use absolute paths, drop autoUpdate, re-architect layout one level deeper).

---

## File structure

The spike runs entirely in a scratch directory **outside** the SpecGraph repo. No production code changes; only a verification report committed back to the repo.

```text
~/Code/specgraph-pr0-spike/                    # scratch, gitignored, deletable
├── .claude/
│   └── settings.json                          # extraKnownMarketplaces + enabledPlugins
├── test-marketplace/                          # the local directory marketplace
│   └── .claude-plugin/
│       ├── marketplace.json                   # lists test-plugin
│       └── plugin.json                        # plugin manifest (single-plugin marketplace)
└── test-plugin/
    ├── .claude-plugin/
    │   └── plugin.json                        # alternate layout for claim 3 separation test
    └── hooks/
        └── verify-root.sh                     # prints $CLAUDE_PLUGIN_ROOT to a sentinel file
```

Verification artefacts saved back to `<specgraph-repo>/docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md`.

---

## Task 0: Pre-flight + bead creation

**Files:**

- Create: `spgr-rwrp-pr0` bead (via `bd create`)

- [ ] **Step 1: Confirm Claude CLI is available**

Run: `claude --version`
Expected: `2.1.133 (Claude Code)` or newer.

If missing or older, stop. Install/upgrade Claude Code per your OS (`brew upgrade claude-code` on macOS) before continuing.

- [ ] **Step 2: Confirm `claude --print` works**

Run: `claude -p "Reply with the single word PONG and nothing else." --output-format text`
Expected: stdout contains `PONG`, exit code 0.

If this fails (auth, model unavailable), resolve before continuing — the rest of the plan needs `--print` mode.

- [ ] **Step 3: File the bead**

```bash
bd create \
  --type=task \
  --priority=2 \
  --title="PR 0: Claude API verification spike for spgr-rwrp" \
  --description="Verification spike preceding spgr-rwrp PR A. Confirm three claims about Claude's plugin loading: relative paths in extraKnownMarketplaces source.path; autoUpdate: false honoured at the marketplace-entry level; \${CLAUDE_PLUGIN_ROOT} resolves to the plugin root, not the marketplace root or .claude-plugin/ subdir. Output: docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md."
```

Note the returned bead id (e.g., `spgr-XXXX`); use it in subsequent commits.

- [ ] **Step 4: Wire dependency to the epic**

```bash
bd dep add spgr-rwrp <new-bead-id>
bd update <new-bead-id> --claim
```

- [ ] **Step 5: Push beads**

```bash
bd dolt push
```

---

## Task 1: Set up the scratch project

**Files:**

- Create: `~/Code/specgraph-pr0-spike/` directory tree
- Create: `~/Code/specgraph-pr0-spike/.git/` (init'd as git repo so Claude treats it as a project)
- Create: scratch `README.md`

- [ ] **Step 1: Create the scratch directory and init git**

```bash
mkdir -p ~/Code/specgraph-pr0-spike
cd ~/Code/specgraph-pr0-spike
git init -q
echo "# spgr-rwrp PR 0 spike" > README.md
git add README.md
git -c user.email=spike@local -c user.name=spike commit -q -m "init spike"
```

Expected: clean git repo with one commit.

- [ ] **Step 2: Confirm the scratch dir is correctly placed**

Run: `pwd && ls -la`
Expected: pwd is `~/Code/specgraph-pr0-spike` (or `/Users/<you>/Code/specgraph-pr0-spike`). Listing shows `.git/`, `README.md`.

The remaining tasks all run in this scratch dir unless otherwise noted.

---

## Task 2: Build the test plugin and marketplace

**Files:**

- Create: `~/Code/specgraph-pr0-spike/test-marketplace/.claude-plugin/marketplace.json`
- Create: `~/Code/specgraph-pr0-spike/test-marketplace/.claude-plugin/plugin.json`
- Create: `~/Code/specgraph-pr0-spike/test-plugin/.claude-plugin/plugin.json`
- Create: `~/Code/specgraph-pr0-spike/test-plugin/hooks/verify-root.sh`

- [ ] **Step 1: Create the single-plugin marketplace (production-shape, used for claims 1, 2, 3a)**

```bash
mkdir -p test-marketplace/.claude-plugin
```

Write `test-marketplace/.claude-plugin/marketplace.json`:

```json
{
  "name": "spgr-pr0-marketplace",
  "version": "0.1.0",
  "plugins": [
    {
      "name": "spgr-pr0-plugin",
      "source": ".",
      "version": "0.1.0",
      "description": "PR 0 spike plugin"
    }
  ]
}
```

Write `test-marketplace/.claude-plugin/plugin.json`:

```json
{
  "name": "spgr-pr0-plugin",
  "version": "0.1.0",
  "description": "PR 0 spike plugin",
  "hooks": {
    "SessionStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/verify-root.sh"
          }
        ]
      }
    ]
  }
}
```

Note: this layout matches the spgr-rwrp design — marketplace + plugin co-located in the same `.claude-plugin/` dir, single-plugin marketplace.

- [ ] **Step 2: Add the verify-root hook script (in the marketplace/plugin dir)**

```bash
mkdir -p test-marketplace/hooks
cat > test-marketplace/hooks/verify-root.sh <<'EOF'
#!/usr/bin/env bash
# Records what ${CLAUDE_PLUGIN_ROOT} resolves to at hook-execution time.
out_dir="${HOME}/Code/specgraph-pr0-spike/.spike-out"
mkdir -p "$out_dir"
{
  echo "ts: $(date -Iseconds)"
  echo "pwd: $(pwd)"
  echo "CLAUDE_PLUGIN_ROOT: ${CLAUDE_PLUGIN_ROOT:-<unset>}"
  echo "CLAUDE_PROJECT_DIR: ${CLAUDE_PROJECT_DIR:-<unset>}"
  echo "argv0: $0"
} > "$out_dir/marketplace-shape.txt"
EOF
chmod +x test-marketplace/hooks/verify-root.sh
```

- [ ] **Step 3: Create the separated plugin directory (used for claim 3b — plugin-root vs marketplace-root distinction)**

```bash
mkdir -p test-plugin/.claude-plugin test-plugin/hooks
```

Write `test-plugin/.claude-plugin/plugin.json` (note: same shape as above, but now plugin lives in a separate directory from any marketplace):

```json
{
  "name": "spgr-pr0-separated-plugin",
  "version": "0.1.0",
  "description": "PR 0 spike plugin in a separated directory",
  "hooks": {
    "SessionStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/verify-root.sh"
          }
        ]
      }
    ]
  }
}
```

Copy the verify script:

```bash
cat > test-plugin/hooks/verify-root.sh <<'EOF'
#!/usr/bin/env bash
out_dir="${HOME}/Code/specgraph-pr0-spike/.spike-out"
mkdir -p "$out_dir"
{
  echo "ts: $(date -Iseconds)"
  echo "pwd: $(pwd)"
  echo "CLAUDE_PLUGIN_ROOT: ${CLAUDE_PLUGIN_ROOT:-<unset>}"
  echo "CLAUDE_PROJECT_DIR: ${CLAUDE_PROJECT_DIR:-<unset>}"
  echo "argv0: $0"
} > "$out_dir/separated-shape.txt"
EOF
chmod +x test-plugin/hooks/verify-root.sh
```

- [ ] **Step 4: Verify the directory tree**

Run: `find . -type f -not -path './.git/*' | sort`

Expected output exactly:

```text
./README.md
./test-marketplace/.claude-plugin/marketplace.json
./test-marketplace/.claude-plugin/plugin.json
./test-marketplace/hooks/verify-root.sh
./test-plugin/.claude-plugin/plugin.json
./test-plugin/hooks/verify-root.sh
```

---

## Task 3: Verify claim 1 — relative path acceptance

**Files:**

- Create: `~/Code/specgraph-pr0-spike/.claude/settings.json`

- [ ] **Step 1: Write `.claude/settings.json` pointing at the marketplace via relative path**

```bash
mkdir -p .claude
cat > .claude/settings.json <<'EOF'
{
  "extraKnownMarketplaces": {
    "spgr-pr0-marketplace": {
      "source": {
        "source": "directory",
        "path": "./test-marketplace/.claude-plugin"
      },
      "autoUpdate": false
    }
  },
  "enabledPlugins": {
    "spgr-pr0-plugin@spgr-pr0-marketplace": true
  }
}
```

- [ ] **Step 2: Restart any running Claude session, then verify plugin appears**

Use the `--print` mode to ask Claude to enumerate its plugins:

```bash
rm -rf .spike-out
claude -p "/plugin list" --output-format text > .spike-out/plugin-list-claim1.txt 2>&1 || true
cat .spike-out/plugin-list-claim1.txt
```

Expected: the response includes `spgr-pr0-plugin` (or similar). If `/plugin list` doesn't render in `-p` mode, fall back to running `claude` interactively and observing output:

```bash
# Interactive fallback
claude
# In Claude: /plugin list
# Capture screenshot or copy text into .spike-out/plugin-list-claim1.txt
```

- [ ] **Step 3: Classify the result**

Inspect `.spike-out/plugin-list-claim1.txt`:

- **GREEN** if `spgr-pr0-plugin` is listed and Claude reports it as enabled / available.
- **RED** if Claude errors with "could not resolve marketplace path" or similar, OR if the plugin doesn't appear.

Record verdict in a scratch note (`.spike-out/verdict-claim1.txt`):

```bash
echo "claim1: GREEN — relative path resolves" > .spike-out/verdict-claim1.txt
# or RED with the observed error text
```

If RED, the design fallout per the spec is: switch to absolute paths. Document the error verbatim in the verdict file; the report task will record the fallback.

---

## Task 4: Verify claim 2 — `autoUpdate: false` is honoured

**Files:**

- Modify: `~/Code/specgraph-pr0-spike/test-marketplace/.claude-plugin/plugin.json` (bump version to test)

- [ ] **Step 1: Confirm Claude accepts the `autoUpdate: false` setting without error**

Already done in Task 3 — settings.json includes `"autoUpdate": false`. If Claude rejected the field with a validation error in Task 3, claim 2 is RED at the syntactic level. If Task 3 was GREEN, the field is at least accepted.

- [ ] **Step 2: Bump the plugin version on disk**

```bash
# Change version 0.1.0 → 0.1.1
sed -i.bak 's/"version": "0.1.0"/"version": "0.1.1"/' test-marketplace/.claude-plugin/plugin.json
rm test-marketplace/.claude-plugin/plugin.json.bak
grep version test-marketplace/.claude-plugin/plugin.json
```

Expected: `"version": "0.1.1"` is now present.

- [ ] **Step 3: Run Claude again and observe whether it auto-detected the update**

```bash
claude -p "/plugin list" --output-format text > .spike-out/plugin-list-claim2.txt 2>&1 || true
cat .spike-out/plugin-list-claim2.txt
```

Look for:

- Plugin still reports version `0.1.0` → autoUpdate is honoured (Claude did NOT auto-pull `0.1.1`). **GREEN.**
- Plugin reports version `0.1.1` → Claude auto-updated despite `autoUpdate: false`. **RED.**
- No version reported in `/plugin list` output → use the next step to disambiguate.

- [ ] **Step 4 (if needed): Disambiguate via session-start hook output**

If `/plugin list` doesn't include the version, trigger a fresh session and inspect what hook fired (the version embedded in `plugin.json` at the time the session started):

```bash
rm -f .spike-out/marketplace-shape.txt
claude -p "exit" --output-format text > /dev/null 2>&1 || true
cat .spike-out/marketplace-shape.txt 2>/dev/null || echo "hook did not fire"
```

If the hook fired with the still-`0.1.0` plugin path (older bytes still cached/loaded), claim 2 is GREEN. If new bytes loaded, RED.

- [ ] **Step 5: Classify the result**

Record verdict at `.spike-out/verdict-claim2.txt`:

```bash
echo "claim2: GREEN — autoUpdate: false respected (plugin pinned at 0.1.0 despite disk-side bump to 0.1.1)" > .spike-out/verdict-claim2.txt
# or RED if Claude auto-pulled 0.1.1
```

If RED, fallout per spec: drop `autoUpdate: false` from managed keys; rely on Claude's local-marketplace defaults (per docs, off by default for non-Anthropic marketplaces).

---

## Task 5: Verify claim 3 — `${CLAUDE_PLUGIN_ROOT}` resolution

This is the most subtle of the three. Two cases:

- **3a:** marketplace and plugin coincide (single-plugin marketplace, our production layout) — does `CLAUDE_PLUGIN_ROOT` = the dir containing `.claude-plugin/`?
- **3b:** plugin lives in a separate directory from the marketplace — does `CLAUDE_PLUGIN_ROOT` = the plugin's own root, or the marketplace's root?

We don't ship 3b's layout but it's the diagnostic test — if 3a's behaviour ambiguously matches both interpretations, 3b distinguishes them.

**Files:**

- No new files. Reuses the plugins from Task 2.

- [ ] **Step 1: Trigger session-start for the production-shape plugin (3a)**

```bash
rm -f .spike-out/marketplace-shape.txt .spike-out/separated-shape.txt
claude -p "say hi" --output-format text > .spike-out/session-claim3a.txt 2>&1 || true
sleep 1  # let any async hooks finish
cat .spike-out/marketplace-shape.txt 2>/dev/null || echo "MISSING — hook did not fire"
```

Expected: `marketplace-shape.txt` exists and contains a `CLAUDE_PLUGIN_ROOT:` line. Read the value.

- [ ] **Step 2: Determine 3a verdict**

If `CLAUDE_PLUGIN_ROOT` = `/Users/<you>/Code/specgraph-pr0-spike/test-marketplace`:

- Resolves to the dir containing `.claude-plugin/`. **Correct interpretation.** Record observed value.

If `CLAUDE_PLUGIN_ROOT` = `/Users/<you>/Code/specgraph-pr0-spike/test-marketplace/.claude-plugin`:

- Resolves to `.claude-plugin/` itself, one level too deep. **Wrong for our layout.**

If `CLAUDE_PLUGIN_ROOT` = some path outside `test-marketplace/`:

- Unexpected; investigate.

- [ ] **Step 3: Switch settings.json to enable the separated plugin (3b)**

To distinguish "marketplace root" vs "plugin root" we need a layout where they differ. Add a second marketplace entry pointing at the separated plugin and enable it:

```bash
cat > .claude/settings.json <<'EOF'
{
  "extraKnownMarketplaces": {
    "spgr-pr0-marketplace": {
      "source": {
        "source": "directory",
        "path": "./test-marketplace/.claude-plugin"
      },
      "autoUpdate": false
    },
    "spgr-pr0-separated-marketplace": {
      "source": {
        "source": "directory",
        "path": "./test-marketplace/.claude-plugin"
      },
      "autoUpdate": false
    }
  },
  "enabledPlugins": {
    "spgr-pr0-plugin@spgr-pr0-marketplace": true,
    "spgr-pr0-separated-plugin@spgr-pr0-separated-marketplace": true
  }
}
EOF
```

(Note: the marketplace.json in this spike only lists one plugin, so adding a second marketplace entry pointing at the same location reuses the same plugin manifest. To test the separated layout properly, we may need to extend the marketplace.json to list `spgr-pr0-separated-plugin` at a different path. If the simple form above doesn't trigger the separated plugin's hook, fall back to extending marketplace.json:)

```bash
# Fallback: extend marketplace to list the separated plugin too
cat > test-marketplace/.claude-plugin/marketplace.json <<'EOF'
{
  "name": "spgr-pr0-marketplace",
  "version": "0.1.0",
  "plugins": [
    {
      "name": "spgr-pr0-plugin",
      "source": ".",
      "version": "0.1.0",
      "description": "in-marketplace plugin"
    },
    {
      "name": "spgr-pr0-separated-plugin",
      "source": "../../test-plugin",
      "version": "0.1.0",
      "description": "out-of-marketplace plugin"
    }
  ]
}
EOF

cat > .claude/settings.json <<'EOF'
{
  "extraKnownMarketplaces": {
    "spgr-pr0-marketplace": {
      "source": {
        "source": "directory",
        "path": "./test-marketplace/.claude-plugin"
      },
      "autoUpdate": false
    }
  },
  "enabledPlugins": {
    "spgr-pr0-plugin@spgr-pr0-marketplace": true,
    "spgr-pr0-separated-plugin@spgr-pr0-marketplace": true
  }
}
EOF
```

- [ ] **Step 4: Trigger session-start, capture both hooks' outputs**

```bash
rm -f .spike-out/marketplace-shape.txt .spike-out/separated-shape.txt
claude -p "say hi" --output-format text > .spike-out/session-claim3b.txt 2>&1 || true
sleep 1
echo "--- marketplace-shape ---"
cat .spike-out/marketplace-shape.txt 2>/dev/null || echo "MISSING"
echo "--- separated-shape ---"
cat .spike-out/separated-shape.txt 2>/dev/null || echo "MISSING"
```

- [ ] **Step 5: Compare CLAUDE_PLUGIN_ROOT in both files**

For the **separated** plugin, the expected (correct) value is `<spike-root>/test-plugin` — i.e., the plugin's own directory, NOT `<spike-root>/test-marketplace`.

If `separated-shape.txt`'s `CLAUDE_PLUGIN_ROOT` is:

- `<spike-root>/test-plugin` → **GREEN.** Resolves to the plugin's own root regardless of marketplace location. Spec assumption is correct.
- `<spike-root>/test-marketplace` (or `.../test-marketplace/.claude-plugin`) → **RED.** Resolves to the marketplace root. Our `.specgraph/agents/claude/` layout will break.
- Other → investigate, document verbatim.

- [ ] **Step 6: Classify the result**

Record verdict at `.spike-out/verdict-claim3.txt`:

```bash
{
  echo "claim3a (production shape): observed CLAUDE_PLUGIN_ROOT = ..."
  echo "claim3b (separated layout): observed CLAUDE_PLUGIN_ROOT = ..."
  echo
  echo "verdict: GREEN — resolves to plugin root in both cases"
  # or RED with the design-fallout note
} > .spike-out/verdict-claim3.txt
cat .spike-out/verdict-claim3.txt
```

If RED, design fallout per spec: re-architect Claude layout one level deeper, so plugin root and `.claude-plugin/` dir coincide. Record the exact path-resolution behaviour for the report.

---

## Task 6: Write the verification report

**Files:**

- Create: `<specgraph-repo>/docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md`

- [ ] **Step 1: Aggregate the three verdicts**

Return to the SpecGraph repo:

```bash
cd /Volumes/Code/github.com/specgraph
mkdir -p docs/plans
```

- [ ] **Step 2: Write the report**

Create `docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md` with this structure:

````markdown
# spgr-rwrp PR 0 — Claude API verification report

**Date:** 2026-05-08
**Claude version:** [output of `claude --version`]
**Spike location:** `~/Code/specgraph-pr0-spike/`
**Bead:** `spgr-XXXX` (the PR 0 bead from Task 0)

## Summary

| Claim | Verdict |
|---|---|
| 1. Relative path acceptance | GREEN / RED |
| 2. `autoUpdate: false` honoured | GREEN / RED |
| 3. `${CLAUDE_PLUGIN_ROOT}` resolves to plugin root | GREEN / RED |

PR A may proceed: YES / NO. If NO, the design fallout below applies.

## Claim 1 — Relative path acceptance

**Setup:** `.claude/settings.json` declared `extraKnownMarketplaces.spgr-pr0-marketplace.source = { type: "directory", path: "./test-marketplace/.claude-plugin" }`.

**Observed:**

```text
[paste contents of .spike-out/plugin-list-claim1.txt]
```

**Verdict:** GREEN/RED + one-sentence reason.

**Fallout (if RED):** [text from the spec or a refinement]

## Claim 2 — `autoUpdate: false`

**Setup:** Added `autoUpdate: false`. Bumped plugin.json version 0.1.0 → 0.1.1 on disk between Claude invocations.

**Observed:**

```text
[paste contents of .spike-out/plugin-list-claim2.txt]
[and/or .spike-out/marketplace-shape.txt]
```

**Verdict:** GREEN/RED + one-sentence reason.

**Fallout (if RED):** drop `autoUpdate` from `.claude/settings.json` managed keys.

## Claim 3 — `${CLAUDE_PLUGIN_ROOT}` resolution

**Setup 3a (production shape):** marketplace and plugin co-located in `test-marketplace/.claude-plugin/`.

**Observed:**

```text
[paste contents of .spike-out/marketplace-shape.txt]
```

**Setup 3b (separated layout):** marketplace at `test-marketplace/`, plugin at `test-plugin/`, marketplace.json pointing at `../../test-plugin`.

**Observed:**

```text
[paste contents of .spike-out/separated-shape.txt]
```

**Verdict:** GREEN/RED + one-sentence reason.

**Fallout (if RED):** re-architect Claude layout. Plugin root and `.claude-plugin/` dir must coincide. Update spgr-rwrp design doc Section 4 manifest entries 10–14 to nest one level deeper.

## Methodology notes

- Claude version observed: [version]
- Hook trigger: SessionStart, fired by `claude -p "say hi"`
- All artefacts live at `~/Code/specgraph-pr0-spike/.spike-out/` and are preserved for re-inspection.

## References

- Design: `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md`
- Plan: `docs/plans/2026-05-08-spgr-rwrp-pr0-plan.md`
- Claude plugins docs: <https://code.claude.com/docs/en/plugins>, <https://code.claude.com/docs/en/discover-plugins>, <https://code.claude.com/docs/en/plugins-reference>
````

Replace each `[paste …]` with the actual captured output. Replace `GREEN/RED` with the actual verdicts.

- [ ] **Step 3: Verify the report contents**

Run: `head -30 docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md`

Expected: header is filled in (no placeholders), Summary table shows three verdicts, Claim sections show observed output.

- [ ] **Step 4: Lint**

Run: `task lint:markdown`
Expected: 0 issues. If `rumdl` complains, fix inline (typically `MD040` for fenced blocks lacking a language tag — add `text` to bare ``` fences).

- [ ] **Step 5: Commit the report**

```bash
git add docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md
git commit -s -m "$(cat <<'MSG'
docs(plans): add spgr-rwrp PR 0 Claude API verification report

Verifies three claims that the spgr-rwrp design depends on:
1. Relative path acceptance in extraKnownMarketplaces.source.path
2. autoUpdate: false is honoured at the marketplace-entry level
3. \${CLAUDE_PLUGIN_ROOT} resolves to the plugin root, not the
   marketplace root or .claude-plugin/ subdir

Output drives the Go/No-Go decision for PR A and any layout fallback.

Closes spgr-XXXX (PR 0 bead).

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
MSG
)"
```

---

## Task 7: Apply spec fallout (if any claim was RED)

**Files:**

- Modify: `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md`

- [ ] **Step 1: For each RED claim, edit the spec to record the fallback**

If claim 1 RED: change all `path: "./..."` examples to `path: "/abs/..."` in §"Per-harness manifest" entry 9. Note in §"Architecture" that paths get resolved to absolute at init time.

If claim 2 RED: remove `"autoUpdate": false` from the example settings.json in §"Per-harness manifest" entry 9 and from §"Per-harness manifest" Claude row notes.

If claim 3 RED: rewrite §"Per-harness manifest" entries 10–14 with paths nested one level deeper. The plugin root becomes `.specgraph/agents/claude/.claude-plugin/`; hooks live at `.specgraph/agents/claude/.claude-plugin/hooks/...`. Update `extraKnownMarketplaces.spgr-pr0-marketplace.source.path` accordingly.

- [ ] **Step 2: Add a Revision-history entry**

Append to the Revision history section in the spec:

```markdown
- **2026-05-08 v5**: PR 0 verification report applied. [Briefly describe which claims were red and the design changes made. If all were green, note "all three claims green; spec stands as v4."]
```

- [ ] **Step 3: Commit the spec changes (if any)**

```bash
git add docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md
git commit -s -m "docs(plans): incorporate spgr-rwrp PR 0 verification report"
```

If all claims were GREEN, skip this task entirely — no spec changes needed.

---

## Task 8: Close the bead and clean up

- [ ] **Step 1: Close the PR 0 bead**

```bash
bd close <pr0-bead-id> --reason="Verification complete; report at docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md"
bd dolt push
```

- [ ] **Step 2: Optionally remove the scratch directory**

```bash
rm -rf ~/Code/specgraph-pr0-spike
```

(Optional — keeping the spike around is useful if the report needs revisiting. The directory is gitignored from any tracked location and consumes minimal space.)

- [ ] **Step 3: Push the spec doc**

```bash
git push origin HEAD:main  # or open a PR per the project convention
```

PR 0 is complete. PR A may now proceed (or proceed-with-fallout) per the report.

---

## Open questions / known limitations

- **Claude `-p` mode and plugin trust:** new plugins may require an interactive trust acknowledgement on first load. The `-p` mode docs say "the workspace trust dialog is skipped when Claude is run in non-interactive mode" but plugin trust may behave differently. If `-p` mode silently skips loading our plugin (rather than prompting), we won't see the hook fire and the spike returns nothing. Mitigation: run interactively once (`claude` no flags) and accept the trust prompt manually before re-running `-p` mode.
- **Hook timing:** SessionStart hooks may run asynchronously relative to the `-p` invocation's exit. The `sleep 1` in Tasks 4 and 5 is a heuristic; if hook artefacts are missing, increase to `sleep 5` and re-run.
- **Multi-plugin marketplace.json schema:** the spec for `marketplace.json` documents `plugins[].source` as a path or string. Step 5.3's fallback reuses this in a way the docs may not have explicitly covered. If the second-plugin entry fails to load, the report should note the schema constraint and we proceed with single-plugin verification only (claim 3a alone, no 3b separation).
