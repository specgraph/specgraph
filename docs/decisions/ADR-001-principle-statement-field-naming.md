# ADR-001: Use 'statement' instead of 'principle' for Principle proto field

- **Status:** Accepted
- **Date:** 2026-03-01
- **Bead:** spgr-43g
- **Supersedes:** Slice 2 plan (Principle message definition)

## Context

The Slice 2 implementation plan specifies the Principle proto message with a
field named `principle` (field 2). During implementation, the field was renamed
to `statement`.

## Decision

Use `statement` as the field name for Principle field 2 in the proto schema and
all corresponding Go structs.

## Rationale

- **Avoids tautology:** `Principle.principle` is redundant and confusing
- **More descriptive:** The field holds the principle's statement text, not a
  nested principle
- **Consistency:** Matches the YAML struct `ConstitutionPrinciple.Statement`
- **Already propagated:** Emitter, CLI show, YAML bootstrap, and storage all
  use `statement` consistently

## Alternatives Considered

- **Keep `principle` per plan:** Rejected — tautological naming is a known
  anti-pattern; the plan is a design input, not a binding contract
- **Rename to `text`:** Rejected — too generic; `statement` better conveys the
  field's purpose

## Consequences

- Plan doc (`docs/plans/2026-02-28-slice-2-constitution-plan.md`) shows the old
  field name; this ADR supersedes that definition
- No migration needed — this is pre-v1 and no persisted data uses the old name
