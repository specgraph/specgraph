# Stage: Spark

Get the idea out of someone's head and into the graph before it evaporates.

Spark is the first stage of the authoring funnel. Its job is minimal: capture
enough of an idea to make it findable and resumable. No scope, no design — just
the raw seed, the signal behind it, and a gut-feel size check.

---

## What Spark Captures

A Spark record answers four questions:

1. **What is the idea?** — The seed: a plain-language description of what should
   exist that doesn't exist yet.
2. **Why now?** — The signal: what happened that made this idea urgent or
   relevant at this moment.
3. **How big, roughly?** — The scope sniff: hours, days, or weeks — not a
   commitment, just calibration.
4. **What would kill it?** — The kill test: the condition under which this idea
   is not worth pursuing.

---

## Elicitation Probes

Work through these conversationally — one at a time. Do not dump all probes at
once. The goal is a conversation, not a form.

1. **Seed** — "What's the idea? Don't overthink it — just describe what you want
   to exist that doesn't exist yet."
2. **Signal** — "Why now? What happened that made this urgent or relevant?"
3. **Scope sniff** — "Gut feel: is this hours, days, or weeks of work?" This is
   not a commitment, just calibration.
4. **Kill test** — "What would make this not worth doing? If you can't think of
   one, that's a yellow flag — everything has a kill condition." If the user is
   stuck, propose candidate kill conditions based on the seed.

---

## Quality Signals

- **Seed longer than two sentences:** The user has probably already done scoping
  work. Nudge toward Shape — "Sounds like you've already thought about scope
  and approach — want to jump straight to Shape?"
- **No signal provided:** Ask about urgency — "Is this something that needs to
  happen now, or is it a backlog idea?"
- **Can't articulate a kill test:** Propose candidates based on the seed rather
  than leaving the field blank.

---

## Duplicate Check

Before persisting a new spec, list existing specs and check for conflicts:

- **Exact slug match:** Do not create a new spec. Present the existing one and
  ask whether to resume it or choose a different slug.
- **Substring / prefix match:** Surface the near-matches and ask whether the
  idea is related to an existing spec or genuinely new.
- **No matches:** Proceed normally.

The check is tool-neutral — use whatever means surfaces the existing spec list
in the current context.

---

## Persistence Contract

When elicitation is complete, persist the Spark output with the `author` tool
(`action: spark`). The `output` argument is **friendly snake_case YAML** — the
same shape you show the user, no translation step. Use these keys verbatim; do
NOT camelCase them (`scopeSniff`, `killTest` are rejected by the parser):

```yaml
seed: "one-line idea or problem statement"
signal: "why this matters now"
questions:
  - "clarifying question to sharpen scope"
scope_sniff: small        # tiny | small | medium | large | epic
kill_test: "condition that would make this not worth pursuing"
```

The required fields are `seed`, `signal`, `scope_sniff`, and `kill_test`.

If you recorded the elicitation conversation, pass the accumulated `exchanges`
alongside the `output` on the same `author` call — exchanges commit atomically
with the stage output. `exchanges` is a **JSON array** and is OPTIONAL for
spark (a seed-only spark with no back-and-forth is valid without it):

```json
[
  { "role": "probe",    "content": "How big, roughly?", "stage": "spark", "sequence": 1 },
  { "role": "response", "content": "A few days.",       "stage": "spark", "sequence": 2 }
]
```

After persisting, show the user what was saved and offer to continue to Shape:
"Spark is saved. Want to continue to Shape? I can help scope the boundaries."

---

## Next Stage

Shape — bounds the idea into a proposal with explicit tradeoffs, approaches,
and success criteria.
