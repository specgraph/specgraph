# Analytical Passes Protocol

After persisting stage output, run analytical passes automatically. This
protocol defines how passes are dispatched, collated, and presented.

## Overview

The pass registry defines which passes auto-run (`autoIn`) and which are
offered (`offeredIn`) per stage and posture. This protocol runs after the
authoring step persists its output.

## Step 1: Determine Passes

The agent knows the slug, stage (implicit), and posture (from persona module).

- **Drive:** Run all passes for this stage (`autoIn` + `offeredIn`).
- **Partner:** Run `autoIn` passes only. Offer `offeredIn` passes in Step 5.
- **Support:** Run `autoIn` passes only. Offer `offeredIn` passes in Step 5
  (note: `offeredIn` sets may differ from Partner per the registry).

Pass types carry both a client-facing kebab-case name and an internal snake_case identifier:

| Client-facing name | Internal name |
|--------------------|---------------|
| `constitution-check` | `constitution_check` |
| `peripheral-vision` | `peripheral_vision` |
| `red-team` | `red_team` |
| `consistency` | `consistency_check` |
| `simplicity` | `simplicity_check` |

## Step 2: Dispatch Passes

For each applicable pass, run the pass against the named spec. The server
provides each pass's template (persona, task, evaluation framework, severity
guidelines, output format) via a server-provided pass template so the pass
runner does not need additional prompt setup. Dispatch passes in parallel when
the platform supports it; otherwise run sequentially.

For each finding, assign a severity:

- `CRITICAL`: Blocks progress. Fundamental conflict or violation.
- `WARNING`: Should be addressed. Risk or inconsistency.
- `NOTE`: Informational. Context or minor suggestion.

Persist findings via the server's findings-storage tool. Return a per-pass
summary (count by severity + one-line description of each finding) to the
parent conversation.

## Step 3: Collate

Wait for all passes to complete. Collect summaries. Order findings: critical
first, then warning, then note. Group by pass type within each severity level.

## Step 4: Present Findings

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

## Step 5: Offer Remaining Passes (Partner/Support only)

Drive already ran all passes in Step 1. For Partner/Support, if there are
`offeredIn` passes not yet run:

- **Partner:** "I also have a {pass_name} pass available. Want me to run it?"
- **Support:** "There's also a {pass_name} pass -- it checks [explanation]. Want me to run it?"

If accepted, run the single pass, then present findings per Step 4.

## Step 6: Transition

Offer to continue to the next stage.

## Error Handling

| Failure mode | Behavior |
|--------------|----------|
| Pass task returns error | "{pass}: failed -- {reason}. Other passes completed normally." |
| Pass task times out | "{pass}: no response. Remaining passes completed normally." |
| Partial success (pass ran, store failed) | Surface summary but note: "{pass} findings surfaced but not persisted -- store failed. Re-run later." |

Pass failures never block the authoring funnel. Passes are advisory.
