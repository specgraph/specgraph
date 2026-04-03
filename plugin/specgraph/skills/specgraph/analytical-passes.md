# Analytical Passes Protocol

After persisting stage output, run analytical passes automatically. This
protocol defines how passes are dispatched, collated, and presented.

## Overview

The pass registry defines which passes auto-run (`autoIn`) and which are
offered (`offeredIn`) per stage and posture. This protocol runs after
the authoring skill persists its output (e.g., `specgraph shape <slug>`).

## Step 1: Determine Passes

The skill knows the slug, stage (implicit), and posture (from persona module).

- **Drive:** Run all passes for this stage (`autoIn` + `offeredIn`).
- **Partner:** Run `autoIn` passes only. Offer `offeredIn` in Step 6.
- **Support:** Run `autoIn` passes only. Offer `offeredIn` in Step 6 (note: `offeredIn` sets may differ from Partner per the registry).

Pass types use CLI-friendly kebab-case names:

| CLI name | Internal name |
|----------|--------------|
| `constitution-check` | `constitution_check` |
| `peripheral-vision` | `peripheral_vision` |
| `red-team` | `red_team` |
| `consistency` | `consistency_check` |
| `simplicity` | `simplicity_check` |

## Step 2: Health Check

```bash
specgraph health
```

If unreachable: "Server unreachable -- skipping analytical passes. Run them later when the server is back." Do not spawn subagents.

## Step 3: Dispatch Subagents

For each pass, dispatch a background subagent with `run_in_background: true`. The `specgraph pass run` output embeds the full pass template from the server, which includes the pass-specific persona, task description, evaluation framework, severity guidelines, and output format. No additional persona setup is needed.

Subagent prompt template:

````text
You are running the {pass_type} analytical pass on spec "{slug}".

{output of specgraph pass run <slug> --pass-type <pass_type>}

Execute the pass using the tools listed above. For each finding, assign a severity:
- FINDING_SEVERITY_CRITICAL: Blocks progress. Fundamental conflict or violation.
- FINDING_SEVERITY_WARNING: Should be addressed. Risk or inconsistency.
- FINDING_SEVERITY_NOTE: Informational. Context or minor suggestion.

When complete, write findings to a temp file and store them:

```bash
TMPFILE="$(mktemp "${TMPDIR:-/tmp}/findings-XXXXXXXX.json")"
trap 'rm -f "$TMPFILE"' EXIT
cat > "$TMPFILE" << 'FINDINGS_EOF'
{
  "findings": [
    {
      "severity": "FINDING_SEVERITY_WARNING",
      "summary": "one-line finding",
      "detail": "explanation",
      "constraint": "constitution principle if applicable",
      "resolution": "suggested fix"
    }
  ]
}
FINDINGS_EOF

if ! specgraph findings store {slug} --pass-type {pass_type} --json-file "$TMPFILE"; then
  echo "findings store failed" >&2
  exit 1
fi
```

Return a summary: count of findings at each severity level, and a one-line
description of each finding.
````

All subagents dispatched in parallel.

## Step 4: Collate

Wait for all background subagents to complete. Collect summaries. Order findings: critical first, then warning, then note. Group by pass type within each severity level.

## Step 5: Present Findings

Severity gating (all postures):

| Severity | Behavior |
|----------|----------|
| Critical | Gate: present each finding, ask user to address or acknowledge before offering stage transition |
| Warning | Present: show findings, disposition depends on posture |
| Note | Mention: show count and one-liners |
| No findings | "All passes completed -- no issues found." |

Posture modulation (how findings are presented, not what is shown):

| Posture | Critical | Warning | Note |
|---------|----------|---------|------|
| Drive | Present, ask to address or acknowledge | Present in one line, move on | Present count + one-liners, move on |
| Partner | Present, discuss | Present, ask how to proceed | Present count + one-liners, mention they're saved |
| Support | Present with explanation of why it matters | Present with context about the pass | Present with explanation of what the pass checks |

## Step 6: Offer Remaining Passes (Partner/Support only)

Drive already ran all passes in Step 1. For Partner/Support, if there are `offeredIn` passes not yet run:

- **Partner:** "I also have a {pass_name} pass available. Want me to run it?"
- **Support:** "There's also a {pass_name} pass -- it checks [explanation]. Want me to run it?"

If accepted, dispatch as a foreground subagent (single pass), then present findings per Step 5.

## Step 7: Transition

Offer to continue to the next stage, same as today.

## Error Handling

| Failure mode | Parent behavior |
|-------------|-----------------|
| Subagent returns error | "{pass}: failed -- {reason}. Other passes completed normally." |
| Subagent times out | "{pass}: no response. Remaining passes completed normally." |
| Partial success (pass ran, store failed) | Surface summary but note: "{pass} findings surfaced but not persisted -- store failed. Re-run later." |

Pass failures never block the authoring funnel. Passes are advisory.
