# Wire Authoring Skills to Call RecordConversation

**Date**: 2026-03-26
**Status**: Approved
**Bead**: spgr-cdd

## Problem

The RecordConversation RPC and Memgraph storage exist end-to-end (ConversationLog nodes, AUTHORED_VIA/CONTINUES/EXPLAINS edges), but none of the authoring skills call it. The conversation log is always empty. The dashboard's conversation count column (spgr-5re) shows 0 because no data flows into the graph.

## Changes

### 1. Shared Conversation Recording Reference

Create `plugin/specgraph/skills/specgraph/conversation-recording.md` (alongside `persona.md` in the shared skill directory — flat, no `references/` subdirectory).

Contents:

- **When to record**: After elicitation and synthesis confirmation, alongside the `specgraph <stage>` persistence command.
- **What to capture**: Each probe/response pair as a ConversationExchange, plus the synthesis summary and user's confirmation/rejection as a final pair. Flag decision points where the user chose between alternatives.
- **How to accumulate**: The LLM maintains a running list of exchanges during the conversation. At persistence time, it writes the full list to a JSON temp file.
- **JSON schema**: The ConversationExchange format:

```json
{
  "exchanges": [
    {
      "role": "probe",
      "content": "What's the idea?",
      "stage": "spark",
      "sequence": 1
    },
    {
      "role": "response",
      "content": "A caching layer for the API",
      "stage": "spark",
      "sequence": 1,
      "decision_point": false
    }
  ]
}
```

- **CLI invocation**:

```bash
cat > /tmp/conv-<slug>.json << 'CONV_EOF'
{ "exchanges": [ ... ] }
CONV_EOF
specgraph conversation record <slug> --stage <stage> --json-file /tmp/conv-<slug>.json
rm /tmp/conv-<slug>.json
```

- **`--amend` flag**: Omit for first-pass recordings. Use `--amend` when the skill is re-entering a stage via the amend flow (the exchange is correcting previous output, not producing it fresh).
- **Approve special case**: Only record on rejection. The approval flow's substantive exchanges (checklist evaluation, concerns raised/resolved) are worth capturing, but only when the outcome is a hold/decline — that's the decision trail with value. Clean approvals are self-evident from the `specgraph approve` call itself.
- **Error handling**: Conversation recording is non-critical. If the CLI call fails, log the error but do not abort the stage persistence. The structured output is the primary artifact.

**Symlink pattern**: Each target skill symlinks from `references/conversation-recording.md` to `../../specgraph/conversation-recording.md` (same relative path pattern as `persona.md`).

### 2. Skill Modifications

Each skill's Persistence section gets a step pointing to the shared reference:

````markdown
N. **Record the conversation:** See `references/conversation-recording.md` for the
   exchange format and accumulation pattern.

   ```bash
   cat > /tmp/conv-<slug>.json << 'CONV_EOF'
   { "exchanges": [ ... accumulated exchanges ... ] }
   CONV_EOF
   specgraph conversation record <slug> --stage <stage> --json-file /tmp/conv-<slug>.json
   rm /tmp/conv-<slug>.json
   ```
````

| Skill | Location | Notes |
|-------|----------|-------|
| `specgraph-spark` | Persistence section, after `specgraph spark` call | Exchanges: seed, signal, scope sniff, kill test |
| `specgraph-shape` | Persistence section, after `specgraph shape` call | Exchanges: scope-in/out, approaches, risks, success criteria, decisions |
| `specgraph-specify` | Persistence section, after `specgraph specify` call | Exchanges: interfaces, verify criteria, invariants, file touches |
| `specgraph-decompose` | Persistence section, after `specgraph decompose` call | Exchanges: strategy, slice definitions, dependencies |
| `specgraph-approve` | After "human declines or wants changes" block (line 163) | Only on hold/decline. No preceding `specgraph approve` call — the recording replaces the approve command as the persistence action for rejections. |

### 3. Enable Real Conversation Count in ListSpecs

Replace the `0 AS conversation_count` placeholder in `internal/storage/memgraph/memgraph.go` ListSpecs query with a real OPTIONAL MATCH subquery that counts ConversationLog nodes per spec.

The current Go code concatenates the query as string segments: `MATCH...` + optional `WHERE...` + `RETURN...`. The change requires inserting a `WITH s` + `OPTIONAL MATCH` + `WITH s, count(cl)` block between the WHERE/MATCH and RETURN segments, and updating the RETURN to reference `conversation_count` from the WITH binding instead of the literal `0`.

The query must be Memgraph-compatible. Known constraints:

- Memgraph does not support `count(DISTINCT ...)` — use `count(cl)` instead (CONTINUES is a chain, not a DAG, so no duplicates)
- Memgraph requires `WITH` bridging to maintain variable bindings across OPTIONAL MATCH + aggregation

Target Cypher pattern:

```cypher
MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec)
[WHERE clauses]
WITH s
OPTIONAL MATCH (s)-[:AUTHORED_VIA]->(:ConversationLog)-[:CONTINUES*0..]->(cl:ConversationLog)
WITH s, count(cl) AS conversation_count
RETURN s.id, s.slug, ..., conversation_count
ORDER BY s.created_at
```

### 4. Integration Test

Add a test in `internal/storage/memgraph/conversation_test.go` (where existing conversation storage tests live) that:

1. Creates a spec
2. Records 2-3 conversations via `RecordConversation`
3. Calls `ListSpecs` and asserts `ConversationCount` matches the expected count
4. Verifies the AUTHORED_VIA -> CONTINUES chain structure

This proves the Cypher subquery works against a real Memgraph instance and covers the data path from recording to display.

## File Changes

### Plugin Skills

- Create: `plugin/specgraph/skills/specgraph/conversation-recording.md`
- Create: symlinks in 5 skill `references/` directories → `../../specgraph/conversation-recording.md`
- Modify: `plugin/specgraph/skills/specgraph-spark/SKILL.md` — add recording step
- Modify: `plugin/specgraph/skills/specgraph-shape/SKILL.md` — add recording step
- Modify: `plugin/specgraph/skills/specgraph-specify/SKILL.md` — add recording step
- Modify: `plugin/specgraph/skills/specgraph-decompose/SKILL.md` — add recording step
- Modify: `plugin/specgraph/skills/specgraph-approve/SKILL.md` — add conditional recording step

### Backend (Go)

- Modify: `internal/storage/memgraph/memgraph.go` — replace `0 AS conversation_count` with real OPTIONAL MATCH subquery
- Modify: `internal/storage/memgraph/conversation_test.go` — add conversation count integration test

## Out of Scope

- Amend conversation recording (existing amend flows unchanged; `--amend` flag documented but not wired into amend skill paths)
- Conversation display changes (spec detail page already renders conversations)
- Conversation count in non-ListSpecs queries (GetSpec, lifecycle operations)
- Modifying the ConversationExchange proto schema
