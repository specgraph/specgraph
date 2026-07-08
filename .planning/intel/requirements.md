# Requirements Intel

## No PRD-classified documents in this ingest batch

All 50 classified documents in `CLASSIFICATIONS_DIR` resolved to type `ADR` (8),
`SPEC` (41), or `DOC` (1). **Zero documents classified as `PRD`.**

Per the doc-synthesizer process, requirements are extracted only from PRD-type
documents (one PRD normally yields multiple `REQ-{slug}` entries with acceptance
criteria). Since this corpus contains no PRDs, there is nothing to extract here.

Functional/behavioral intent in this codebase is instead captured as:

- **ADR decisions** — see `decisions.md`
- **SPEC design constraints** (API contracts, schemas, NFRs, protocols) — see `constraints.md`
- **Narrative/roadmap context** — see `context.md`

Downstream (`gsd-roadmapper`) should treat `constraints.md` as the primary source
of implementation-level requirements for this project, since the corpus is
design-doc-driven rather than PRD-driven. There is consequently **no
`competing-variants` bucket populated** in `INGEST-CONFLICTS.md` — that bucket is
specifically for divergent acceptance criteria across PRDs on the same
requirement, which cannot occur with zero PRDs in the input set.

If PRDs are added in a future ingest pass (`merge` mode), re-run ingestion to
populate this file properly.
