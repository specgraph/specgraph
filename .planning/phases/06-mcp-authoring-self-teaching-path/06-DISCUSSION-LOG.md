# Phase 6: MCP Authoring Self-Teaching Path - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-14
**Phase:** 6-MCP Authoring Self-Teaching Path
**Areas discussed:** Write-input ergonomics, Skill-rewrite scope, CLI treatment in skills, Verification mechanism, Prime entry-point role

---

## Write-input ergonomics

### Turn 1 — how far to go

| Option | Description | Selected |
|--------|-------------|----------|
| Round-trip only, defer the rest | Ship only get→modify→update; server is the schema; defer mappers/alt-format | |
| Round-trip + forgiving mappers | Add referenceTypeFromString + wire constitutionLayerFromString into handleUpdate | |
| Round-trip + mappers + alt format | Also introduce YAML/TOML at the MCP write boundary | |

**User's choice:** Free-text — *"I'm not convinced that round trip or protojson is the right format here. We should explore yaml and/or some other token friendly format, or perhaps a change in tool signature to allow for setting section by section — door is wide open."*

### Turn 2 — which interface direction should research pursue

| Option | Description | Selected |
|--------|-------------|----------|
| Friendly whole-doc format | One tool, YAML/token-lean whole-constitution input instead of protojson | |
| Section-by-section tool actions | Granular actions (set-tech, add-principle, …); incremental build | |
| Both granular + friendly bulk | Granular actions + friendly bulk get/set | |
| Leave open, direct research to evaluate | Reject protojson blob as primary; research scores YAML/token-friendly vs section-by-section | ✓ |

**User's choice:** Leave open, direct research to evaluate.

### Turn 3 — scope of the interface rethink

| Option | Description | Selected |
|--------|-------------|----------|
| Both constitution + author funnel | Rethink covers constitution tool AND spark/shape/specify/decompose outputs | ✓ |
| Constitution tool only | Only the constitution tool; funnel stays protojson for now | |

**User's choice:** Both constitution + author funnel.

**Notes:** Strongest and most consequential area. Departs from issue #1002's round-trip proposal — user wants a genuine design comparison, prioritizing an interface an MCP-only agent cannot get wrong, and token efficiency.

---

## Skill-rewrite scope

| Option | Description | Selected |
|--------|-------------|----------|
| 2 deep + 5 audit-gate | Deep rewrite constitution + authoring; lighter audit-gate on the other 5 | |
| All 7 full rewrite | Full MCP-first rewrite of all 7 embedded skills | ✓ |
| 2 skills only, defer 5 | Only the two authoring-path skills; defer the rest | |

**User's choice:** All 7 full rewrite.

---

## CLI treatment in skills

| Option | Description | Selected |
|--------|-------------|----------|
| Demote CLI to gated appendix | MCP-first; CLI in a "Requires local CLI" appendix per skill | ✓ |
| Remove CLI from skills entirely | Skills become purely MCP; CLI docs live elsewhere | |
| Co-equal, labeled per step | Keep CLI + MCP as co-equal labeled paths | |

**User's choice:** Demote CLI to gated appendix.

---

## Verification mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Automated MCP-only e2e test | Drives prime→skills→tool calls with CLI unavailable to approved/completed state | ✓ |
| Manual walkthrough doc | Documented human/agent walkthrough | |
| Both automated + manual | e2e gate + manual evidence doc | |

**User's choice:** Automated MCP-only e2e test.

---

## Prime entry-point role

| Option | Description | Selected |
|--------|-------------|----------|
| Prime routes to skills | Prime stays orientation, made a reliable entry point routing to authoring skills | ✓ |
| Prime teaches the pattern inline | Embed the authoring pattern directly in prime output | |
| No prime change unless broken | Leave prime as-is | |

**User's choice:** Prime routes to skills.

---

## the agent's Discretion

- Specific write-input mechanism (YAML vs token-friendly vs section-by-section vs blend) delegated to the research phase to score and recommend; planning locks.
- e2e harness details for simulating "MCP-only / no CLI" left to planning/implementation.

## Deferred Ideas

- None — discussion stayed within phase scope.
- Concern (not deferred scope): `specgraph://prime` failed to load in this session ("internal error"); flagged for research to confirm reliability since prime is now the entry point.
