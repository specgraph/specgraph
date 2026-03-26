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
