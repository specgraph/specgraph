---
name: specgraph-specify
description: >
  Define interface contracts, verification criteria, and invariants.
  Use when ready to define precise technical shape. Triggered by
  "define the interface", "acceptance criteria", "specify", "write the contract".
---

# SpecGraph Specify

Make the spec precise enough for implementation without making decisions.

## Prerequisites

```bash
specgraph health
specgraph show <slug> --format=json
specgraph deps <slug>
```

## Workflow

### 1. Define Interface Contract

- Function signatures, API endpoints, data structures
- Input/output types with constraints

### 2. Verification Criteria

- What must be true for this spec to be "done"?
- Automated test descriptions

### 3. Invariants

- What must never be violated?
- Edge cases and error conditions

### 4. Persist

```bash
specgraph specify <slug>
```

### 5. Next Steps

- Continue to **Decompose** → `/specgraph-decompose <slug>`
