# Slices & Execution Units

When a spec reaches the [Decompose stage](authoring.md#decompose), SpecGraph
doesn't just record a JSON field — it creates **Slice nodes** in the graph. Each
slice is an independently claimable and completable unit of work with its own
intent, verify criteria, and touches list. Slices are the handoff point between
design (the spec graph) and execution (an agent or developer picking up
work).

---

## Graph Model

Slices attach to their parent spec via a `HAS_SLICE` edge:

```text
(:Spec) -[:HAS_SLICE]-> (:Slice)
```

This is distinct from `COMPOSES`, which connects parent specs to child specs:

```text
(:Spec) -[:COMPOSES]-> (:Spec)   # structural composition — a spec broken into sub-specs
(:Spec) -[:HAS_SLICE]-> (:Slice) # execution units — a spec broken into implementable slices
```

`COMPOSES` represents **structural decomposition** of a large spec into smaller
specs that each go through the full authoring funnel. `HAS_SLICE` represents
**execution decomposition** of an approved spec into parallel work items that can
be claimed and completed independently.

A slice carries:

| Field | Description |
|---|---|
| `slug` | Stable, human-readable identifier (e.g. `healthz-v1`) |
| `intent` | What this slice accomplishes |
| `verify` | Acceptance criteria specific to this slice |
| `touches` | Files, endpoints, or components this slice modifies |
| `status` | Current lifecycle state (`open`, `claimed`, `completed`) |
| `assignee` | Who claimed this slice (set on claim) |

---

## Slice Lifecycle

Slices move through a three-state machine:

```text
open ──▶ claimed ──▶ completed
```

| State | Meaning |
|---|---|
| `open` | Available for claiming — no one is working on it |
| `claimed` | An agent or developer has taken ownership |
| `completed` | Work is done; verify criteria should be satisfied |

The parent spec's done-state depends on all of its slices completing. A spec
remains in-flight as long as any slice is `open` or `claimed`. When the last
slice transitions to `completed`, the spec is eligible for the done transition.

---

## CLI Usage

### List slices for a spec

```bash
specgraph slice list <parent-slug> [--json]
```

Shows all slices attached to the given spec. Use `--json` for machine-readable
output.

### Show slice details

```bash
specgraph slice get <slug> [--json]
```

Shows full slice detail including intent, verify criteria, touches list, and
current status.

### Claim a slice

```bash
specgraph slice claim <slug> --assignee <name>
```

Marks the slice as `claimed` and records the assignee. `--assignee` is
required — every claim must have an owner.

### Complete a slice

```bash
specgraph slice complete <slug>
```

Marks the slice as `completed`. Once all slices on a spec are completed, the
spec becomes eligible for its done transition.

---

## Worked Example

Starting from the `healthz` spec in the [quickstart](../quickstart.md).
Decompose produces a single slice because the spec is small enough to implement
atomically:

```bash
specgraph decompose healthz --json-file decompose-output.json
```

The decompose output includes one slice — `healthz-v1`: implement the handler,
register the route, add the test. SpecGraph creates a `Slice` node in the graph
and attaches it with a `HAS_SLICE` edge.

List the slices to confirm:

```bash
specgraph slice list healthz
```

```text
SLUG          STATUS    ASSIGNEE    INTENT
healthz-v1    open      —           Implement handler, register route, add test
```

Claim the slice before starting work:

```bash
specgraph slice claim healthz-v1 --assignee alice
```

Mark it done when the implementation is complete:

```bash
specgraph slice complete healthz-v1
```

With all slices completed, `healthz` is ready for its done transition.
