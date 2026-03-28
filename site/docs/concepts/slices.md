# Slices & Execution Units

When a spec reaches the [Decompose stage](authoring.md#decompose), SpecGraph
doesn't just record a JSON field — it creates **Slice nodes** in the graph. Each
slice is an independently claimable and completable unit of work with its own
intent, verify criteria, and touches list. Slices are the handoff point between
design (the spec graph) and execution (an agent or developer picking up
work).

---

## Graph Model

Each slice points back to its parent spec via a `COMPOSES` edge:

```text
(:Slice) -[:COMPOSES]-> (:Spec)
```

The same `COMPOSES` edge type is used for both spec-to-spec structural
composition and slice-to-spec execution composition:

```text
(:Spec)  -[:COMPOSES]-> (:Spec)  # structural — a spec broken into sub-specs
(:Slice) -[:COMPOSES]-> (:Spec)  # execution — implementable work items from decompose
```

In both cases, the child node composes into the parent. Slices also carry a
`BELONGS_TO` edge to the project for scoping. Unlike sub-specs, slices do not
go through the authoring funnel — they are claimed and done directly.

A slice carries:

| Field | Description |
|---|---|
| `slug` | Stable, human-readable identifier (e.g. `healthz-v1`) |
| `intent` | What this slice accomplishes |
| `verify` | Acceptance criteria specific to this slice |
| `touches` | Files, endpoints, or components this slice modifies |
| `status` | Current lifecycle state (`open`, `claimed`, `done`) |
| `assignee` | Who claimed this slice (set on claim) |

---

## Slice Lifecycle

Slices move through a three-state machine:

```text
open ──▶ claimed ──▶ done
```

| State | Meaning |
|---|---|
| `open` | Available for claiming — no one is working on it |
| `claimed` | An agent or developer has taken ownership |
| `done` | Work is done; verify criteria should be satisfied |

The parent spec's done-state depends on all of its slices completing. A spec
remains in-flight as long as any slice is `open` or `claimed`. When the last
slice transitions to `done`, the spec is eligible for the done transition.

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

Marks the slice as `done`. Once all slices on a spec are done, the
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
and connects it to the parent via a `COMPOSES` edge.

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

With all slices done, `healthz` is ready for its done transition.
