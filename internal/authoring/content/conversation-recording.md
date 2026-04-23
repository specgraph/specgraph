# Conversation Recording

## What to Capture

Each probe/response pair from the elicitation is one exchange pair. Include:

1. **Elicitation exchanges** — every question the agent asked and the user's answer
2. **Synthesis exchange** — the agent's final summary and the user's confirmation or rejection
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

## Accumulating Exchanges

As the agent conducts elicitation, track each probe/response pair with an incrementing sequence number. Carry the full accumulated list into the tool call that persists the stage.

## Persisting Exchanges

> Conversation exchanges are persisted atomically with the stage output at
> Shape, Specify, Decompose, and Approve transitions — pass the accumulated
> exchange list as part of the same persistence call that saves the stage
> output. No separate conversation-recording call is needed after a stage
> transition. The standalone conversation-record tool is reserved for
> post-hoc amendments to prior recordings.

Pass the complete list of exchange objects alongside the stage output on the same persistence call. The stage output and the conversation log are committed together — either both succeed or neither does.

## Amend Semantics

Omit the amend flag on first-pass recordings. Set `amend: true` (or the equivalent tool argument) when re-entering a stage via the amend flow — that is, when correcting previously persisted output rather than producing it fresh. Amended exchanges replace the prior recording for that stage; they do not append.

## Approve Special Case

Record conversation only on rejection (hold or decline). The approval flow's discussion carries decision-trail value when the outcome is negative. Clean approvals are self-evident from the approval call itself and do not require a separate conversation record. For approve-stage rejections, pass the rejection-reason exchanges alongside the rejection on the same persistence call — the coupling is atomic, same as the other stages.
