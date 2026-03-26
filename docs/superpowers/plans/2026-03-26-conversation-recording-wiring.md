# Conversation Recording Wiring Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the 5 authoring skills (spark, shape, specify, decompose, approve) to call the existing RecordConversation RPC, and enable the real conversation_count Cypher query in ListSpecs.

**Architecture:** Create a shared reference doc for the conversation recording pattern, symlink it into each skill, add a recording step to each skill's Persistence section. On the backend, replace the `0 AS conversation_count` placeholder with a real OPTIONAL MATCH subquery and prove it with an integration test.

**Tech Stack:** Markdown (skill files), Go (Memgraph Cypher query), Memgraph (graph DB)

**Spec:** `docs/superpowers/specs/2026-03-26-conversation-recording-wiring-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `plugin/specgraph/skills/specgraph/conversation-recording.md` | Create | Shared reference: exchange format, accumulation pattern, CLI invocation |
| `plugin/specgraph/skills/specgraph-spark/references/conversation-recording.md` | Create (symlink) | Points to `../../specgraph/conversation-recording.md` |
| `plugin/specgraph/skills/specgraph-shape/references/conversation-recording.md` | Create (symlink) | Same symlink target |
| `plugin/specgraph/skills/specgraph-specify/references/conversation-recording.md` | Create (symlink) | Same symlink target |
| `plugin/specgraph/skills/specgraph-decompose/references/conversation-recording.md` | Create (symlink) | Same symlink target |
| `plugin/specgraph/skills/specgraph-approve/references/conversation-recording.md` | Create (symlink) | Same symlink target |
| `plugin/specgraph/skills/specgraph-spark/SKILL.md` | Modify | Add recording step after line 132 |
| `plugin/specgraph/skills/specgraph-shape/SKILL.md` | Modify | Add recording step after line 249 |
| `plugin/specgraph/skills/specgraph-specify/SKILL.md` | Modify | Add recording step after line 229 |
| `plugin/specgraph/skills/specgraph-decompose/SKILL.md` | Modify | Add recording step after line 127 |
| `plugin/specgraph/skills/specgraph-approve/SKILL.md` | Modify | Add recording step after line 164 |
| `internal/storage/memgraph/memgraph.go` | Modify | Replace `0 AS conversation_count` with real Cypher subquery (lines 369-377) |
| `internal/storage/memgraph/conversation_test.go` | Modify | Add `TestListSpecs_ConversationCount` integration test |

---

## Chunk 1: Shared Reference + Symlinks + Skill Modifications

### Task 1: Create shared conversation-recording.md reference

**Files:**

- Create: `plugin/specgraph/skills/specgraph/conversation-recording.md`

- [ ] **Step 1: Create the shared reference file**

````markdown
<!-- conversation-recording.md — Shared conversation recording reference for SpecGraph authoring skills.

     This is a SOURCE OF TRUTH document, NOT a skill file.
     It has no YAML front matter and is not loaded by the skill runner.
     Each authoring skill symlinks to it from references/conversation-recording.md.
-->

# Recording Conversations

After completing elicitation and receiving user confirmation on the synthesized output, record the conversation exchanges to the graph. This happens alongside (after) the `specgraph <stage>` persistence command.

## What to Capture

Each probe/response pair from the elicitation is one exchange pair. Include:

1. **Elicitation exchanges** — every question you asked and the user's answer
2. **Synthesis exchange** — your final summary and the user's confirmation or rejection
3. **Decision points** — flag any exchange where the user chose between alternatives (`decision_point: true`)

Exclude meta-conversation (greetings, clarifications about the tool itself, status messages).

## Exchange Format

Each exchange is a JSON object:

```json
{
  "role": "probe",
  "content": "What's the idea? Don't overthink it.",
  "stage": "spark",
  "sequence": 1,
  "decision_point": false
}
```

- `role`: `"probe"` (agent asks) or `"response"` (user answers)
- `content`: The substantive text of the exchange
- `stage`: The authoring stage (`spark`, `shape`, `specify`, `decompose`, `approve`)
- `sequence`: Pairs probes with responses — same sequence number = same Q&A pair
- `decision_point`: `true` if the user made a judgment call between alternatives

## How to Accumulate

As you conduct the elicitation, mentally track each probe/response pair with an incrementing sequence number. At persistence time, write the full list to a JSON temp file.

## CLI Invocation

After persisting the structured stage output, record the conversation:

```bash
CONV_TMP="$(mktemp /tmp/conv-XXXXXX.json)"
trap 'rm -f "$CONV_TMP"' EXIT
cat > "$CONV_TMP" << 'CONV_EOF'
{
  "exchanges": [
    {"role": "probe", "content": "...", "stage": "<stage>", "sequence": 1},
    {"role": "response", "content": "...", "stage": "<stage>", "sequence": 1, "decision_point": false},
    ...
  ]
}
CONV_EOF
specgraph conversation record "<slug>" --stage "<stage>" --json-file "$CONV_TMP"
```

Replace `<slug>` and `<stage>` with the actual spec slug and stage name.

## Amend Flag

Omit `--amend` for first-pass recordings. Use `--amend` when re-entering a stage via the amend flow (correcting previous output, not producing it fresh):

```bash
specgraph conversation record "<slug>" --stage "<stage>" --json-file "$CONV_TMP" --amend
```

## Approve Special Case

Only record on rejection (hold/decline). The approval flow's discussion is worth capturing when the outcome is negative — that's the decision trail with value. Clean approvals are self-evident from the `specgraph approve` call itself.

## Error Handling

Conversation recording is non-critical. If the CLI call fails, log the error but do NOT abort the stage persistence. The structured stage output is the primary artifact.
````

- [ ] **Step 2: Verify file is in the right location**

```bash
ls -la plugin/specgraph/skills/specgraph/conversation-recording.md
ls -la plugin/specgraph/skills/specgraph/persona.md
```

Expected: Both files side by side in the same directory.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "docs(skills): add shared conversation-recording reference (spgr-cdd)"
```

### Task 2: Create symlinks in all 5 skill references directories

**Files:**

- Create: symlinks in `specgraph-{spark,shape,specify,decompose,approve}/references/`

- [ ] **Step 1: Create all 5 symlinks**

```bash
for skill in spark shape specify decompose approve; do
  ln -s ../../specgraph/conversation-recording.md \
    plugin/specgraph/skills/specgraph-$skill/references/conversation-recording.md
done
```

- [ ] **Step 2: Verify symlinks resolve**

```bash
for skill in spark shape specify decompose approve; do
  echo "$skill: $(readlink plugin/specgraph/skills/specgraph-$skill/references/conversation-recording.md)"
  head -1 plugin/specgraph/skills/specgraph-$skill/references/conversation-recording.md
done
```

Expected: All 5 print `../../specgraph/conversation-recording.md` and the first line of the file.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "feat(skills): symlink conversation-recording reference into authoring skills (spgr-cdd)"
```

### Task 3: Add recording step to specgraph-spark

**Files:**

- Modify: `plugin/specgraph/skills/specgraph-spark/SKILL.md:132`

- [ ] **Step 1: Add recording step after the spark persistence commands**

After the existing step 1 block (lines 127-132, ending with `specgraph spark <slug> --seed "<seed>"`), and before step 2 ("Show the user what was saved"), insert:

````markdown

2. Record the conversation (see `references/conversation-recording.md`):

```bash
CONV_TMP="$(mktemp /tmp/conv-XXXXXX.json)"
trap 'rm -f "$CONV_TMP"' EXIT
cat > "$CONV_TMP" << 'CONV_EOF'
{ "exchanges": [ ... accumulated probe/response exchanges ... ] }
CONV_EOF
specgraph conversation record "<slug>" --stage spark --json-file "$CONV_TMP"
```

````

Renumber existing steps 2-3 to 3-4.

- [ ] **Step 2: Verify markdown renders correctly**

Read the file and confirm step numbering is sequential and no markdown fences are broken.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "feat(skills): add conversation recording to spark skill (spgr-cdd)"
```

### Task 4: Add recording step to specgraph-shape

**Files:**

- Modify: `plugin/specgraph/skills/specgraph-shape/SKILL.md:249`

- [ ] **Step 1: Add recording step after shape persistence**

After existing step 4 (lines 238-249, ending with `specgraph shape <slug> --json-file /tmp/shape-<slug>.json`), and before step 5 ("Confirm"), insert:

````markdown

5. **Record the conversation:** See `references/conversation-recording.md` for the exchange format.

   ```bash
   CONV_TMP="$(mktemp /tmp/conv-XXXXXX.json)"
   trap 'rm -f "$CONV_TMP"' EXIT
   cat > "$CONV_TMP" << 'CONV_EOF'
   { "exchanges": [ ... accumulated probe/response exchanges ... ] }
   CONV_EOF
   specgraph conversation record "<slug>" --stage shape --json-file "$CONV_TMP"
   ```

````

Renumber existing step 5 ("Confirm") to step 6.

- [ ] **Step 2: Verify markdown**

Read file, confirm step numbering.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "feat(skills): add conversation recording to shape skill (spgr-cdd)"
```

### Task 5: Add recording step to specgraph-specify

**Files:**

- Modify: `plugin/specgraph/skills/specgraph-specify/SKILL.md:229`

- [ ] **Step 1: Add recording step after specify persistence**

After existing step 4 (lines 221-229, ending with `specgraph specify <slug> --json-file /tmp/specify-<slug>.json`), and before step 5 ("Confirm"), insert:

````markdown

5. **Record the conversation:** See `references/conversation-recording.md` for the exchange format.

   ```bash
   CONV_TMP="$(mktemp /tmp/conv-XXXXXX.json)"
   trap 'rm -f "$CONV_TMP"' EXIT
   cat > "$CONV_TMP" << 'CONV_EOF'
   { "exchanges": [ ... accumulated probe/response exchanges ... ] }
   CONV_EOF
   specgraph conversation record "<slug>" --stage specify --json-file "$CONV_TMP"
   ```

````

Renumber existing step 5 ("Confirm") to step 6.

- [ ] **Step 2: Verify markdown**

Read file, confirm step numbering.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "feat(skills): add conversation recording to specify skill (spgr-cdd)"
```

### Task 6: Add recording step to specgraph-decompose

**Files:**

- Modify: `plugin/specgraph/skills/specgraph-decompose/SKILL.md:127`

- [ ] **Step 1: Add recording step after decompose persistence**

After existing step 4 (lines 123-127, ending with `specgraph decompose <slug> --json-file <tmpfile>`), and before step 5 ("Confirm"), insert:

````markdown

5. **Record the conversation:** See `references/conversation-recording.md` for the exchange format.

   ```bash
   CONV_TMP="$(mktemp /tmp/conv-XXXXXX.json)"
   trap 'rm -f "$CONV_TMP"' EXIT
   cat > "$CONV_TMP" << 'CONV_EOF'
   { "exchanges": [ ... accumulated probe/response exchanges ... ] }
   CONV_EOF
   specgraph conversation record "<slug>" --stage decompose --json-file "$CONV_TMP"
   ```

````

Renumber existing step 5 ("Confirm") to step 6.

- [ ] **Step 2: Verify markdown**

Read file, confirm step numbering.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "feat(skills): add conversation recording to decompose skill (spgr-cdd)"
```

### Task 7: Add conditional recording step to specgraph-approve

**Files:**

- Modify: `plugin/specgraph/skills/specgraph-approve/SKILL.md:163-164`

- [ ] **Step 1: Add recording step to rejection handling**

The current rejection block (line 163-164) reads:

```markdown
**If the human declines or wants changes:** Record the hold reason, suggest
which stage to revisit, and do NOT re-offer approval.
```

Replace with:

````markdown
**If the human declines or wants changes:**

1. Record the conversation (see `references/conversation-recording.md`). Only
   record on hold/decline — approvals are self-evident.

   ```bash
   CONV_TMP="$(mktemp /tmp/conv-XXXXXX.json)"
   trap 'rm -f "$CONV_TMP"' EXIT
   cat > "$CONV_TMP" << 'CONV_EOF'
   { "exchanges": [ ... review discussion + rejection rationale ... ] }
   CONV_EOF
   specgraph conversation record "<slug>" --stage approve --json-file "$CONV_TMP"
   ```

2. Record the hold reason, suggest which stage to revisit, and do NOT re-offer
   approval.
````

- [ ] **Step 2: Verify markdown**

Read file, confirm the rejection block is properly formatted.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "feat(skills): add conditional conversation recording to approve skill (spgr-cdd)"
```

---

## Chunk 2: Backend — Enable Real Conversation Count + Integration Test

### Task 8: Replace `0 AS conversation_count` with real Cypher subquery

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go:369-377`

- [ ] **Step 1: Update the ListSpecs query**

In `internal/storage/memgraph/memgraph.go`, the ListSpecs function (line 365) builds a query string. Replace the current block (lines 369-377):

```go
	// TODO(spgr-cdd): Replace literal 0 with OPTIONAL MATCH conversation count
	// once ConversationLog nodes exist. Memgraph requires WITH bridging for
	// OPTIONAL MATCH aggregation (see Memgraph variable scoping rules).
	query += ` RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.notes, s.content_hash,
		       s.spark_output, s.shape_output, s.specify_output, s.decompose_output,
		       0 AS conversation_count`
```

With:

```go
	query += ` WITH s
		OPTIONAL MATCH (s)-[:AUTHORED_VIA]->(:ConversationLog)-[:CONTINUES*0..]->(cl:ConversationLog)
		WITH s, count(cl) AS conversation_count
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.notes, s.content_hash,
		       s.spark_output, s.shape_output, s.specify_output, s.decompose_output,
		       conversation_count`
```

- [ ] **Step 2: Verify Go build**

```bash
go build ./internal/storage/... ./internal/server/...
```

Expected: Clean build.

- [ ] **Step 3: Run existing unit test**

```bash
go test ./internal/storage/memgraph/ -run TestRecordToSpecOffset -v -count=1
```

Expected: PASS. The unit test with 18-value records skips the conversation_count column via bounds check. The unit test with 19-value records (added in spgr-5re) has `Keys` containing `"conversation_count"` and should still pass.

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe -m "feat(storage): enable real conversation_count Cypher subquery in ListSpecs (spgr-cdd)"
```

### Task 9: Verify existing integration test for conversation count in ListSpecs

**Files:**

- Verify: `internal/storage/memgraph/conversation_test.go` (already contains `TestListSpecs_ConversationCount`)

- [ ] **Step 1: Verify the existing test**

`TestListSpecs_ConversationCount` already exists in `conversation_test.go`. Verify it covers the expected behavior: creates a spec, records 2 conversations with stage transitions, asserts `ConversationCount` increments correctly, and verifies the AUTHORED_VIA → CONTINUES chain via `ListConversations`. If the Cypher query changed (e.g., added `WHERE s IS NOT NULL`), confirm the test still passes.

```go
func TestListSpecs_ConversationCount(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	slug := "conv-count-test"

	// Create a spec.
	_, err = store.CreateSpec(ctx, slug, "Test conversation count", "p2", "medium")
	require.NoError(t, err)

	// Before any conversations, count should be 0.
	specs, err := store.ListSpecs(ctx, "", "", 0)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, 0, specs[0].ConversationCount)

	// Record first conversation (spark stage).
	_, err = store.RecordConversation(ctx, slug, storage.ConversationLogEntry{
		Stage: storage.SpecStageSpark,
		Exchanges: []storage.ConversationExchange{
			{Role: "probe", Content: "What's the idea?", Stage: "spark", Sequence: 1},
			{Role: "response", Content: "A caching layer", Stage: "spark", Sequence: 1},
		},
		ExchangeCount: 2,
	})
	require.NoError(t, err)

	// Count should be 1 (one ConversationLog node via AUTHORED_VIA).
	specs, err = store.ListSpecs(ctx, "", "", 0)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, 1, specs[0].ConversationCount)

	// Transition to shape, then record second conversation (CONTINUES chain).
	err = store.TransitionStage(ctx, slug, "spark", "shape")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, slug, storage.ConversationLogEntry{
		Stage: storage.SpecStageShape,
		Exchanges: []storage.ConversationExchange{
			{Role: "probe", Content: "What's in scope?", Stage: "shape", Sequence: 1},
			{Role: "response", Content: "Just the API layer", Stage: "shape", Sequence: 1},
		},
		ExchangeCount: 2,
	})
	require.NoError(t, err)

	// Count should be 2 (AUTHORED_VIA → CL1 → CONTINUES → CL2).
	specs, err = store.ListSpecs(ctx, "", "", 0)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, 2, specs[0].ConversationCount)

	// Also verify via ListConversations that the chain is intact.
	entries, err := store.ListConversations(ctx, slug, "")
	require.NoError(t, err)
	require.Len(t, entries, 2, "AUTHORED_VIA → CONTINUES chain should have 2 entries")
	assert.Equal(t, storage.SpecStageSpark, entries[0].Stage)
	assert.Equal(t, storage.SpecStageShape, entries[1].Stage)
}
```

- [ ] **Step 2: Run the integration test** (requires Docker)

```bash
go test -tags integration ./internal/storage/memgraph/ -run TestListSpecs_ConversationCount -v -count=1
```

Expected: PASS. This proves the Cypher subquery works against a real Memgraph instance.

- [ ] **Step 3: Run full test suite to check for regressions**

```bash
go test -tags integration ./internal/storage/memgraph/ -v -count=1
```

Expected: All tests pass.

- [ ] **Step 4: Run task check**

```bash
task check
```

Expected: All checks pass.

- [ ] **Step 5: Commit**

```bash
jj --no-pager describe -m "test(storage): add integration test for conversation_count in ListSpecs (spgr-cdd)"
```

---

## Chunk 3: Verify + PR

### Task 10: Final verification and PR

- [ ] **Step 1: Run full task check**

```bash
task check
```

Expected: All Go checks pass.

- [ ] **Step 2: Verify symlinks**

```bash
for skill in spark shape specify decompose approve; do
  head -3 plugin/specgraph/skills/specgraph-$skill/references/conversation-recording.md
done
```

Expected: All 5 print the first 3 lines of the reference file.

- [ ] **Step 3: Squash into single change**

```bash
jj --no-pager squash --from <first-change> --into <last-change> -m "feat(skills): wire authoring skills to call RecordConversation + enable conversation_count query (spgr-cdd)"
```

Or squash incrementally as each task completes.

- [ ] **Step 4: Create bookmark and push**

```bash
jj --no-pager bookmark set feat/conversation-recording -r @
jj --no-pager git push --bookmark feat/conversation-recording
```

- [ ] **Step 5: Create PR**

```bash
gh pr create \
  --title "feat(skills): wire authoring skills to RecordConversation + enable conversation_count (spgr-cdd)" \
  --body "$(cat <<'EOF'
## Summary
- **Shared reference**: `conversation-recording.md` documents exchange format, accumulation pattern, CLI invocation, and approve special case
- **5 skill modifications**: spark, shape, specify, decompose add recording step in Persistence; approve adds conditional recording on rejection
- **Backend**: Replace `0 AS conversation_count` placeholder with real OPTIONAL MATCH Cypher subquery in ListSpecs
- **Integration test**: Proves conversation count query works against Memgraph with AUTHORED_VIA → CONTINUES chain

Dashboard conversation count column (from spgr-5re) will show real data once skills are used.

## Test plan
- [ ] Integration test `TestListSpecs_ConversationCount` passes against Memgraph
- [ ] All existing conversation tests pass (no regressions)
- [ ] `task check` passes
- [ ] All 5 symlinks resolve correctly
- [ ] Skill SKILL.md files have correct step numbering

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```
