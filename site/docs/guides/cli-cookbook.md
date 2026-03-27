# CLI Cookbook

Practical recipes for common SpecGraph CLI workflows. Each recipe is self-contained — run the commands in order.

---

## 1. Author a spec end-to-end

**Goal:** Take a raw idea through all five authoring stages and approve it for execution.

```bash
# Step 1 — create the spark (seed primes the AI authoring skill)
specgraph spark auth-service --seed "JWT-based auth with refresh tokens"

# Step 2 — shape: defines problem statement, goals, non-goals
#   Run the specgraph-shape skill, save its JSON output, then commit it
specgraph shape auth-service --json-file shape-output.json

# Step 3 — specify: adds requirements, acceptance criteria, verify criteria
specgraph specify auth-service --json-file specify-output.json

# Step 4 — decompose: breaks the spec into slices with effort estimates
specgraph decompose auth-service --json-file decompose-output.json

# Step 5 — approve: transitions the spec to approved, ready for execution
specgraph approve auth-service
```

??? example "Expected output after `approve`"
    ```
    ✓ auth-service approved
      stage: approved
      slug:  auth-service
    ```

!!! tip
    Use `specgraph show auth-service` after each step to verify the stage
    advanced correctly before proceeding.

---

## 2. Query the dependency graph

**Goal:** Understand what a spec depends on, what would be affected by changes to it, and what work is unblocked right now.

```bash
# Direct dependencies of a spec
specgraph deps auth-service

# Transitive closure — all upstream dependencies recursively
specgraph deps auth-service --transitive

# Downstream impact — what specs depend on auth-service
specgraph impact auth-service

# Longest path to completion from this spec
specgraph critical-path auth-service

# All specs with no unresolved upstream dependencies (ready to start)
specgraph ready
```

??? example "Expected output — `deps auth-service`"
    ```
    Dependencies of auth-service:
      → user-schema     (approved)
      → token-storage   (in-progress)
    ```

??? example "Expected output — `impact auth-service`"
    ```
    Specs that depend on auth-service:
      ← api-gateway     (shape)
      ← mobile-client   (spark)
    ```

??? example "Expected output — `critical-path auth-service`"
    ```
    Critical path (3 specs, ~13 days):
      token-storage → auth-service → api-gateway
    ```

??? example "Expected output — `ready`"
    ```
    Ready to start (no unresolved deps):
      user-schema     p1  approved
      token-storage   p2  approved
    ```

---

## 3. Work with slices

**Goal:** Decompose a spec into parallel work units, claim one, track progress, and complete it.

```bash
# Decompose first (generates slices from the spec)
specgraph decompose auth-service --json-file decompose-output.json

# List all slices for a spec
specgraph slice list auth-service

# Claim a specific slice (--assignee is required)
specgraph slice claim auth-service.slice.1 --assignee alice

# Report progress on the claimed slice
specgraph report-progress auth-service.slice.1 --message "JWT signing logic done, writing tests"

# Mark the slice complete
specgraph slice complete auth-service.slice.1
```

??? example "Expected output — `slice list auth-service`"
    ```
    Slices for auth-service:

      SLUG                    TITLE                    STATUS    ASSIGNEE
      auth-service.slice.1    Implement JWT signing     open      —
      auth-service.slice.2    Implement refresh flow    open      —
      auth-service.slice.3    Write integration tests   open      —
    ```

??? example "Expected output — `slice claim auth-service.slice.1 --assignee alice`"
    ```
    ✓ auth-service.slice.1 claimed by alice
    ```

??? example "Expected output — `slice complete auth-service.slice.1`"
    ```
    ✓ auth-service.slice.1 marked complete
    ```

!!! note
    `--assignee` is required for `slice claim`. Omitting it returns an error.

---

## 4. Detect and resolve drift

**Goal:** Identify when an upstream spec has changed after a dependency was baselined, then acknowledge the drift with a note.

```bash
# Create two specs and link them
specgraph create token-storage --intent "Persistent token store"
specgraph create auth-service --intent "JWT auth"
specgraph add-edge auth-service --to token-storage --type depends-on

# Approve both and baseline the dependency
specgraph approve token-storage
specgraph approve auth-service

# (Later) token-storage gets updated — its content hash changes
specgraph update token-storage --intent "Persistent token store with TTL support"

# Detect drift — shows specs with stale dependency hashes
specgraph drift

# Narrow to a single spec
specgraph drift auth-service

# Scope to only dependency-related drift
specgraph drift auth-service --scope deps

# Acknowledge drift on a specific upstream (--note is required)
specgraph drift acknowledge auth-service --upstream token-storage --note "Reviewed TTL change; no impact on auth-service interface"

# Or acknowledge all drift at once
specgraph drift acknowledge auth-service --all --note "Batch baseline after token-storage refactor"
```

??? example "Expected output — `drift auth-service`"
    ```
    Drift detected in auth-service:

      UPSTREAM         TYPE   DETAIL
      token-storage    deps   content hash changed since baseline
    ```

??? example "Expected output — `drift acknowledge`"
    ```
    ✓ drift acknowledged for auth-service → token-storage
    ```

!!! warning
    `--note` is required for `drift acknowledge`. An empty note is rejected.

---

## 5. Lint before merging

**Goal:** Catch spec quality issues before a spec moves to review or approval.

```bash
# Lint all specs
specgraph lint

# Lint a single spec
specgraph lint auth-service
```

??? example "Example violation output"
    ```
    auth-service  [WARN]  missing verify criteria — add at least one verify item
    auth-service  [ERROR] circular dependency: auth-service → api-gateway → auth-service
    ```

Fix the issues — in this example, add verify criteria to the spec and remove the circular edge:

```bash
# Re-specify to add verify criteria
specgraph specify auth-service --json-file specify-output-v2.json

# Remove the circular edge
specgraph remove-edge auth-service --to api-gateway --type depends-on

# Re-run lint to confirm clean
specgraph lint auth-service
```

??? example "Expected output after fixes"
    ```
    ✓ auth-service — no issues found
    ```

---

## 6. Manage execution lifecycle

**Goal:** Claim a spec for execution, report status updates, signal blockers, and record completion.

```bash
# Claim the spec for execution (--assignee required, --ttl sets lease duration)
specgraph claim auth-service --assignee alice --ttl 30m

# Send a progress update (visible in spec timeline)
specgraph report-progress auth-service --message "JWT signing done, refresh flow in progress"

# Signal a blocker (pauses SLA clock, notifies dependents)
specgraph report-blocker auth-service --message "Waiting on token-storage schema migration"

# Once the blocker is cleared, send another progress update
specgraph report-progress auth-service --message "Blocker resolved, resuming refresh flow"

# Record completion (transitions spec to done)
specgraph report-completion auth-service
```

??? example "Expected output — `claim`"
    ```
    ✓ auth-service claimed by alice
      lease expires: 2026-03-27T14:30:00Z
    ```

??? example "Expected output — `report-completion`"
    ```
    ✓ auth-service marked complete
      stage: done
    ```

!!! tip
    The `--ttl` flag accepts Go duration strings: `30m`, `2h`, `24h`. Leases
    auto-expire if not renewed, returning the spec to unclaimed state.

---

## 7. Generate an execution bundle

**Goal:** Produce a self-contained context package that an agent (or human) needs to implement a spec.

```bash
# Generate the bundle (human-readable table output)
specgraph bundle auth-service

# Generate as JSON (useful for piping to an agent or CI step)
specgraph bundle auth-service --json
```

The bundle output contains:

| Section | Contents |
|---------|----------|
| **Spec fields** | Slug, intent, stage, priority, requirements, acceptance criteria |
| **Verify criteria** | Conditions that must hold for the spec to be considered done |
| **Constitution context** | Applicable constitution layers (user → org → project → domain) |
| **Dependency context** | Direct and transitive upstream specs with their current stage |
| **Decisions** | Linked ADRs and their rationale |

??? example "Expected output — `bundle auth-service` (truncated)"
    ```
    ╔══════════════════════════════════════╗
    ║  Execution Bundle: auth-service      ║
    ╚══════════════════════════════════════╝

    SPEC
      slug:     auth-service
      intent:   JWT-based auth with refresh tokens
      stage:    approved
      priority: p1

    VERIFY CRITERIA
      • POST /auth/token returns signed JWT on valid credentials
      • Refresh token rotates on each use
      • Expired tokens return 401

    CONSTITUTION CONTEXT
      [project] all auth endpoints must use HTTPS
      [domain]  token TTL must not exceed 24h

    DEPENDENCIES
      token-storage   approved   Persistent token store with TTL support
      user-schema     approved   User identity schema

    DECISIONS
      adr-007   Use RS256 over HS256 for token signing
    ```
