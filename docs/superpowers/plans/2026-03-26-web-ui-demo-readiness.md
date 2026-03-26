# Web UI Demo-Readiness Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the web UI demo-ready by adding a browseable spec table to the dashboard, analytical findings to spec detail, linked specs to decision detail, and metadata everywhere.

**Architecture:** Backend adds `conversation_count` field to proto + Cypher subquery. Frontend adds four new Svelte components (TabBar, SpecTable, MetadataBar, FindingsSection) and enhances three existing pages (dashboard, spec detail, decision detail). All data comes from existing RPCs except findings which needs the AnalyticalPassService client.

**Tech Stack:** Go (proto, storage, convert), SvelteKit 2 + Svelte 5 ($state/$derived/$props runes), ConnectRPC (@connectrpc/connect-web), TypeScript

**Spec:** `docs/superpowers/specs/2026-03-26-web-ui-demo-readiness-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `proto/specgraph/v1/spec.proto` | Modify | Add `conversation_count` field 21 |
| `internal/storage/spec_domain.go` | Modify | Add `ConversationCount` to Spec struct |
| `internal/storage/memgraph/memgraph.go` | Modify | Add OPTIONAL MATCH subquery to ListSpecs |
| `internal/server/convert.go` | Modify | Map ConversationCount to proto |
| `web/src/lib/api/client.ts` | Modify | Add analyticalPassClient export |
| `web/src/lib/components/TabBar.svelte` | Create | Generic tab switcher |
| `web/src/lib/components/SpecTable.svelte` | Create | Reusable spec table with sort + links |
| `web/src/lib/components/MetadataBar.svelte` | Create | Timestamps/lifecycle/hash bar |
| `web/src/lib/components/FindingsSection.svelte` | Create | Findings grouped by pass type |
| `web/src/routes/+page.svelte` | Modify | Add tabbed content area below existing dashboard |
| `web/src/routes/spec/[...slug]/+page.svelte` | Modify | Add MetadataBar + FindingsSection |
| `web/src/routes/decision/[...slug]/+page.svelte` | Modify | Add linked specs + timestamps |

---

## Chunk 1: Backend — conversation_count Field

### Task 1: Add conversation_count to proto + domain + storage + convert

**Files:**

- Modify: `proto/specgraph/v1/spec.proto:44` (after decompose_output field 20)
- Modify: `internal/storage/spec_domain.go:155` (after DecomposeOutput)
- Modify: `internal/storage/memgraph/memgraph.go:369-373` (ListSpecs RETURN clause)
- Modify: `internal/storage/memgraph/memgraph.go:702+` (recordToSpecOffset)
- Modify: `internal/server/convert.go:45` (specToProto)

- [ ] **Step 1: Add field to proto**

In `proto/specgraph/v1/spec.proto`, after line 44 (`decompose_output = 20`), add:

```proto
  int32 conversation_count = 21; // count of conversation log entries (populated by ListSpecs)
```

- [ ] **Step 2: Run `task proto`**

```bash
task proto
```

Expected: Regenerates `gen/specgraph/v1/spec.pb.go` and TS types.

- [ ] **Step 3: Add field to domain Spec struct**

In `internal/storage/spec_domain.go`, after the `DecomposeOutput` field (line 155), add:

```go
	ConversationCount int // count of conversation log entries (populated by ListSpecs)
```

- [ ] **Step 4: Add OPTIONAL MATCH to ListSpecs query**

In `internal/storage/memgraph/memgraph.go`, the ListSpecs query (line 369) currently returns 18 columns. After the MATCH and WHERE clauses but before RETURN, add an OPTIONAL MATCH for conversation count. Replace the RETURN clause:

Current (line 369-373):

```go
	query += ` RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.notes, s.content_hash,
		       s.spark_output, s.shape_output, s.specify_output, s.decompose_output`
```

Replace with:

```go
	query += `
		OPTIONAL MATCH (s)-[:AUTHORED_VIA]->(:ConversationLog)-[:CONTINUES*0..]->(cl:ConversationLog)
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.notes, s.content_hash,
		       s.spark_output, s.shape_output, s.specify_output, s.decompose_output,
		       count(DISTINCT cl) AS conversation_count`
```

This adds a 19th column `conversation_count`.

- [ ] **Step 5: Update recordToSpecOffset to read conversation_count**

In `internal/storage/memgraph/memgraph.go`, the `recordToSpecOffset` function returns a Spec from record columns. After the existing field parsing (the last field is decompose_output at offset+17), add parsing for the new column. Find where the Spec struct is assembled (around line 787) and add:

```go
	convCount, _ := recordInt64(rec, offset+18, "conversation_count")
```

Then set it on the spec:

```go
	spec.ConversationCount = int(convCount)
```

Note: use `_` for the error since OPTIONAL MATCH may return null (0) and that's fine.

- [ ] **Step 6: Update specToProto in convert.go**

In `internal/server/convert.go`, in the `specToProto` function, after `ContentHash` (around line 44), add:

```go
	pb.ConversationCount = int32(s.ConversationCount)
```

- [ ] **Step 7: Verify build**

```bash
go build ./...
```

Expected: Clean build.

- [ ] **Step 8: Run tests**

```bash
go test ./internal/storage/memgraph/ -run TestCreateAndGetSpec -v -count=1
go test ./internal/server/ -run TestSpec -v -count=1
```

Expected: All pass. The conversation_count will be 0 in existing tests (no ConversationLog nodes).

- [ ] **Step 9: Commit**

```bash
jj --no-pager describe -m "feat(proto): add conversation_count field to Spec for dashboard display (spgr-5re)"
```

---

## Chunk 2: Frontend Components — TabBar, SpecTable, MetadataBar

### Task 2: Create TabBar component

**Files:**

- Create: `web/src/lib/components/TabBar.svelte`

- [ ] **Step 1: Create TabBar.svelte**

```svelte
<script lang="ts">
  interface Props {
    tabs: string[];
    active: string;
    onchange: (tab: string) => void;
  }
  let { tabs, active, onchange }: Props = $props();
</script>

<div class="tab-bar">
  {#each tabs as tab}
    <button
      class="tab"
      class:active={tab === active}
      onclick={() => onchange(tab)}
    >
      {tab}
    </button>
  {/each}
</div>

<style>
  .tab-bar {
    display: flex;
    gap: 0;
    border-bottom: 2px solid #e2e8f0;
    margin-bottom: 1rem;
  }

  .tab {
    padding: 0.5rem 1rem;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    margin-bottom: -2px;
    cursor: pointer;
    font-size: 0.85rem;
    font-weight: 500;
    color: #64748b;
    transition: color 0.15s, border-color 0.15s;
  }

  .tab:hover {
    color: #1a1a2e;
  }

  .tab.active {
    color: #2563eb;
    border-bottom-color: #2563eb;
    font-weight: 600;
  }
</style>
```

- [ ] **Step 2: Verify it compiles**

```bash
cd web && pnpm svelte-check
```

Expected: No errors on TabBar.svelte.

### Task 3: Create SpecTable component

**Files:**

- Create: `web/src/lib/components/SpecTable.svelte`

- [ ] **Step 1: Create SpecTable.svelte**

```svelte
<script lang="ts">
  import type { Spec } from '$lib/api/gen/specgraph/v1/spec_pb';

  interface Props {
    specs: Spec[];
    showConversations?: boolean;
  }
  let { specs, showConversations = true }: Props = $props();

  let sortKey = $state<'slug' | 'stage' | 'priority' | 'updated'>('slug');
  let sortAsc = $state(true);

  function relativeTime(ts: { seconds: bigint } | undefined): string {
    if (!ts) return '—';
    const now = Date.now();
    const then = Number(ts.seconds) * 1000;
    const diff = now - then;
    if (diff < 60_000) return 'just now';
    if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
    if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
    return `${Math.floor(diff / 86_400_000)}d ago`;
  }

  function sortedSpecs(): Spec[] {
    return [...specs].sort((a, b) => {
      let cmp = 0;
      switch (sortKey) {
        case 'slug': cmp = a.slug.localeCompare(b.slug); break;
        case 'stage': cmp = a.stage.localeCompare(b.stage); break;
        case 'priority': cmp = a.priority.localeCompare(b.priority); break;
        case 'updated': cmp = Number(a.updatedAt?.seconds ?? 0n) - Number(b.updatedAt?.seconds ?? 0n); break;
      }
      return sortAsc ? cmp : -cmp;
    });
  }

  function toggleSort(key: typeof sortKey) {
    if (sortKey === key) { sortAsc = !sortAsc; }
    else { sortKey = key; sortAsc = true; }
  }

  let rows = $derived(sortedSpecs());
</script>

<table class="spec-table">
  <thead>
    <tr>
      <th class="sortable" onclick={() => toggleSort('slug')}>Slug {sortKey === 'slug' ? (sortAsc ? '↑' : '↓') : ''}</th>
      <th class="sortable" onclick={() => toggleSort('stage')}>Stage {sortKey === 'stage' ? (sortAsc ? '↑' : '↓') : ''}</th>
      <th class="sortable" onclick={() => toggleSort('priority')}>Pri {sortKey === 'priority' ? (sortAsc ? '↑' : '↓') : ''}</th>
      {#if showConversations}<th>💬</th>{/if}
      <th>Intent</th>
      <th class="sortable" onclick={() => toggleSort('updated')}>Updated {sortKey === 'updated' ? (sortAsc ? '↑' : '↓') : ''}</th>
    </tr>
  </thead>
  <tbody>
    {#each rows as spec}
      <tr>
        <td><a href="/spec/{spec.slug}">{spec.slug}</a></td>
        <td><span class="badge stage-{spec.stage}">{spec.stage}</span></td>
        <td>{spec.priority || '—'}</td>
        {#if showConversations}<td class="count">{spec.conversationCount}</td>{/if}
        <td class="intent">{spec.intent}</td>
        <td class="time">{relativeTime(spec.updatedAt)}</td>
      </tr>
    {/each}
    {#if rows.length === 0}
      <tr><td colspan={showConversations ? 6 : 5} class="empty">No specs found</td></tr>
    {/if}
  </tbody>
</table>

<style>
  .spec-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }

  th {
    text-align: left;
    padding: 0.4rem 0.5rem;
    background: #f8fafc;
    color: #475569;
    font-weight: 600;
    border-bottom: 1px solid #e2e8f0;
    white-space: nowrap;
  }

  th.sortable {
    cursor: pointer;
    user-select: none;
  }

  th.sortable:hover {
    color: #2563eb;
  }

  td {
    padding: 0.4rem 0.5rem;
    border-bottom: 1px solid #f1f5f9;
    vertical-align: top;
  }

  td a {
    color: #2563eb;
    text-decoration: none;
    font-weight: 500;
  }

  td a:hover {
    text-decoration: underline;
  }

  .badge {
    display: inline-block;
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    font-size: 0.75rem;
    font-weight: 600;
  }

  .badge.stage-spark { background: #ede9fe; color: #7c3aed; }
  .badge.stage-shape { background: #dbeafe; color: #2563eb; }
  .badge.stage-specify { background: #dcfce7; color: #16a34a; }
  .badge.stage-decompose { background: #fef9c3; color: #ca8a04; }
  .badge.stage-approved { background: #ccfbf1; color: #0d9488; }
  .badge.stage-in_progress { background: #ffedd5; color: #ea580c; }
  .badge.stage-done { background: #f1f5f9; color: #6b7280; }

  .intent {
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .time {
    color: #64748b;
    white-space: nowrap;
  }

  .count {
    text-align: center;
    color: #64748b;
  }

  .empty {
    text-align: center;
    color: #94a3b8;
    padding: 1.5rem;
  }
</style>
```

- [ ] **Step 2: Verify it compiles**

```bash
cd web && pnpm svelte-check
```

### Task 4: Create MetadataBar component

**Files:**

- Create: `web/src/lib/components/MetadataBar.svelte`

- [ ] **Step 1: Create MetadataBar.svelte**

```svelte
<script lang="ts">
  interface Props {
    createdAt?: { seconds: bigint };
    updatedAt?: { seconds: bigint };
    lifecycle?: string;
    contentHash?: string;
  }
  let { createdAt, updatedAt, lifecycle, contentHash }: Props = $props();

  function formatDate(ts: { seconds: bigint } | undefined): string {
    if (!ts) return '—';
    return new Date(Number(ts.seconds) * 1000).toLocaleDateString('en-US', {
      month: 'short', day: 'numeric', year: 'numeric',
    });
  }

  function relativeTime(ts: { seconds: bigint } | undefined): string {
    if (!ts) return '—';
    const now = Date.now();
    const then = Number(ts.seconds) * 1000;
    const diff = now - then;
    if (diff < 60_000) return 'just now';
    if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
    if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
    return `${Math.floor(diff / 86_400_000)}d ago`;
  }
</script>

<div class="metadata-bar">
  <span>Created: <strong>{formatDate(createdAt)}</strong></span>
  <span class="sep">·</span>
  <span>Updated: <strong>{relativeTime(updatedAt)}</strong></span>
  {#if lifecycle}
    <span class="sep">·</span>
    <span>Lifecycle: <span class="lifecycle-badge">{lifecycle}</span></span>
  {/if}
  {#if contentHash}
    <span class="sep">·</span>
    <span class="hash" title={contentHash}>hash: {contentHash.slice(0, 8)}…</span>
  {/if}
</div>

<style>
  .metadata-bar {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    align-items: center;
    font-size: 0.8rem;
    color: #64748b;
    margin-bottom: 0.75rem;
  }

  .sep {
    color: #cbd5e1;
  }

  strong {
    color: #475569;
    font-weight: 500;
  }

  .lifecycle-badge {
    background: #dbeafe;
    color: #2563eb;
    padding: 0.05rem 0.3rem;
    border-radius: 3px;
    font-size: 0.7rem;
    font-weight: 600;
  }

  .hash {
    font-family: ui-monospace, monospace;
    font-size: 0.7rem;
    color: #94a3b8;
  }
</style>
```

- [ ] **Step 2: Commit components**

```bash
jj --no-pager describe -m "feat(web): add TabBar, SpecTable, MetadataBar components (spgr-5re)"
```

---

## Chunk 3: Frontend — FindingsSection + analyticalPassClient

### Task 5: Add analyticalPassClient and FindingsSection

**Files:**

- Modify: `web/src/lib/api/client.ts`
- Create: `web/src/lib/components/FindingsSection.svelte`

- [ ] **Step 1: Add analyticalPassClient to client.ts**

In `web/src/lib/api/client.ts`, add the import and export:

```typescript
import { AnalyticalPassService } from './gen/specgraph/v1/analytical_pass_pb';
```

And at the bottom:

```typescript
export const analyticalPassClient = createClient(AnalyticalPassService, transport);
```

- [ ] **Step 2: Create FindingsSection.svelte**

```svelte
<script lang="ts">
  import type { AnalyticalFinding } from '$lib/api/gen/specgraph/v1/analytical_pass_pb';
  import { FindingSeverity, PassType } from '$lib/api/gen/specgraph/v1/analytical_pass_pb';

  interface Props {
    findings: AnalyticalFinding[];
  }
  let { findings }: Props = $props();

  function passTypeLabel(pt: PassType): string {
    const labels: Record<number, string> = {
      [PassType.CONSTITUTION_CHECK]: 'Constitution Check',
      [PassType.COMPLEXITY_ANALYSIS]: 'Complexity Analysis',
      [PassType.DEPENDENCY_REVIEW]: 'Dependency Review',
      [PassType.SCOPE_VALIDATION]: 'Scope Validation',
      [PassType.RISK_ASSESSMENT]: 'Risk Assessment',
    };
    return labels[pt] ?? `Pass ${pt}`;
  }

  function severityClass(s: FindingSeverity): string {
    switch (s) {
      case FindingSeverity.INFO: return 'info';
      case FindingSeverity.WARNING: return 'warning';
      case FindingSeverity.ERROR: return 'error';
      default: return 'info';
    }
  }

  interface GroupedPass {
    passType: PassType;
    label: string;
    findings: AnalyticalFinding[];
  }

  let grouped = $derived(
    Object.values(
      findings.reduce((acc, f) => {
        const key = f.passType;
        if (!acc[key]) acc[key] = { passType: key, label: passTypeLabel(key), findings: [] };
        acc[key].findings.push(f);
        return acc;
      }, {} as Record<number, GroupedPass>)
    )
  );
</script>

{#each grouped as group}
  <div class="pass-card {group.findings.length > 0 ? 'has-findings' : 'passed'}">
    <div class="pass-header">
      <strong>{group.label}</strong>
      <span class="count-badge {group.findings.length > 0 ? 'amber' : 'green'}">
        {group.findings.length > 0 ? `${group.findings.length} finding${group.findings.length > 1 ? 's' : ''}` : 'passed'}
      </span>
    </div>
    {#if group.findings.length > 0}
      <div class="findings-list">
        {#each group.findings as finding}
          <div class="finding {severityClass(finding.severity)}">
            <span class="finding-summary">{finding.summary}</span>
            {#if finding.resolution}
              <span class="finding-resolution">→ {finding.resolution}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/each}

<style>
  .pass-card {
    padding: 0.5rem 0.75rem;
    margin-bottom: 0.5rem;
    border-radius: 0 4px 4px 0;
  }

  .pass-card.has-findings {
    border-left: 3px solid #f59e0b;
    background: #fffbeb;
  }

  .pass-card.passed {
    border-left: 3px solid #22c55e;
    background: #f0fdf4;
  }

  .pass-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .count-badge {
    font-size: 0.7rem;
    padding: 0.05rem 0.35rem;
    border-radius: 3px;
    font-weight: 600;
  }

  .count-badge.amber {
    background: #fef3c7;
    color: #b45309;
  }

  .count-badge.green {
    background: #dcfce7;
    color: #16a34a;
  }

  .findings-list {
    margin-top: 0.35rem;
  }

  .finding {
    font-size: 0.8rem;
    padding: 0.15rem 0;
    color: #92400e;
  }

  .finding.error {
    color: #dc2626;
  }

  .finding.info {
    color: #475569;
  }

  .finding-summary::before {
    content: '⚠ ';
  }

  .finding.error .finding-summary::before {
    content: '✘ ';
  }

  .finding.info .finding-summary::before {
    content: 'ℹ ';
  }

  .finding-resolution {
    font-size: 0.75rem;
    color: #64748b;
    margin-left: 0.25rem;
  }
</style>
```

- [ ] **Step 3: Verify build**

```bash
cd web && pnpm svelte-check
```

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe -m "feat(web): add FindingsSection component + analyticalPassClient (spgr-5re)"
```

---

## Chunk 4: Page Modifications — Dashboard, Spec Detail, Decision Detail

### Task 6: Enhance dashboard with tabbed content area

**Files:**

- Modify: `web/src/routes/+page.svelte`

- [ ] **Step 1: Add imports and state**

At the top of the `<script>` block, add imports for the new components:

```typescript
import TabBar from '$lib/components/TabBar.svelte';
import SpecTable from '$lib/components/SpecTable.svelte';
```

Add new state variables after the existing ones:

```typescript
let specs = $state<any[]>([]);
let activeTab = $state('All Specs');
const tabs = ['All Specs', 'Recent', 'By Priority', 'Decisions'];
```

- [ ] **Step 2: Store specs array from loadDashboard**

In the `loadDashboard` function, after `const specs = specsRes.specs ?? [];`, add:

```typescript
// Store full spec list for tabbed content (shadow the const above)
```

Actually, rename the local `specs` const to avoid shadowing. Change:

```typescript
const specs = specsRes.specs ?? [];
```

to:

```typescript
const specsList = specsRes.specs ?? [];
specs = specsList;
```

And update all subsequent references in that function from `specs` to `specsList` (the filter for `topLevel`, `sliceSlugs`, etc.).

- [ ] **Step 3: Compute derived data for tabs**

Add after state declarations:

```typescript
let recentSpecs = $derived(
  [...specs].sort((a, b) => Number(b.updatedAt?.seconds ?? 0n) - Number(a.updatedAt?.seconds ?? 0n)).slice(0, 10)
);

let priorityGroups = $derived(
  ['p0', 'p1', 'p2', 'p3'].map(p => ({
    label: p.toUpperCase(),
    specs: specs.filter(s => s.priority === p),
  })).filter(g => g.specs.length > 0)
);

// Decision tab: decisions with linked spec count from DECIDED_IN edges
let decisions = $state<any[]>([]);
let decisionSpecCounts = $derived(
  graphEdges
    .filter(e => e.edgeType === EdgeType.DECIDED_IN)
    .reduce((acc, e) => {
      acc[e.toId] = (acc[e.toId] ?? 0) + 1;
      return acc;
    }, {} as Record<string, number>)
);
```

In `loadDashboard`, after the `decisionsRes` line, add:

```typescript
decisions = decisionsRes.decisions ?? [];
```

- [ ] **Step 4: Add tabbed content area to template**

After the existing `</section>` closing tag (around line 86), but still inside the `{:else}` block, add:

```svelte
  <section class="tabbed-content">
    <TabBar {tabs} active={activeTab} onchange={(t) => activeTab = t} />

    {#if activeTab === 'All Specs'}
      <SpecTable {specs} />
    {:else if activeTab === 'Recent'}
      <SpecTable specs={recentSpecs} />
    {:else if activeTab === 'By Priority'}
      {#each priorityGroups as group}
        <h3 class="priority-heading">{group.label} <span class="priority-count">({group.specs.length})</span></h3>
        <SpecTable specs={group.specs} showConversations={false} />
      {/each}
    {:else if activeTab === 'Decisions'}
      <table class="decision-table">
        <thead>
          <tr><th>Slug</th><th>Title</th><th>Status</th><th>Linked Specs</th></tr>
        </thead>
        <tbody>
          {#each decisions as d}
            <tr>
              <td><a href="/decision/{d.slug}">{d.slug}</a></td>
              <td>{d.title || '—'}</td>
              <td><span class="badge">{d.status ? ['', 'proposed', 'accepted', 'deprecated', 'superseded'][d.status] || '?' : '—'}</span></td>
              <td class="count">{decisionSpecCounts[d.slug] ?? 0}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </section>
```

- [ ] **Step 5: Add styles**

Add to the `<style>` block:

```css
  .tabbed-content {
    margin-top: 1.25rem;
  }

  .priority-heading {
    font-size: 0.9rem;
    font-weight: 600;
    color: #475569;
    margin: 1rem 0 0.25rem;
  }

  .priority-count {
    font-weight: 400;
    color: #94a3b8;
  }

  .decision-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }

  .decision-table th {
    text-align: left;
    padding: 0.4rem 0.5rem;
    background: #f8fafc;
    color: #475569;
    font-weight: 600;
    border-bottom: 1px solid #e2e8f0;
  }

  .decision-table td {
    padding: 0.4rem 0.5rem;
    border-bottom: 1px solid #f1f5f9;
  }

  .decision-table a {
    color: #2563eb;
    text-decoration: none;
    font-weight: 500;
  }

  .decision-table a:hover {
    text-decoration: underline;
  }

  .decision-table .count {
    text-align: center;
    color: #64748b;
  }
```

- [ ] **Step 6: Verify build**

```bash
cd web && pnpm build
```

- [ ] **Step 7: Commit**

```bash
jj --no-pager describe -m "feat(web): add tabbed spec/decision content area to dashboard (spgr-5re)"
```

### Task 7: Enhance spec detail with MetadataBar + FindingsSection

**Files:**

- Modify: `web/src/routes/spec/[...slug]/+page.svelte`

- [ ] **Step 1: Add imports**

At the top of the `<script>` block, add:

```typescript
import { analyticalPassClient } from '$lib/api/client';
import type { AnalyticalFinding } from '$lib/api/gen/specgraph/v1/analytical_pass_pb';
import MetadataBar from '$lib/components/MetadataBar.svelte';
import FindingsSection from '$lib/components/FindingsSection.svelte';
import AccordionSection from '$lib/components/AccordionSection.svelte';
```

Note: AccordionSection is already imported — just add the other three.

- [ ] **Step 2: Add findings state and fetch**

Add state variable:

```typescript
let findings = $state<AnalyticalFinding[]>([]);
```

In the `loadSpec` function, after the edges fetch (inside the try block), add a non-critical findings fetch:

```typescript
      try {
        const findingsResp = await analyticalPassClient.listFindings({ slug: s });
        findings = findingsResp.findings;
      } catch {
        findings = [];
      }
```

- [ ] **Step 3: Add MetadataBar to template**

After `<h1>{spec.slug}</h1>` (line 117) and before `<table class="meta">`, add:

```svelte
  <MetadataBar
    createdAt={spec.createdAt}
    updatedAt={spec.updatedAt}
    lifecycle={spec.lifecycle}
    contentHash={spec.contentHash}
  />
```

- [ ] **Step 4: Add FindingsSection accordion**

After the Edges accordion (around line 283) and before the Conversations accordion (line 285), add:

```svelte
    {#if findings.length > 0}
      <AccordionSection title="Findings" badge={String(findings.length)}>
        <FindingsSection {findings} />
      </AccordionSection>
    {/if}
```

- [ ] **Step 5: Verify build**

```bash
cd web && pnpm build
```

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(web): add metadata bar + analytical findings to spec detail (spgr-5re)"
```

### Task 8: Enhance decision detail with linked specs + timestamps

**Files:**

- Modify: `web/src/routes/decision/[...slug]/+page.svelte`

- [ ] **Step 1: Add imports and state**

Add to imports:

```typescript
import { graphClient } from '$lib/api/client';
import type { Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
import { EdgeType } from '$lib/api/gen/specgraph/v1/graph_pb';
```

Add state:

```typescript
let linkedSpecs = $state<string[]>([]);
```

- [ ] **Step 2: Fetch edges in loadDecision**

After `decision = resp.decision ?? null;`, add:

```typescript
      try {
        const edgeResp = await graphClient.listEdges({ slug: s });
        linkedSpecs = edgeResp.edges
          .filter(e => e.edgeType === EdgeType.DECIDED_IN && e.toId === s)
          .map(e => e.fromId);
      } catch {
        linkedSpecs = [];
      }
```

- [ ] **Step 3: Add timestamps to meta table**

After the `supersededBy` row (line 58), add:

```svelte
      {#if decision.createdAt}
        <tr><td class="label">Created</td><td>{new Date(Number(decision.createdAt.seconds) * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}</td></tr>
      {/if}
      {#if decision.updatedAt}
        <tr><td class="label">Updated</td><td>{new Date(Number(decision.updatedAt.seconds) * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}</td></tr>
      {/if}
```

- [ ] **Step 4: Add linked specs section**

After the rationale section (after line 74), add:

```svelte
  {#if linkedSpecs.length > 0}
    <section class="section">
      <h2>Referenced by</h2>
      <div class="spec-chips">
        {#each linkedSpecs as specSlug}
          <a href="/spec/{specSlug}" class="spec-chip">{specSlug}</a>
        {/each}
      </div>
    </section>
  {/if}
```

- [ ] **Step 5: Add styles**

Add to the `<style>` block:

```css
  .spec-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .spec-chip {
    background: #eff6ff;
    color: #2563eb;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.8rem;
    text-decoration: none;
    font-weight: 500;
  }

  .spec-chip:hover {
    background: #dbeafe;
  }
```

- [ ] **Step 6: Verify build**

```bash
cd web && pnpm build
```

- [ ] **Step 7: Commit**

```bash
jj --no-pager describe -m "feat(web): add linked specs + timestamps to decision detail (spgr-5re)"
```

---

## Chunk 5: Final Verification + PR

### Task 9: Verify, squash, and create PR

- [ ] **Step 1: Run task check**

```bash
task check
```

Expected: All Go checks pass.

- [ ] **Step 2: Run svelte-check**

```bash
cd web && pnpm svelte-check
```

Expected: No TypeScript or Svelte errors.

- [ ] **Step 3: Build web UI**

```bash
cd web && pnpm build
```

Expected: Clean build with new components bundled.

- [ ] **Step 4: Squash changes**

```bash
jj --no-pager squash --into <first-change-id> -m "feat(web): dashboard overhaul + spec/decision detail enhancements for demo-readiness (spgr-5re)"
```

- [ ] **Step 5: Create bookmark and push**

```bash
jj --no-pager bookmark set feat/web-ui-demo -r @
jj --no-pager git push --bookmark feat/web-ui-demo
```

- [ ] **Step 6: Create PR**

```bash
gh pr create \
  --title "feat(web): dashboard overhaul + detail page enhancements for demo-readiness (spgr-5re)" \
  --body "$(cat <<'EOF'
## Summary
- **Dashboard**: Add tabbed content area (All Specs / Recent / By Priority / Decisions) below existing stats/funnel/graph
- **Spec detail**: Add metadata bar (timestamps, lifecycle, content hash) + analytical findings accordion
- **Decision detail**: Add linked specs section (DECIDED_IN edges) + timestamps
- **Backend**: Add `conversation_count` field to Spec proto + Cypher subquery in ListSpecs

Conversation count shows 0 until spgr-cdd (skill → RecordConversation wiring) is complete.

## New Components
- `TabBar.svelte` — generic tab switcher
- `SpecTable.svelte` — sortable spec table with stage badges and relative timestamps
- `MetadataBar.svelte` — created/updated/lifecycle/hash display
- `FindingsSection.svelte` — analytical findings grouped by pass type

## Test plan
- [ ] Dashboard shows spec table with All Specs tab by default
- [ ] Recent tab shows last 10 specs sorted by updated time
- [ ] By Priority tab groups specs under P0/P1/P2/P3 headers
- [ ] Decisions tab shows decisions with linked spec counts
- [ ] Spec detail shows metadata bar with timestamps
- [ ] Spec detail shows findings accordion when findings exist
- [ ] Decision detail shows linked spec chips
- [ ] Decision detail shows created/updated dates
- [ ] `task check` passes
- [ ] `pnpm svelte-check` passes

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```
