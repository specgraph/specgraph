---
name: specgraph-spark
description: >
  Capture a vague idea and create a new spec in Spark stage. Use when the user
  has a problem, feature idea, or rough concept. Triggered by "I have an idea",
  "what if we...", "we need to...", or "new spec".
---

# SpecGraph Spark

Get the idea out of someone's head and into the graph before it evaporates.

---

## Persona

> **Read `references/persona.md` for the full shared persona** — core identity, posture system
> (Drive/Partner/Support with auto-detection), pushback protocol, tone calibration,
> judgment heuristics, and conversational style.

---

## Domain

### Elicitation Probes

Work through these conversationally — one at a time, in the posture-appropriate
style. Do not dump all probes at once.

1. **Seed** — "What's the idea? Don't overthink it — just describe what you want
   to exist that doesn't exist yet."
2. **Signal** — "Why now? What happened that made this urgent or relevant?"
3. **Scope sniff** — "Gut feel: is this hours, days, or weeks of work?" This is
   not a commitment, just calibration.
4. **Kill test** — "What would make this not worth doing? If you can't think of
   one, that's a yellow flag — everything has a kill condition." If the user is
   stuck, propose candidate kill conditions based on the seed.

### Quality Heuristics

- **Seed longer than 2 sentences:** Nudge toward Shape — "Sounds like you've
  already thought about scope and approach — want to jump straight to Shape?"
- **No signal provided:** Ask about urgency — "Is this something that needs to
  happen now, or is it a backlog idea?"
- **Can't articulate kill test:** Agent offers candidates based on the seed.

### Posture Behavior at Spark

- **Drive:** Agent proposes seed/signal/kill test based on input, asks for
  confirmation.
- **Partner:** Agent asks probes one at a time, discusses each before moving on.
- **Support:** Agent waits for user to describe the idea, reflects back, fills
  gaps when user seems done.

### Analytical Passes

No analytical passes at Spark — too early to analyze.

---

## Execution

### Prerequisites

1. Verify server is reachable:

```bash
specgraph health
```

2. Load and summarize the project constitution:

```bash
specgraph constitution show
```

Summarize to user: "Your project constitution has N principles and M
constraints. Key ones for this spec: [relevant subset]."

### Duplicate Check

Before creating a new spec, check for existing specs with the same or similar
slugs. This prevents accidental duplicates and catches near-misses.

```bash
specgraph list --json
```

1. **Exact match:** If a spec with the proposed slug already exists, do NOT
   create a new one. Instead, present the existing spec and ask:
   - "A spec with slug `<slug>` already exists at stage `<stage>`. Resume it,
     or pick a different slug?"

2. **Substring/prefix match:** If any existing slug contains the proposed slug
   as a substring (or vice versa), surface the matches:
   - "I found similar specs: `<match-1>` (stage), `<match-2>` (stage). Is your
     idea related to one of these, or is it a new spec?"

3. **No matches:** Proceed normally.

Use `AskUserQuestion` to present matches with options: "Resume existing",
"Create new spec anyway", or "Pick a different slug."

### Slug Handling

- If `$ARGUMENTS` contains a slug, load existing spec:

```bash
specgraph show <slug>
```

- If `$ARGUMENTS` is a description, generate a slug or ask the user for one.
  Run the **Duplicate Check** above before creating.

### Resumption

If the spec is already past spark stage:

1. Load via `specgraph show <slug>`.
2. Present the current state as a summary.
3. Offer to revise or continue to the next stage.

### Persistence

After completing elicitation:

1. Create the spec (if new) and run the spark command:

```bash
specgraph create <slug> --intent "<seed>"
specgraph spark <slug> --seed "<seed>"
```

2. Record the conversation (see `references/conversation-recording.md`):

   ```bash
   cat > /tmp/conv-<slug>.json << 'CONV_EOF'
   { "exchanges": [ ... accumulated probe/response exchanges ... ] }
   CONV_EOF
   specgraph conversation record <slug> --stage spark --json-file /tmp/conv-<slug>.json
   rm /tmp/conv-<slug>.json
   ```

3. Show the user what was saved.
4. Offer to continue: "Spark is saved. Want to continue to Shape? I can help
   scope the boundaries."
