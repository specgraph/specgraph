# SpecGraph Design and Implementation Plans

This directory holds the running record of significant design decisions and
implementation plans for SpecGraph. Plans pair `*-design.md` (the why and
what) with `*-plan.md` (the TDD-style how). Older plans pre-date this
convention and are single documents.

Status legend:

- **Active** — current design or in-flight implementation; describes how the
  code actually works today, or describes work being done now.
- **Shipped** — completed work whose design still accurately describes the
  final state. Useful as architecture reference.
- **Superseded** — explicitly replaced by a newer design. Marked with a
  status banner in the document itself.
- **Historical** — early-project plans that pre-date current conventions.
  Not formally superseded but should not be treated as authoritative for
  current behavior.

## Active

| Date | Document | Summary |
|------|----------|---------|
| 2026-05-06 | [harness-parity-epic-design](2026-05-06-harness-parity-epic-design.md) / [plan](2026-05-06-harness-parity-epic-plan.md) | Consolidate Claude / Cursor / OpenCode integration with shared in-tree skills, per-harness shims, and post-stage automation parity. Tracked as `spgr-cceg`. |
| 2026-05-04 | [spgr-7htb-init-idempotent-mcp-configs-design](2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md) / [plan](2026-05-04-spgr-7htb-init-idempotent-mcp-configs-plan.md) | Idempotent `specgraph init` writing per-harness MCP configs via JSON Merge Patch. Shipped as `spgr-7htb`. |
| 2026-04-27 | [task-32-read-mcp-resource-design](2026-04-27-task-32-read-mcp-resource-design.md) / [plan](2026-04-27-task-32-read-mcp-resource-plan.md) | `specgraph read-mcp-resource` CLI subcommand for the Claude session-start hook. |
| 2026-04-22 | [cli-lifecycle-split-design](2026-04-22-cli-lifecycle-split-design.md) / [plan](2026-04-22-cli-lifecycle-split-plan.md) | Split `up`/`down` lifecycle verbs from `install`/`uninstall`; retire `--rm`; gate `down --purge`. |
| 2026-04-20 | [multi-platform-plugin-design](2026-04-20-multi-platform-plugin-design.md) / [plan](2026-04-20-multi-platform-plugin-plan.md) | Phase A: thin per-platform plugins backed by server-embedded content; `specgraph://prime`; profile mapping. |
| 2026-04-10 | [mcp-server-design](2026-04-10-mcp-server-design.md) / [plan](2026-04-10-mcp-server-plan.md) | MCP server foundation: thin adapter over ConnectRPC; tools/resources/prompts; profile tiers. |
| 2026-03-18 | [auth-interceptor-design](2026-03-18-auth-interceptor-design.md) / [plan](2026-03-18-auth-interceptor-plan.md) | Bearer-token auth via ConnectRPC interceptor. |
| 2026-03-07 | [domain-types-consistency-design](2026-03-07-domain-types-consistency-design.md) | Storage interfaces use domain types, not protobuf. |

## Shipped (architecture reference)

| Date | Document | Summary |
|------|----------|---------|
| 2026-03-17 | [full-pipeline-e2e-design](2026-03-17-full-pipeline-e2e-design.md) / [plan](2026-03-17-full-pipeline-e2e-plan.md) | End-to-end test system using Ginkgo/Gomega. |
| 2026-03-06 | [storage-domain-types-design](2026-03-06-storage-domain-types-design.md) / [plan](2026-03-06-storage-domain-types-plan.md) | First cut of domain types in `internal/storage/`. |
| 2026-03-05 | [e2e-test-system-design](2026-03-05-e2e-test-system-design.md) / [plan](2026-03-05-e2e-test-system-plan.md) | Initial e2e test infrastructure. |
| 2026-02-28 | [client-server-architecture-design](2026-02-28-client-server-architecture-design.md) | Client/server split, transport choices. |

## Superseded

| Date | Document | Superseded by | Why |
|------|----------|---------------|-----|
| 2026-03-17 | [skill-personas-design](2026-03-17-skill-personas-design.md) / [plan](2026-03-17-skill-personas-plan.md) | 2026-04-20 multi-platform-plugin-design and 2026-05-06 harness-parity-epic-design | Persona layering moved to `internal/authoring/content/persona.md` and embedded composer; per-stage SKILL.md now lives in shared `skills/`. |
| 2026-03-16 | [slice-7-global-daemon-and-plugin-design](2026-03-16-slice-7-global-daemon-and-plugin-design.md) / [plan](2026-03-16-slice-7-global-daemon-and-plugin-plan.md) | 2026-04-20 multi-platform-plugin-design and 2026-05-06 harness-parity-epic-design | Plugin became thin (server-embedded content); `up/down` CLI surface revised in cli-lifecycle-split. |
| 2026-02-28 | [slice-7-claude-code-plugin-plan](2026-02-28-slice-7-claude-code-plugin-plan.md) | 2026-04-20 multi-platform-plugin-design and 2026-05-06 harness-parity-epic-design | Pre-MCP plugin design assumed per-project server and skills-as-CLI-wrappers; both assumptions are gone. |

## Historical

These plans pre-date the current architecture but are not formally superseded.
They cover early-project bootstrapping (vertical slices 1–6) and are kept as
project archeology.

| Date | Document |
|------|----------|
| 2026-03-07 | [slice-5-spec-lifecycle-revised-plan](2026-03-07-slice-5-spec-lifecycle-revised-plan.md), [domain-types-and-slice4-plan](2026-03-07-domain-types-and-slice4-plan.md) |
| 2026-03-03 | [slice-3.5-scanner-cleanup-plan](2026-03-03-slice-3.5-scanner-cleanup-plan.md) |
| 2026-02-28 | [vertical-slice-roadmap-design](2026-02-28-vertical-slice-roadmap-design.md), [vertical-slice-plan](2026-02-28-vertical-slice-plan.md), [implementation-tracker](2026-02-28-implementation-tracker.md), [slice-2-constitution-plan](2026-02-28-slice-2-constitution-plan.md), [slice-3-authoring-funnel-plan](2026-02-28-slice-3-authoring-funnel-plan.md), [slice-4-execution-bundles-plan](2026-02-28-slice-4-execution-bundles-plan.md), [slice-5-spec-lifecycle-plan](2026-02-28-slice-5-spec-lifecycle-plan.md), [slice-6-sync-integration-plan](2026-02-28-slice-6-sync-integration-plan.md) |

## Conventions

- **Filename:** `YYYY-MM-DD-<topic>-design.md` and `YYYY-MM-DD-<topic>-plan.md`.
- **Pairing:** every implementation plan should reference its companion
  design doc in the header.
- **Supersession:** when a plan is superseded, add a status banner at the top
  of the document and update this index.
- **Plans for closed/shipped beads** stay in this directory permanently.
  Don't delete plans; mark them shipped or superseded.
