---
name: specgraph-shape
description: >
  Shape a spec by bounding scope, exploring solutions, and surfacing risks.
  Use when the user wants to design or plan. Triggered by "let's design...",
  "scope this out", "what's the approach for...", or "shape".
---

# SpecGraph Shape

Bound scope, explore solutions, capture decisions, surface risks.

## Prerequisites

```bash
specgraph health
specgraph constitution show --format=json
```

## Workflow

### 1. Load the Spec

```bash
specgraph show <slug> --format=json
```

### 2. Shaping Moves

Work through all five — each is a conversation:

1. **Bound the Scope** — What's in? What's explicitly out?
2. **Explore Solutions** — 2-3 approaches with tradeoffs. Decide.
3. **Identify Edges** — Dependencies, blockers, related specs.
4. **Surface Risks** — Technical, operational, business. Be specific.
5. **Define Success** — Must-have, should-have, won't-have.

### 3. Capture Decisions

```bash
specgraph decision create --spec=<slug> --title="<title>" --chosen="<chosen>" --rationale="<rationale>"
```

### 4. Capture Edges

```bash
specgraph edge <slug> depends-on <dep-slug>
```

### 5. Persist

```bash
specgraph shape <slug>
```

### 6. Next Steps

- Continue to **Specify** → `/specgraph-specify <slug>`
