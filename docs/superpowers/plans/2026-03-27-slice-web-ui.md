# Slice Web UI Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Slice support to the web UI — client export, spec detail page async fetch with status badges, and graph visualization with filled pill nodes.

**Architecture:** Add `sliceClient` export. Replace inline `decomposeOutput.slices` rendering on spec detail page with async `sliceClient.listSlices()` fetch. Update `Graph.svelte` to render Slice nodes as filled pills with status-tinted backgrounds. Svelte 5 runes (`$state`, `$derived`, `$effect`). pnpm not npm.

**Tech Stack:** SvelteKit, Svelte 5 runes, @connectrpc/connect-web, TypeScript, pnpm

**Bead:** spgr-6sw.7

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `web/src/lib/api/client.ts` | Add `sliceClient` export |
| Modify | `web/src/routes/spec/[...slug]/+page.svelte` | Async slice fetch, status badges, replace inline rendering |
| Modify | `web/src/lib/components/Graph.svelte` | Slice node rendering (filled pill), `isSlice` check, status colors |

---

## Task 1: Add sliceClient export

**Files:**

- Modify: `web/src/lib/api/client.ts`

- [ ] **Step 1: Add SliceService import and client export**

In `web/src/lib/api/client.ts`, add the import alongside the existing service imports:

```typescript
import { SliceService } from './gen/specgraph/v1/slice_pb';
```

And add the export at the end:

```typescript
export const sliceClient = createClient(SliceService, transport);
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && pnpm svelte-check`

Expected: No errors related to sliceClient.

- [ ] **Step 3: Commit**

```text
jj --no-pager describe -m "feat(web): add sliceClient export for SliceService

Follows the existing pattern for specClient, decisionClient, etc.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Spec detail page — async slice fetch with status badges

Replace inline `decomposeOutput.slices` rendering with an async `sliceClient.listSlices()` call. Show slice cards with status badges and assignee.

**Files:**

- Modify: `web/src/routes/spec/[...slug]/+page.svelte`

- [ ] **Step 1: Add sliceClient import and state**

At the top of the `<script>` block, add to the existing imports:

```typescript
import { sliceClient } from '$lib/api/client';
import type { Slice } from '$lib/api/gen/specgraph/v1/slice_pb';
import { SliceStatus } from '$lib/api/gen/specgraph/v1/slice_pb';
```

Add state variable alongside existing ones (`spec`, `edges`, `findings`):

```typescript
let slices = $state<Slice[]>([]);
```

- [ ] **Step 2: Add async slice fetch in loadSpec**

Inside the `loadSpec` function, after the findings fetch block (around line 39), add a new non-critical fetch:

```typescript
      // Slices are non-critical — fetch separately.
      try {
        if (specResp.spec?.decomposeOutput) {
          const sliceResp = await sliceClient.listSlices({ parentSlug: s });
          slices = sliceResp.slices;
        } else {
          slices = [];
        }
      } catch {
        slices = [];
      }
```

- [ ] **Step 3: Add slice status helper**

Add after the existing `strategyLabel` function:

```typescript
  function sliceStatusBadge(status: SliceStatus): { label: string; color: string } {
    switch (status) {
      case SliceStatus.OPEN: return { label: 'open', color: '#6b7280' };
      case SliceStatus.CLAIMED: return { label: 'claimed', color: '#ea580c' };
      case SliceStatus.DONE: return { label: 'done', color: '#16a34a' };
      default: return { label: 'unknown', color: '#6b7280' };
    }
  }
```

- [ ] **Step 4: Replace decompose section template**

Replace the decompose section (lines 269-288) with:

```svelte
    {#if spec.decomposeOutput}
      <AccordionSection title="Decompose" badge={slices.length ? slices.length + ' slices' : 'output'} expanded={shouldExpand('decompose')}>
        {#if strategyLabel(spec.decomposeOutput.strategy)}
          <p><strong>Strategy:</strong> {strategyLabel(spec.decomposeOutput.strategy)}</p>
        {/if}
        {#if slices.length > 0}
          {#each slices as slice}
            {@const badge = sliceStatusBadge(slice.status)}
            <div class="slice-card">
              <div class="slice-header">
                <strong>{slice.sliceId}</strong>
                <span class="slice-badge" style="background:{badge.color}">{badge.label}</span>
              </div>
              {#if slice.intent}<p>{slice.intent}</p>{/if}
              {#if slice.assignedTo}
                <p class="slice-label">Assigned to: {slice.assignedTo}</p>
              {/if}
              {#if slice.verify.length > 0}
                <p class="slice-label">Verify:</p>
                <ul>{#each slice.verify as v}<li>{v}</li>{/each}</ul>
              {/if}
              {#if slice.dependsOn.length > 0}
                <p class="slice-label">Depends on: {slice.dependsOn.join(', ')}</p>
              {/if}
            </div>
          {/each}
        {:else if spec.decomposeOutput.sliceSlugs.length > 0}
          <p class="slice-label">{spec.decomposeOutput.sliceSlugs.length} slice(s) — loading failed or pending</p>
        {/if}
      </AccordionSection>
    {/if}
```

- [ ] **Step 5: Update styles**

Replace the existing `.slice-card` and `.slice-label` styles with:

```css
  .slice-card {
    padding: 0.5rem 0.75rem;
    margin: 0.25rem 0;
    background: #f8fafc;
    border-radius: 6px;
    border-left: 3px solid #ca8a04;
  }

  .slice-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .slice-badge {
    font-size: 0.7rem;
    color: white;
    padding: 0.1rem 0.4rem;
    border-radius: 9999px;
    font-weight: 500;
  }

  .slice-label {
    font-size: 0.85rem;
    color: #64748b;
    margin: 0.25rem 0 0;
  }
```

- [ ] **Step 6: Verify build**

Run: `cd web && pnpm build`

Expected: Clean build.

- [ ] **Step 7: Commit**

```text
jj --no-pager describe -m "feat(web): spec detail page fetches slices via SliceService RPC (spgr-6sw.7)

Replace inline decomposeOutput.slices rendering with async
sliceClient.listSlices() fetch. Status badges (open=gray, claimed=orange,
done=green), assigned_to display, fallback for loading failure.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Graph.svelte — Slice node rendering as filled pills

**Files:**

- Modify: `web/src/lib/components/Graph.svelte`

- [ ] **Step 1: Add Slice constants and colors**

After the existing constants (`NODE_W`, `NODE_H`, `DECISION_SIZE`), add:

```typescript
  const SLICE_W = 130;
  const SLICE_H = 36;

  const sliceColors: Record<string, { fill: string; stroke: string }> = {
    open: { fill: '#f0f9ff', stroke: '#0ea5e9' },
    claimed: { fill: '#fff7ed', stroke: '#ea580c' },
    done: { fill: '#f0fdf4', stroke: '#16a34a' },
  };
```

- [ ] **Step 2: Update LayoutNode interface and node sizing**

Add `isSlice` to the `LayoutNode` interface:

```typescript
  interface LayoutNode {
    slug: string;
    label: string;
    stage: string;
    intent: string;
    priority: string;
    x: number;
    y: number;
    isDecision: boolean;
    isSlice: boolean;
  }
```

In the `layout` derived block, update the node sizing loop (around line 66):

```typescript
    for (const n of nodes) {
      const isDecision = n.label === 'Decision';
      const isSlice = n.label === 'Slice';
      g.setNode(n.slug, {
        width: isDecision ? DECISION_SIZE * 1.5 : isSlice ? SLICE_W : NODE_W,
        height: isDecision ? DECISION_SIZE * 1.5 : isSlice ? SLICE_H : NODE_H,
      });
    }
```

And in the `layoutNodes` map (around line 92), add `isSlice`:

```typescript
      return {
        slug: n.slug,
        label: n.label,
        stage: n.stage,
        intent: n.intent,
        priority: n.priority,
        x: pos.x,
        y: pos.y,
        isDecision: n.label === 'Decision',
        isSlice: n.label === 'Slice',
      };
```

- [ ] **Step 3: Add Slice rendering in SVG template**

In the SVG template, update the node rendering block (around line 229). The current structure is:

```svelte
{#if node.isDecision}
  <!-- diamond -->
{:else}
  <!-- rectangle -->
{/if}
```

Change to:

```svelte
{#if node.isDecision}
  <!-- diamond (unchanged) -->
{:else if node.isSlice}
  {@const sc = sliceColors[node.stage] ?? { fill: '#f8fafc', stroke: '#6b7280' }}
  <rect
    x={node.x - SLICE_W / 2}
    y={node.y - SLICE_H / 2}
    width={SLICE_W}
    height={SLICE_H}
    rx="18"
    fill={sc.fill}
    stroke={sc.stroke}
    stroke-width="1.5"
  />
  <text
    x={node.x}
    y={node.y - 2}
    text-anchor="middle"
    font-size="10"
    font-weight="500"
    fill="#1a1a2e"
  >
    {truncate(node.slug.split('/').pop() ?? node.slug, 16)}
  </text>
  <text
    x={node.x}
    y={node.y + 10}
    text-anchor="middle"
    font-size="8"
    fill={sc.stroke}
  >
    {node.stage}
  </text>
{:else}
  <!-- rectangle (unchanged) -->
{/if}
```

- [ ] **Step 4: Update the link href for Slice nodes**

The current link wraps all nodes:

```svelte
<a href="{node.isDecision ? '/decision' : '/spec'}/{node.slug}">
```

Update to link Slice nodes to their parent spec:

```svelte
<a href="{node.isDecision ? '/decision' : '/spec'}/{node.isSlice ? node.slug.split('/').slice(0, -1).join('/') : node.slug}">
```

- [ ] **Step 5: Verify build**

Run: `cd web && pnpm build`

Expected: Clean build.

- [ ] **Step 6: Full quality check**

Run: `task check`

Expected: All pass.

- [ ] **Step 7: Commit**

```text
jj --no-pager describe -m "feat(web): render Slice nodes as filled pills in graph (spgr-6sw.7)

Slice nodes: pill shape (rx=18, 130x36), tinted fill by status
(open=blue, claimed=orange, done=green). Shows slice-id (not full slug)
and status label. Links to parent spec detail page.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 8: Close bead**

```text
bd close spgr-6sw.7 --reason="Web UI: sliceClient, spec detail async fetch with status badges, graph pill rendering"
```
