# Web UI Demo-Readiness: Dashboard Overhaul + Detail Page Enhancements

**Date**: 2026-03-26
**Status**: Approved
**Bead**: spgr-5re
**Depends on**: spgr-cdd (conversation recording — column shows 0 until that lands)

## Problem

The web UI has significant data gaps that make it look lossy during demos:

- **Dashboard** (27% data utilization): Shows aggregate stats, funnel, and mini graph but no spec list. Users must navigate to /graph to find individual specs.
- **Spec detail**: Missing timestamps, lifecycle, content hash, and analytical findings.
- **Decision detail**: Shows title/status/rationale but no edges back to specs — decisions appear disconnected.
- **Conversation count**: Always 0 because authoring skills don't call RecordConversation (tracked as spgr-cdd).

## Changes

### 1. Dashboard Tabbed Content Area

Keep existing elements (stats bar, funnel chart, mini graph) at the top. Add a tabbed content area below with four tabs:

| Tab | Content | Data Source |
|-----|---------|-------------|
| **All Specs** | Sortable table: slug (link to /spec/), stage (badge), priority, 💬 count, intent (truncated), updated (relative time) | `ListSpecs` response |
| **Recent** | Same table columns, sorted by `updatedAt` desc, limited to 10 | Same data, pre-sorted |
| **By Priority** | Specs grouped under P0/P1/P2/P3 section headers | Same data, grouped |
| **Decisions** | Decision table: slug (link to /decision/), title, status (badge), linked spec count | `ListDecisions` + DECIDED_IN edges from `GetFullGraph` |

Default tab: "All Specs". Tab state is client-side only (reactive variable, no URL routing).

### 2. Spec Detail Metadata Bar

New row between `<h1>` and existing meta table. Shows:

```text
Created: Mar 15, 2026 · Updated: 2 hours ago · Lifecycle: task · hash: a1b2c3d4...
```

Uses `spec.createdAt`, `spec.updatedAt` (relative time, client-side formatting), `spec.lifecycle`, `spec.contentHash` (truncated).

### 3. Spec Detail Analytical Findings Accordion

New `AccordionSection` after Edges, before Conversations. Requires new RPC call: `analyticalPassClient.listFindings({ slug })`.

Renders each pass type as a card:

- Pass name as heading
- Finding count badge (green = 0 findings/passed, amber = has findings)
- Individual findings show summary text
- Gated on findings array length > 0

### 4. Decision Detail Linked Specs

After the rationale section, add a "Referenced by" section. Fetch edges via `graphClient.listEdges({ slug })`. Filter for `DECIDED_IN` edges where the decision is the `toId`. Render linked spec slugs as chip-style links to `/spec/<slug>`.

Also add timestamps (`createdAt`, `updatedAt`) in muted metadata style matching spec detail.

### 5. Conversation Count in Spec Table

The dashboard spec table includes a 💬 column showing conversation log count per spec. This requires `ListSpecs` to return conversation counts.

**Backend change**: Add a `conversation_count` field to the Spec domain type. The Memgraph `ListSpecs` query populates it via a Cypher subquery counting ConversationLog nodes per spec. This avoids N+1 calls from the frontend.

**Proto change**: Add `int32 conversation_count = 21` to the `Spec` message (field 21 in `proto/specgraph/v1/spec.proto`).

The column shows 0 until spgr-cdd (authoring skill conversation recording) is complete.

## New Components

| Component | Path | Purpose |
|-----------|------|---------|
| `SpecTable` | `web/src/lib/components/SpecTable.svelte` | Reusable spec table with columns, sort, and links |
| `TabBar` | `web/src/lib/components/TabBar.svelte` | Generic tab switcher component |
| `FindingsSection` | `web/src/lib/components/FindingsSection.svelte` | Findings accordion content |
| `MetadataBar` | `web/src/lib/components/MetadataBar.svelte` | Timestamp/lifecycle/hash bar |

## File Changes

### Backend (Go)

- Modify: `proto/specgraph/v1/spec.proto` — add `conversation_count` field
- Modify: `internal/storage/spec_domain.go` — add `ConversationCount int` to Spec struct
- Modify: `internal/storage/memgraph/memgraph.go` — add subquery count in ListSpecs query
- Modify: `internal/server/convert.go` — map ConversationCount to proto
- Run: `task proto` to regenerate

### Frontend (Svelte)

- Modify: `web/src/routes/+page.svelte` — add tabbed content area, import new components
- Modify: `web/src/routes/spec/[...slug]/+page.svelte` — add MetadataBar, FindingsSection, import analyticalPassClient
- Modify: `web/src/routes/decision/[...slug]/+page.svelte` — add linked specs section, timestamps, import graphClient
- Modify: `web/src/lib/api/client.ts` — add analyticalPassClient export
- Create: `web/src/lib/components/SpecTable.svelte`
- Create: `web/src/lib/components/TabBar.svelte`
- Create: `web/src/lib/components/FindingsSection.svelte`
- Create: `web/src/lib/components/MetadataBar.svelte`

## Out of Scope

- Wiring authoring skills to call RecordConversation (spgr-cdd)
- Drift acknowledgment UI
- Authoring actions from the web UI (stays read-only)
- Client-side search/filter on spec table (sort only)
