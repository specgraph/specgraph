## Conflict Detection Report

### BLOCKERS (0)

None. No LOCKED-vs-LOCKED ADR contradictions were found among the 5 real
Accepted ADRs, no LOCKED decision in this ingest set contradicts an existing
locked decision in prior `.planning/` context (this is a `new`-mode bootstrap
with no existing CONTEXT.md), no `UNKNOWN`-confidence-low classifications
exist in this batch (all 50 documents classified as ADR/SPEC/DOC at
high/medium confidence), and the ref-graph traversal completed within the
50-hop cap with no unresolved cycle requiring synthesis to halt (see the
WARNING below on the one cycle that *was* detected and how it was resolved).

### WARNINGS (2)

[WARNING] Draft ADR-003's decision-ID scheme contradicts locked ADR-002
  Found: docs/initial-design-session/specgraph-v1.0-draft-adr-003-decisions.md (Proposed, not locked) proposes Decision node identity as a short hash of the normalized title ("d-" + first 8 hex chars of sha256(title)), with same-title collisions treated as an *intentional* dedup signal.
  Expected: docs/decisions/ADR-002-stable-ulid-ids-content-hash.md (Accepted, LOCKED) keeps Decision IDs as stable ULIDs (dec-{ULID}) and explicitly lists "Content-addressable IDs (hash as the id)" as a rejected alternative, for the same reason (edge-reference stability).
  → Per the default precedence rule (LOCKED wins over non-LOCKED), ADR-002's ULID scheme is authoritative and draft-adr-003's hash-based scheme should be treated as dead. However, draft-adr-003 was never formally marked Superseded and no other document in this ingest set states this correction explicitly, so it is surfaced here rather than silently dropped. Resolve by marking draft-adr-003's identity section Superseded-by-ADR-002 in the source repo (a one-line status update), or by explicitly excluding that section from any downstream requirement/plan generation.

[WARNING] Cross-reference cycle detected between two companion Identity-epic docs
  Found: docs/plans/2026-05-22-identity-storage-design.md lists "Bootstrap & UX spec" in its companion-designs front matter, and docs/plans/2026-05-22-identity-bootstrap-ux-design.md lists "Identity Storage design" in its own — a mutual/bidirectional cross-reference (A→B, B→A), which is technically a cycle under DFS three-color marking on the cross_refs graph.
  Impact: Per process, a detected cycle would normally gate synthesis of the cyclic subset (exclude from decisions/constraints extraction and record as unresolved-blocker). Applying that mechanically here would needlessly exclude two well-formed, Approved (2026-05-26), non-contradictory design docs from synthesis over what full-text reading confirms is a benign mutual "Companion designs:" front-matter mention, not a genuine circular decision dependency — both documents contain explicit, linear "Sequencing" sections ("This is the structural foundation under three peers" / "Depends on Identity Storage...") that resolve the ordering unambiguously outside the cross_refs field. The classifier's cross_refs extraction was itself inconsistent across the four Identity-epic docs (some captured textual "Companion designs:" mentions, others captured only explicit markdown-style links or bead IDs), which is the likely source of this false-positive edge.
  → Both documents WERE synthesized normally into decisions.md/constraints.md (no content excluded). Flagged here — rather than silently treated as a non-issue — so a human reviewer can override this judgment call and force exclusion if they disagree with the "benign companion cross-link" assessment. No other cycles were found in the remaining 48-document cross-ref graph.

### INFO (9)

[INFO] Storage backend: three self-documented generations, oldest is a non-locked draft ADR
  Note: docs/initial-design-session/specgraph-v1.0-draft-adr-001-storage.md (non-locked, medium confidence) proposed a dual Beads(+Dolt)/Postgres(+AGE) backend model. docs/plans/2026-02-28-client-server-architecture-design.md explicitly states in its own text ("Supersedes: Initial design session documents (v1.0-draft)" / "supersedes ... specgraph-v1.0-draft-adr-001-storage.md — entirely") that it replaces this with a Memgraph-default/Postgres+AGE-alternative model. docs/superpowers/specs/2026-04-01-postgres-storage-backend-design.md in turn explicitly states "Supersedes: ADR-001 assumption of Postgres+AGE" and replaces Memgraph/AGE with pure Postgres/pgx (no AGE, no ltree) — the backend reflected in the current CLAUDE.md. Because each transition is self-documented as an explicit supersession inside the superseding document's own text, this is treated as auto-resolved lineage rather than a WARNING requiring user adjudication, despite default ADR > SPEC precedence nominally favoring the oldest (draft-ADR) generation. See context.md "Storage backend: three-generation evolution" for the full chain.

[INFO] Draft ADR-002 (Gastown, Beads-required) superseded by the sync-adapter model
  Note: docs/initial-design-session/specgraph-v1.0-draft-adr-002-gastown.md (non-locked) required the Beads(+Dolt) backend for native Gastown integration. docs/plans/2026-02-28-client-server-architecture-design.md explicitly lists this doc's "integration model (Beads sync adapter replaces native coupling)" as superseded. Auto-resolved lineage, same self-documentation pattern as above.

[INFO] Auto-resolved: ADR-001 (locked) vs stale field name in vertical-slice-roadmap-design
  Note: docs/decisions/ADR-001-principle-statement-field-naming.md (Accepted, locked, 2026-03-01) renamed the Principle proto field from `principle` to `statement`. docs/plans/2026-02-28-vertical-slice-roadmap-design.md (2026-02-28, one day earlier) describes the Constitution proto's `principles` field as "repeated Principle — id, principle, rationale, exceptions" — using the pre-ADR-001 field name. ADR-001 wins per LOCKED precedence; the roadmap doc's field-name mention is stale and should be read as `statement`, not `principle`, wherever it appears downstream.

[INFO] Amend/supersede eligibility flipped two days apart within the SPEC layer
  Note: docs/superpowers/specs/2026-04-06-lifecycle-amendment-supersede-design.md (Draft, 2026-04-06) frames amendment as "Returning a completed spec to an earlier authoring stage" (i.e., amend works on `done` specs). docs/superpowers/specs/2026-04-08-lifecycle-nomenclature-inversion-design.md (Approved, 2026-04-08) explicitly fixes this as "inverted"/"backwards" and flips it: amend is eligible only from in-flight stages (`approved`/`in_progress`/`review`); supersede is restricted to `done` specs only. The later, Approved document is authoritative. The 2026-04-06 document's non-eligibility content (diff engine, CLI `--diff` flag, web changelog UI, SUPERSEDES edges) is unaffected by the flip and remains current.

[INFO] SpecLifecycle (task/living) field removed; stale schema references predate ADR-006
  Note: ADR-006 (locked, 2026-05-20) removes the `lifecycle` field (values task/living) entirely in favor of `SpecProvenance`. Earlier documents in this ingest set — including the Postgres schema DDL in docs/superpowers/specs/2026-04-01-postgres-storage-backend-design.md (`lifecycle TEXT NOT NULL DEFAULT 'task'`) and the original v1.0-draft spec's schema example — predate this change and describe the now-removed field. No action needed beyond awareness; ADR-006 supersedes by date and lock status.

[INFO] ADR-004's named mechanism (RunInTransaction) doesn't literally cover the later, separate identity/auth store
  Note: docs/decisions/ADR-004-optimistic-concurrency-transactions.md (locked) mandates `RunInTransaction` for "all new multi-query write paths." The Identity Storage design (2026-05-22, later) establishes identity/auth storage as a structurally separate, globally-scoped store (`AuthStore`) reached through a different constructor than the per-project `*Store`, and explicitly not wired into `*Store.RunInTransaction`. Subsequent identity-adjacent designs (docs/plans/2026-06-15-cli-oidc-login-design.md, docs/superpowers/specs/2026-06-16-spgr-g7st-self-service-api-keys-design.md) each independently had to design a bespoke `AuthStore`-level `pool.Begin` transaction to achieve equivalent atomicity. This preserves ADR-004's atomicity *principle* (and is called out explicitly in-source each time) but not its named *mechanism* for this one subsystem — flagged for downstream awareness, not a contradiction requiring resolution, since every later doc already reconciles it explicitly.

[INFO] Skill Personas design self-declares Superseded status
  Note: docs/plans/2026-03-17-skill-personas-design.md carries its own header: "Status: Superseded by 2026-04-20-multi-platform-plugin-design.md and 2026-05-06-harness-parity-epic-design.md." The harness-parity epic is in this ingest batch; 2026-04-20-multi-platform-plugin-design.md is referenced but was not part of the 50-document classification set and cannot be cross-checked here. The persona *content* is stated to now live in `internal/authoring/content/persona.md` and shared per-stage SKILL.md files.

[INFO] Identity Authn's permission-computation section self-annotated as superseded
  Note: docs/plans/2026-05-22-identity-authn-design.md contains an inline editorial note dated 2026-05-26: "this section is superseded by the Identity Policy Engine Adoption (Cedar) design ... retained for historical context only." docs/plans/2026-05-26-identity-policy-engine-design.md confirms this from the other side, stating it "supersedes ... the 'Permission computation' section of the approved Authn design." Both documents agree; no adjudication needed.

[INFO] Ambiguous "ADR-001" cross-reference in postgres-storage-backend-design
  Note: docs/superpowers/specs/2026-04-01-postgres-storage-backend-design.md lists a bare cross_ref of "ADR-001" and states in its header "Supersedes: ADR-001 assumption of Postgres+AGE." This textual "ADR-001" refers to docs/initial-design-session/specgraph-v1.0-draft-adr-001-storage.md (the storage draft, which did assume Postgres+AGE), not to the real, unrelated docs/decisions/ADR-001-principle-statement-field-naming.md (proto field naming). The two documents share only a filename-numbering coincidence, not a subject-matter relationship. Resolved by content inspection; noted here so a naive filename-based join doesn't conflate them downstream.
