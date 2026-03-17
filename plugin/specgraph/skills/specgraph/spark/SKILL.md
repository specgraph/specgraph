---
name: specgraph-spark
description: >
  Capture a vague idea and create a new spec in Spark stage. Use when the user
  has a problem, feature idea, or rough concept. Triggered by "I have an idea",
  "what if we...", "we need to...", or "new spec".
---

# SpecGraph Spark

Capture a vague idea and turn it into a seed spec.

## Prerequisites

Verify server is reachable:

```bash
specgraph health
```

## Workflow

### 1. Gather the Seed

If `$ARGUMENTS` contains a slug, load it:

```bash
specgraph show <slug> --format=json
```

If `$ARGUMENTS` is a description, ask the user for a slug (kebab-case) or generate one.

### 2. Create and Spark

```bash
specgraph create <slug> --title="<title>" --priority=p2 --size=medium
specgraph spark <slug>
```

### 3. Elicitation Probes

Work through these conversationally — one at a time:

1. **Seed** — What is the core idea? One sentence.
2. **Signal** — Why now? What triggered this?
3. **Scope Sniff** — Gut estimate: hours, days, weeks?
4. **Unknowns** — What could surprise us?
5. **Kill Test** — What would make this not worth doing?

### 4. Next Steps

- Continue to **Shape** → `/specgraph-shape <slug>`
- Or park it — the spec is saved at spark stage
