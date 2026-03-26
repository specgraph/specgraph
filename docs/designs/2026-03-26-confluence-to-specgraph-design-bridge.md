# Confluence-to-SpecGraph Design Bridge

| | |
|---|---|
| **Status** | RFC / DRAFT |
| **Owner** | Sean Brandt |
| **Date** | 2026-03-26 |
| **Intent** | Give teams a lightweight system design template in Confluence that maps cleanly to SpecGraph's authoring funnel, enabling zero-rework migration when SpecGraph is ready. |

## Context & Signal

Teams need to produce system designs now, but SpecGraph's authoring funnel isn't ready for them yet. Without a bridge, teams will either (a) write unstructured docs that are hard to migrate later, or (b) wait for SpecGraph and lose momentum. We need a template that's useful today and feeds into SpecGraph tomorrow.

This work could be killed if SpecGraph ships its full authoring UI before any new designs are started.

## Scope

### In

- A system design template (Confluence and markdown) that covers intent validation, scope alignment, and directional approval — the "should we build this?" question
- Template sections that map 1:1 to SpecGraph's Spark and Shape stages so content carries forward without rework
- A documented migration path: which template fields feed which SpecGraph funnel stages
- Confluence variant uses Page Properties macro for structured metadata enabling page-properties-report queries across designs
- Markdown variant lives in-repo for teams that don't use Confluence or prefer docs-as-code

### Out

- Automated ingestion from Confluence into SpecGraph — that's a SpecGraph feature, not a template concern
- Specify-level detail (interface contracts, verify criteria, invariants, file touches) — deliberately omitted because that's where SpecGraph adds the most value over a static doc
- Decompose-level detail (implementation slices, execution ordering) — belongs in tickets after design approval
- Retrofitting existing designs into this format — designs that predate the template can stay as-is

## Approach & Decisions

| Approach | Description | Tradeoffs |
|----------|-------------|-----------|
| Full SpecGraph mirror | Replicate all 5 funnel stages (Spark through Decompose) as Confluence sections | Perfect migration fidelity. But overwhelmingly large for teams unfamiliar with spec-driven development; Specify and Decompose are tedious without tooling support. |
| Spark + Shape only | Cover the first two funnel stages (intent validation and design shaping) with lightweight sections, leave Specify and Decompose for SpecGraph | Right level of detail for approval decisions. Teams fill in Specify/Decompose when SpecGraph is available and those stages actually add value. Slight gap: no formal interface contracts in the template. |
| Freeform with conventions | No template; publish a style guide and let teams structure docs however they want | Maximum flexibility. But inconsistent structure across teams makes migration to SpecGraph a manual, per-doc effort. |

**Chosen:** Spark + Shape only — covers the approval surface (intent, scope, direction, success criteria) without pulling teams into implementation detail they don't need yet. The sections that are omitted (Specify, Decompose) are precisely where SpecGraph's tooling adds the most value, making the migration story compelling rather than redundant.

### Key Decisions

| Decision | Rationale |
|----------|-----------|
| Page Properties macro for metadata | Enables Page Properties Report macro on a parent page to show a dashboard of all designs with their status, owner, and intent. Also makes metadata machine-readable for future ingestion. |
| MoSCoW for success criteria, not a requirements table | "Won't" items are as important as "Must" items for scope control. Traditional requirements tables only capture what's in scope; MoSCoW forces explicit documentation of what was considered and rejected. |
| Depth goes in child pages, not inline | Keeps the top-level page scannable for approval decisions. Child pages are added after approval, not before — prevents over-investment in detail before direction is validated. |
| No Specify or Decompose sections | These stages are tedious without tooling (interface contracts, verify criteria, file touches, slice dependency graphs). Omitting them creates a natural on-ramp to SpecGraph: "you've validated the design in Confluence, now let SpecGraph help you make it precise and executable." |

## Success Criteria

### Must (ship-blockers)

- Template sections map to SpecGraph Spark + Shape fields with no ambiguity — a future ingestion tool can extract structured data without human interpretation
- A team unfamiliar with spec-driven development can fill in the template in under an hour for a medium-complexity design
- Page Properties macro works with Page Properties Report on a parent page

### Should (strong expectations)

- At least one team uses the template before SpecGraph is available, validating that it's practical and sufficient for approval decisions
- Placeholder text is clear enough that teams don't need separate instructions or training to use the template

### Won't (explicitly deferred)

- Build automated Confluence-to-SpecGraph ingestion — manual migration is fine for the expected volume of designs
- Retrofit existing designs into this template — comprehensive designs that predate the template don't need reformatting
- Create sub-templates for child pages (API design, deployment, etc.) — teams can structure depth pages however suits their domain

## Dependencies & Risks

| Dependency / Risk | Impact | Mitigation |
|-------------------|--------|------------|
| SpecGraph proto schema changes | If SparkOutput or ShapeOutput fields change, the template's section mapping drifts | Template is intentionally higher-level than proto fields. Mapping is conceptual (scope = scope, decisions = decisions), not field-for-field, so minor proto changes don't break it. |
| Teams add non-standard sections | Extra sections make automated ingestion harder | Acceptable. Extra content goes into SpecGraph notes/description fields. The core sections are what matter for structured extraction. |
| Template adoption resistance | Teams skip the template and write freeform docs | Template is deliberately small (7 sections, ~1 page when filled). Lower ceremony than most design doc templates. If teams still resist, the content is still extractable — just with more manual mapping. |

## Open Questions

| # | Question | Owner | Target Date | Resolution |
|---|----------|-------|-------------|------------|
| 1 | Should we add a Page Properties Report macro to the parent page immediately, or wait until there are 3+ designs using the template? | Sean | | |
| 2 | When SpecGraph ingestion is built, should it pull from the Confluence API (structured extraction from Page Properties + known heading structure) or expect a manual copy-paste step? | Sean | | |

---

## Appendix: Template-to-SpecGraph Field Mapping

Reference for future ingestion work. Shows how each template section feeds into SpecGraph's authoring funnel.

| Template Section | SpecGraph Stage | Field(s) |
|------------------|----------------|----------|
| Page Properties: Intent | Spark | `seed` |
| Context & Signal | Spark | `signal`, `kill_test` |
| Scope (In / Out) | Shape | `scope.in`, `scope.out` |
| Approach & Decisions (table) | Shape | `approaches[]` (name, description, tradeoffs, chosen) |
| Key Decisions | Shape | `decisions[]` — become first-class Decision nodes with DECIDED_IN edges |
| Success Criteria (Must/Should/Won't) | Shape | `success_criteria.must[]`, `should[]`, `wont[]` |
| Dependencies & Risks | Shape + Graph edges | Dependencies become `DEPENDS_ON` edges; risks inform scope notes |
| Open Questions | Metadata | Resolved questions feed into `decisions[]`; unresolved stay as spec notes |

### Templates

Two variants of the same template, identical in structure:

- **Confluence** — Uses Page Properties macro for structured metadata; placeholder text in grey; intended for wiki-based teams
- **Markdown** — [`docs/designs/template-system-design.md`](../designs/template-system-design.md) — plain markdown with YAML-style header table; intended for docs-as-code workflows or teams without Confluence

Both contain these sections, each with placeholder guidance text:

1. **Header** — Status, Owner, Date, Intent
2. **Context & Signal** — Why now? What triggered this?
3. **Scope** (In / Out) — What's included and explicitly excluded
4. **Approach & Decisions** — Options considered, chosen approach, key decisions with rationale
5. **Success Criteria** — Must / Should / Won't (MoSCoW)
6. **Dependencies & Risks** — External blockers with impact and mitigation
7. **Open Questions** — Unresolved items with owners and target dates
