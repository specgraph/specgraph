<!-- SPDX-License-Identifier: Apache-2.0 -->

# ADR-006: Spec Provenance Model

- **Status:** Proposed
- **Date:** 2026-05-20
- **Supersedes:** SpecLifecycle enum (task/living)
- **Implementation:** see docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md

## Context

*(to be finalized in Task 8.3 after implementation)*

## Decision

Replace the `SpecLifecycle` enum + field with `SpecProvenance` (AUTHORED / RETROACTIVE_FROM_PR / DECLARED) plus a structured `provenance_detail` oneof. See design doc for full rationale.

## Consequences

*(to be finalized in Task 8.3)*
