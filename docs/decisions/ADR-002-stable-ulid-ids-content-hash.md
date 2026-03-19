# ADR-002: Stable ULID IDs with Murmur3-128 Content Hash

- **Status:** Accepted
- **Date:** 2026-03-18
- **Bead:** spgr-hya
- **Supersedes:** Implicit "content-addressable ID" convention in docs and proto comments

## Context

The proto comments and site documentation describe spec and decision IDs as
"content-addressable" — implying the ID is derived from the spec's content and
changes when the content changes. However, the actual implementation generates
ULIDs (timestamp + randomness) via `newID()` in the Memgraph storage layer.
These are assigned once at creation and never change.

Content-addressable IDs that change on every mutation would break graph edges
(DEPENDS_ON, BLOCKS, COMPOSES, etc.) and require cascading updates across the
graph on every spec edit — significant complexity for the storage layer.

The system still needs a way to detect whether a spec's content has changed,
primarily for drift detection and sync adapter reconciliation.

## Decision

1. **Keep ULIDs as stable node IDs.** The `id` field on Spec and Decision
   remains a ULID (`spec-{ULID}`, `dec-{ULID}`), assigned once at creation.
   Graph edges reference these stable IDs and never need updating.

2. **Add a `content_hash` field.** A Murmur3-128 hash (32 hex characters)
   computed from the spec's substantive fields. Recomputed on every create
   and update.

3. **Hash inputs (Spec):** `intent`, `stage`, `priority`, `complexity`, and
   all authoring stage outputs (`spark_output`, `shape_output`,
   `specify_output`, `decompose_output`).

4. **Hash inputs (Decision):** `title`, `status` (string representation of
   the `DecisionStatus` enum), `decision`, `rationale`.

5. **Hash excludes:** `id`, `slug`, `version`, `created_at`, `updated_at`,
   `history`, `superseded_by`, `supersedes`, `notes`, `lifecycle`,
   `drift_acknowledged`, `drift_acknowledge_note`.

## Rationale

- **Graph stability:** Edges reference IDs. Changing IDs on content mutation
  would require O(edges) updates per edit — unacceptable complexity.
- **Change detection without diffing:** Comparing two 32-char strings is O(1)
  vs. comparing every field. Drift detection and sync adapters benefit directly.
- **Murmur3-128 over SHA-256:** This is fingerprinting, not security. Murmur3
  is ~10x faster and 32 hex chars is more readable than 64. 128 bits provides
  sufficient collision resistance for change detection.
- **Separate concerns:** The slug is the human name. The ULID is the machine
  pointer. The content hash is the fingerprint. Three jobs, three fields.

## Alternatives Considered

- **Content-addressable IDs (hash as the id):** Rejected — ID changes on every
  edit break graph edges. The complexity cost is not justified.
- **Hash only slug + spark as identity:** Rejected — unclear benefit over using
  slug as key. Two people sparking the same slug with different wording produce
  different IDs, defeating dedup.
- **SHA-256 full (64 hex chars):** Rejected — cryptographic strength unnecessary
  for fingerprinting. Longer string, slower computation, no benefit.
- **SHA-256 truncated:** Rejected — if we're truncating anyway, use a hash
  designed for the purpose (Murmur3) rather than truncating a crypto hash.

## Consequences

- Proto messages gain a `content_hash` field (Spec field 15, Decision field 10).
- Storage layer computes hash on create/update; no caller changes needed.
- Drift detection can compare hashes instead of field-by-field diffing.
- Sync adapters can use hash comparison for efficient reconciliation.
- Site docs and proto comments updated to remove "content-addressable" language
  from ID descriptions.
