# Requirements Intel

Synthesized from the 2 PRD-classified documents in the full 177-document ingest corpus.
(The prior 50-doc-only round found zero PRDs; this round's larger corpus surfaced two.)
Neither PRD overlaps in scope with the other, so there are **no competing acceptance-criteria
variants** in this corpus — the `competing-variants` bucket in `INGEST-CONFLICTS.md` is empty.

Functional/behavioral intent for the core SpecGraph product is still primarily captured as
ADR decisions (`decisions.md`) and SPEC design constraints (`constraints.md`), since this
corpus is overwhelmingly design-doc-driven. These two PRDs cover documentation/release-gate
scope and an external-facing template product, respectively — narrower slices than the core
system.

---

## REQ-quickstart-docs-overhaul

- **source:** `docs/superpowers/specs/2026-03-20-quickstart-and-docs-overhaul-design.md`
- **title:** Quick Start Guide and Documentation Overhaul for 0.1.0
- **status:** Approved
- **description:** Last release gate before SpecGraph 0.1.0 (first public release). Requires (1) a Quick Start guide taking a new user from install to first authored spec, (2) a full accuracy review of existing site docs against actual 0.1.0 capabilities, (3) release-please wiring to keep in-doc version numbers current, and (4) removal of internal project-management artifacts (roadmap) from user-facing docs.
- **acceptance criteria (from Goals):**
  1. `site/docs/quickstart.md` exists with flow: Install → Start Server → Install Claude Code Plugin → Author First Spec → Check Drift → Next Steps, using the `/healthz` endpoint example (chosen over the OAuth2 example in `example-spec.md` for approachability).
  2. Every existing doc reviewed and corrected for truthfulness against 0.1.0 capabilities.
  3. Version numbers in docs use `x-release-please-version` inline annotations (or block markers for multi-line blocks) with `extra-files` `"type": "generic"` config, since `.md` has no default release-please updater.
  4. `site/docs/roadmap.md` (internal PM artifact) removed from user-facing docs.
- **explicit non-goals:** README rewrite, API reference docs, site deployment (CloudPages), analytical pass execution (deferred to 0.2.0 milestone beads).
- **companion implementation plan:** `docs/superpowers/plans/2026-03-20-quickstart-and-docs-overhaul.md` — confirms the same scope, no divergence found.
- **later superseded/extended by:** `docs/superpowers/specs/2026-04-03-site-docs-overhaul-design.md` (updates docs for the Postgres migration, removes Memgraph terminology) and `docs/superpowers/specs/2026-04-10-site-narrative-restructure-design.md` (IA/narrative restructure for enterprise tech-lead audience) — both are DOC-classified follow-on doc-maintenance passes, not competing requirements; they extend rather than contradict this PRD's acceptance criteria.

## REQ-confluence-specgraph-design-bridge

- **source:** `docs/designs/2026-03-26-confluence-to-specgraph-design-bridge.md`
- **title:** Confluence-to-SpecGraph Design Bridge
- **status:** RFC / DRAFT (not locked, not Approved)
- **description:** A lightweight Confluence + markdown system-design template that lets teams produce structured designs today (before SpecGraph's authoring funnel is ready for them) while mapping cleanly onto SpecGraph's Spark and Shape authoring stages, enabling zero-rework migration later.
- **scope (In):** Template covering intent validation, scope alignment, and directional approval — the "should we build this?" question. Sections map 1:1 to Spark/Shape stages. Confluence variant uses Page Properties macro for structured, queryable metadata. Markdown variant for docs-as-code teams.
- **scope (Out):** Automated Confluence→SpecGraph ingestion; Specify-level detail (interface contracts, verify criteria, invariants, file touches); Decompose-level detail (implementation slices); retrofitting pre-existing designs.
- **chosen approach:** "Spark + Shape only" (rejected: full 5-stage funnel mirror — too heavyweight; rejected: freeform with style guide only — inconsistent, hard to migrate).
- **acceptance criteria — Must (ship-blockers):**
  1. Template sections map to SpecGraph Spark + Shape fields with no ambiguity — a future ingestion tool can extract structured data without human interpretation.
  2. A team unfamiliar with spec-driven development can fill in the template in under an hour for a medium-complexity design.
  3. Page Properties macro works with Page Properties Report on a parent page.
- **acceptance criteria — Should (strong expectations):**
  1. At least one team uses the template before SpecGraph is available, validating practicality.
  2. Placeholder text is self-explanatory (no separate training needed).
- **acceptance criteria — Won't (explicitly deferred, MoSCoW):**
  1. Automated Confluence-to-SpecGraph ingestion.
  2. Retrofitting existing designs into this template.
  3. Sub-templates for child/depth pages (API design, deployment, etc.).
- **risk noted in-source:** This work could be killed outright if SpecGraph ships its full authoring UI before any new designs are started — an explicit self-aware kill-condition, not a hidden dependency.

---

## Cross-cutting note

No requirement in either PRD overlaps in scope with a requirement in the other PRD (one is
an internal docs/release gate, the other is an external-facing Confluence template product),
so there is no `competing-variants` entry to raise. If a future ingest adds a PRD that touches
either documentation-overhaul or design-template scope, re-check for acceptance-criteria overlap
before merging into `REQUIREMENTS.md` downstream.
