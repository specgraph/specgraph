<!-- SPDX-License-Identifier: Apache-2.0 -->

# ADR-006: Spec Provenance Model

- **Status:** Accepted
- **Date:** 2026-05-20

## Context

SpecGraph specs previously carried a `lifecycle` field with values `task` and `living`. The intent was to distinguish one-time work (task) from ongoing contracts (living). The actual implementation encoded only the type tag — `GetReady` never consulted it, drift detection ignored it, and the conceptual boundary between `task`/`living`/`done` was vestigial after `done`.

Adding a `retroactive` lifecycle value (an early proposal) would have created a third value behaviorally identical to LIVING, distinguished only by provenance.

## Decision

Replace `SpecLifecycle` with `SpecProvenance` — an enum capturing **how a spec entered the graph**, not how the funnel should treat it after `done`. Values: `AUTHORED`, `RETROACTIVE_FROM_PR`, `DECLARED`. Per-variant structured payload via a `provenance_detail` oneof at proto fields 22–24.

Stage drives funnel behavior; provenance drives the forward-vs-imported axis. Done specs of any provenance share drift, dependency, and supersession semantics.

See `docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md` for full design rationale, alternatives considered, and adversarial review trail.

## Consequences

- Wire-break at proto field 10 (pre-1.0, no production data).
- `specgraph ready` semantics tightened: only AUTHORED specs at `stage=approved` with no active claim.
- `claim` and `report-completion` reject non-AUTHORED specs (sentinels `ErrClaimRequiresAuthored` / `ErrCompletionRequiresAuthored`).
- Drift detection unified on `stage=done`.
- Provenance is immutable through amend; supersede creates a fresh spec with fresh provenance.
- Adding new provenance values (e.g. `IMPORTED_FROM_BACKUP`) later is wire-compatible per proto enum evolution rules.
