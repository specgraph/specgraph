# Analytical Passes & Safety Net

## Overview

SpecGraph includes two complementary quality mechanisms: **structured analytical
passes** that run during authoring, and an **always-on safety net** that catches
critical issues. Together they ensure specs are thorough, consistent, and safe
before they ever reach an executor.

Passes are optional depth — they sharpen thinking. The safety net is mandatory
protection — it catches danger. Both run during the
[authoring funnel](authoring.md), but they serve fundamentally different
purposes and follow different rules.

!!! info "Planned"
    Pass scheduling infrastructure is fully implemented — passes are
    registered per-stage with posture-aware auto/offered rules
    (`internal/authoring/passes.go`). Pass execution currently returns
    placeholder findings. LLM-driven pass execution is planned.

---

## Analytical Passes

Each pass runs at a specific point in the authoring funnel, asks a focused
question, and produces structured findings. Passes are designed to be run by AI
agents, human reviewers, or both — the output format is the same regardless of
who performs the analysis.

| Pass | Trigger Stage | Question | Severity |
|---|---|---|---|
| Red Team | After Specify | What could go wrong? | Critical / Warning / Note |
| Peripheral Vision | After Shape | What else should we think about? | — |
| Consistency Check | After Specify | Does this contradict anything? | — |
| Simplicity Check | After Decompose | Can this be simpler? | — |
| Constitution Check | Every stage | Does this respect our rules? | Critical / Warning / Note |

### Red Team

**Runs after:** Specify
**Question:** "What could go wrong?"

The red team pass takes an adversarial stance against the spec. It challenges
correctness, safety, edge cases, and failure modes — the things that are easy
to miss when you are focused on making something work. Each finding carries a
severity: **critical** (blocks approval), **warning** (should address), or
**note** (worth knowing).

> *Example:* "If two rotation requests arrive simultaneously, what prevents a
> race condition? The spec defines no concurrency invariant for the token store."

### Peripheral Vision

**Runs after:** Shape
**Question:** "What else should we think about?"

During shaping, context expands. The peripheral vision pass captures
out-of-scope concerns that surfaced while exploring the problem space. Each
finding gets triaged into one of three buckets:

- **Add to this spec** — the concern is in-scope and was missed
- **Create a separate spec** — the concern is real but belongs elsewhere
- **Note it** — worth recording but no action needed now

> *Example:* "Token rotation may affect existing session management — should
> that be a separate spec?"

### Consistency Check

**Runs after:** Specify
**Question:** "Does this contradict anything?"

The consistency check walks the graph looking for conflicts between the current
spec and specs that are already approved or in progress. It examines interface
contracts, invariants, shared dependencies, and overlapping scope.

> *Example:* "This spec's interface contract conflicts with the API versioning
> approach in `spec-auth-v2`. Both define `POST /auth/token` with incompatible
> response schemas."

### Simplicity Check

**Runs after:** Decompose
**Question:** "Can this be simpler?"

After a spec is decomposed into child specs, the simplicity check looks for
complexity reduction opportunities. It flags duplication across children, overly
fine-grained splits, and cases where combining specs would reduce total
integration surface.

> *Example:* "Three of the five child specs share the same database migration —
> consider combining them into a single spec with a shared migration step."

### Constitution Check

**Runs at:** Every stage
**Question:** "Does this respect our rules?"

The constitution check validates the spec against the project's
[constitution](constitution.md) at every stage of the funnel. It catches
violations of hard constraints (forbidden technologies, required patterns,
naming conventions) and surfaces soft guidance that may be relevant. Like the
red team pass, findings carry severity levels.

> *Example:* "Spec uses Redis, but the constitution lists it as forbidden with
> reason: 'Team has no Redis expertise.'"

---

## Posture-Aware Execution

Passes run differently depending on the active AI posture:

- **Drive** — Passes run automatically after each stage. Results appear in the
  authoring output without asking. The AI assumes you want thorough analysis and
  delivers it proactively.
- **Partner** — Passes are offered at each stage ("Want me to run a red team
  pass?"). The user decides which passes to run and which to skip. The AI
  suggests but does not assume.
- **Support** — Passes are held unless explicitly requested. The user leads the
  analysis and asks for specific passes when they want them. The AI stays out of
  the way until called on.

This means the same authoring flow adapts to the user's working style without
changing the underlying quality checks. A senior engineer in Support mode gets a
lightweight flow. A team onboarding a new domain in Drive mode gets maximum
coverage. The passes themselves are identical — only the triggering behavior
changes.

---

## The Safety Net

The safety net is a separate system from analytical passes. It is **always on**,
runs at **every stage**, and **cannot be skipped or deferred** — regardless of
posture, user preference, or urgency. If the safety net flags something, the
finding is attached to the spec and surfaced immediately.

The safety net catches two categories:

- **Security issues** — hardcoded credentials, disabled authentication, missing
  encryption, code execution patterns, exposed secrets
- **Data loss risks** — destructive operations without rollback plans, missing
  migration strategies, irreversible state changes

Additional categories (consistency contradictions, constitution violations, and
showstoppers) are planned for the analytical pass system but not yet implemented
in the safety net.

Patterns the safety net checks for (from `internal/authoring/safety.go`):

```text
CRITICAL security:  "hardcoded secret", "hardcoded password",
                    "disable auth", "skip validation",
                    "no encryption", "rm -rf"
WARNING  security:  "credential", "injection", "eval(",
                    "exec(", "plaintext"
CRITICAL data_loss: "drop table", "drop all", "delete all",
                    "without migration", "without backup",
                    "no rollback", "force delete"
WARNING  data_loss: "truncate", "purge"
```

The safety net does not perform deep analysis — that is what the analytical
passes are for. It performs fast pattern matching to catch the things that should never
ship, no matter how rushed the timeline.

---

## Why Both?

Analytical passes and the safety net exist because quality has two distinct
dimensions. Passes are **analytical** — they deepen understanding, surface
tradeoffs, and improve the spec's design. They are posture-aware because
experienced developers may not need every pass on every spec. A senior engineer
writing a well-understood CRUD endpoint can skip the red team pass and still
produce a solid spec. The passes respect that judgment.

The safety net is **protective** — it catches patterns that are dangerous
regardless of context or expertise. Hardcoded credentials are wrong whether you
are a junior developer or a principal engineer. A circular dependency is broken
whether you are in Drive mode or Support mode. The safety net does not care about
posture because the things it catches are not matters of judgment — they are
structural defects. You can skip a red team pass if you are confident. You
cannot skip a security check. Different purposes, complementary systems.

---

## Working with Findings

Findings from analytical passes are stored as graph nodes linked to specs
via `HAS_FINDING` edges. Use the CLI to inspect them:

```bash
# List all findings for a spec
specgraph findings list <slug>

# Filter by pass type
specgraph findings list <slug> --pass-type constitution-check
specgraph findings list <slug> --pass-type red-team
```

Available pass types: `constitution-check`, `red-team`, `peripheral-vision`,
`consistency`, `simplicity`.
