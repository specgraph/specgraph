# Spec Detail Page Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the web UI spec detail page with collapsible accordion sections for authoring stage outputs, edges, and conversations.

**Architecture:** Create one new `AccordionSection.svelte` component, then expand the existing `+page.svelte` with stage output sections, edge fetching via `graphClient.listEdges`, and conversation rendering. All data from `GetSpec` response (stage outputs + conversations) plus one `listEdges` call.

**Tech Stack:** SvelteKit 2, Svelte 5 (runes), @connectrpc/connect-web, TypeScript

**Spec:** `docs/superpowers/specs/2026-03-25-spec-detail-page-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `web/src/lib/components/AccordionSection.svelte` | Create | Reusable collapsible section with title, badge, chevron |
| `web/src/routes/spec/[...slug]/+page.svelte` | Modify | Add accordion sections for stages, edges, conversations |

---

## Chunk 1: AccordionSection Component

### Task 1: Create AccordionSection.svelte

**Files:**

- Create: `web/src/lib/components/AccordionSection.svelte`

- [ ] **Step 1: Create the component**

```svelte
<script lang="ts">
  interface Props {
    title: string;
    expanded?: boolean;
    badge?: string;
    children: import('svelte').Snippet;
  }
  let { title, expanded = false, badge = '', children }: Props = $props();
  let open = $state(expanded);
</script>

<div class="accordion">
  <button class="accordion-header" onclick={() => open = !open}>
    <span class="chevron" class:open>▶</span>
    <span class="accordion-title">{title}</span>
    {#if badge}<span class="accordion-badge">{badge}</span>{/if}
  </button>
  {#if open}
    <div class="accordion-body">
      {@render children()}
    </div>
  {/if}
</div>

<style>
  .accordion {
    border-bottom: 1px solid #e2e8f0;
    margin-bottom: 0.25rem;
  }

  .accordion-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.6rem 0;
    background: none;
    border: none;
    cursor: pointer;
    font-size: 0.95rem;
    font-weight: 600;
    color: #1a1a2e;
    text-align: left;
  }

  .accordion-header:hover {
    color: #2563eb;
  }

  .chevron {
    font-size: 0.7rem;
    transition: transform 0.15s ease;
    color: #94a3b8;
  }

  .chevron.open {
    transform: rotate(90deg);
  }

  .accordion-badge {
    font-size: 0.75rem;
    font-weight: 500;
    color: #64748b;
    background: #f1f5f9;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
  }

  .accordion-body {
    padding: 0 0 0.75rem 1.25rem;
    font-size: 0.9rem;
    color: #374151;
    line-height: 1.6;
  }
</style>
```

- [ ] **Step 2: Verify build**

Run: `cd web && pnpm svelte-check`

Expected: No errors on AccordionSection.svelte.

- [ ] **Step 3: Commit**

```text
feat(web): add AccordionSection component (spgr-zn1)
```

---

## Chunk 2: Expand Spec Detail Page

### Task 2: Add imports, edge fetching, and helper functions

**Files:**

- Modify: `web/src/routes/spec/[...slug]/+page.svelte`

- [ ] **Step 1: Add imports and state**

Update the `<script>` block. Add imports for `graphClient`, `AccordionSection`, the generated types, and `EdgeType`. Add state for edges:

```typescript
import { onMount } from 'svelte';
import { page } from '$app/stores';
import { specClient, graphClient } from '$lib/api/client';
import type { Spec } from '$lib/api/gen/specgraph/v1/spec_pb';
import type { Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
import { EdgeType } from '$lib/api/gen/specgraph/v1/graph_pb';
import { ScopeSniff, DecompositionStrategy } from '$lib/api/gen/specgraph/v1/authoring_pb';
import AccordionSection from '$lib/components/AccordionSection.svelte';

let spec = $state<Spec | null>(null);
let edges = $state<Edge[]>([]);
let loading = $state(true);
let error = $state<string | null>(null);

let slug = $derived($page.params.slug);
```

- [ ] **Step 2: Update loadSpec to also fetch edges**

Replace the existing `loadSpec` function:

```typescript
async function loadSpec(s: string) {
  try {
    const [specResp, edgeResp] = await Promise.all([
      specClient.getSpec({ slug: s }),
      graphClient.listEdges({ slug: s }),
    ]);
    spec = specResp.spec ?? null;
    edges = edgeResp.edges;
  } catch (err) {
    error = err instanceof Error ? err.message : 'Failed to load spec';
  } finally {
    loading = false;
  }
}
```

- [ ] **Step 3: Add helper functions**

After `loadSpec`, add helpers for display:

```typescript
function isCurrentStage(stage: string): boolean {
  return spec?.stage === stage;
}

function isPastStage(stage: string): boolean {
  const order = ['spark', 'shape', 'specify', 'decompose', 'approved', 'in_progress', 'review', 'done'];
  const current = order.indexOf(spec?.stage ?? '');
  const target = order.indexOf(stage);
  return target >= 0 && current > target;
}

function shouldExpand(stage: string): boolean {
  return isCurrentStage(stage);
}

function scopeSniffLabel(val: ScopeSniff): string {
  const labels: Record<number, string> = {
    [ScopeSniff.TINY]: 'tiny',
    [ScopeSniff.SMALL]: 'small',
    [ScopeSniff.MEDIUM]: 'medium',
    [ScopeSniff.LARGE]: 'large',
    [ScopeSniff.EPIC]: 'epic',
  };
  return labels[val] ?? '';
}

function strategyLabel(val: DecompositionStrategy): string {
  const labels: Record<number, string> = {
    [DecompositionStrategy.VERTICAL_SLICE]: 'vertical slice',
    [DecompositionStrategy.LAYER_CAKE]: 'layer cake',
    [DecompositionStrategy.SINGLE_UNIT]: 'single unit',
  };
  return labels[val] ?? '';
}

function edgeTypeLabel(val: EdgeType): string {
  const labels: Record<number, string> = {
    [EdgeType.DEPENDS_ON]: 'Depends on',
    [EdgeType.BLOCKS]: 'Blocks',
    [EdgeType.COMPOSES]: 'Composes',
    [EdgeType.RELATES_TO]: 'Relates to',
    [EdgeType.INFORMS]: 'Informs',
    [EdgeType.DECIDED_IN]: 'Decision',
    [EdgeType.SUPERSEDES]: 'Supersedes',
  };
  return labels[val] ?? String(val);
}

// Group edges by type for display
let groupedEdges = $derived(
  edges.reduce((acc, e) => {
    const label = edgeTypeLabel(e.edgeType);
    if (!acc[label]) acc[label] = [];
    // Show the "other" end: if fromId is our slug, show toId; otherwise show fromId
    const target = e.fromId === slug ? e.toId : e.fromId;
    acc[label].push(target);
    return acc;
  }, {} as Record<string, string[]>)
);
```

- [ ] **Step 4: Verify build**

Run: `cd web && pnpm svelte-check`

Expected: Pass (template not using new state yet, just script changes).

- [ ] **Step 5: Commit**

```text
feat(web): add edge fetching and helper functions to spec detail page (spgr-zn1)
```

### Task 3: Add stage output sections to template

**Files:**

- Modify: `web/src/routes/spec/[...slug]/+page.svelte` (template section)

- [ ] **Step 1: Replace the notes section and add all accordion sections**

After the metadata `<table>`, replace the existing `{#if spec.notes}` block with the full accordion layout. Add after the closing `</table>`:

```svelte
  <div class="sections">
    {#if spec.notes}
      <AccordionSection title="Notes" expanded={true}>
        <p class="notes">{spec.notes}</p>
      </AccordionSection>
    {/if}

    {#if spec.sparkOutput}
      <AccordionSection title="Spark" expanded={shouldExpand('spark')}>
        {#if spec.sparkOutput.seed}
          <blockquote><strong>Seed:</strong> {spec.sparkOutput.seed}</blockquote>
        {/if}
        {#if spec.sparkOutput.signal}
          <blockquote><strong>Signal:</strong> {spec.sparkOutput.signal}</blockquote>
        {/if}
        {#if scopeSniffLabel(spec.sparkOutput.scopeSniff)}
          <p><strong>Scope Sniff:</strong> {scopeSniffLabel(spec.sparkOutput.scopeSniff)}</p>
        {/if}
        {#if spec.sparkOutput.killTest}
          <p><strong>Kill Test:</strong> {spec.sparkOutput.killTest}</p>
        {/if}
        {#if spec.sparkOutput.questions.length > 0}
          <p><strong>Questions:</strong></p>
          <ul>{#each spec.sparkOutput.questions as q}<li>{q}</li>{/each}</ul>
        {/if}
      </AccordionSection>
    {/if}

    {#if spec.shapeOutput}
      <AccordionSection title="Shape" expanded={shouldExpand('shape')}>
        {#if spec.shapeOutput.scopeIn.length > 0}
          <p><strong>Scope In:</strong></p>
          <ul>{#each spec.shapeOutput.scopeIn as s}<li>{s}</li>{/each}</ul>
        {/if}
        {#if spec.shapeOutput.scopeOut.length > 0}
          <p><strong>Scope Out:</strong></p>
          <ul>{#each spec.shapeOutput.scopeOut as s}<li>{s}</li>{/each}</ul>
        {/if}
        {#if spec.shapeOutput.approaches.length > 0}
          <h3>Approaches</h3>
          {#each spec.shapeOutput.approaches as approach}
            <div class="approach" class:chosen={approach.name === spec.shapeOutput?.chosenApproach}>
              <strong>{approach.name}</strong>
              {#if approach.name === spec.shapeOutput?.chosenApproach}<span class="chosen-badge">chosen</span>{/if}
              {#if approach.description}<p>{approach.description}</p>{/if}
              {#if approach.tradeoffs.length > 0}
                <ul class="tradeoffs">{#each approach.tradeoffs as t}<li>{t}</li>{/each}</ul>
              {/if}
            </div>
          {/each}
        {/if}
        {#if spec.shapeOutput.risks.length > 0}
          <h3>Risks</h3>
          <ul>{#each spec.shapeOutput.risks as r}<li>{r}</li>{/each}</ul>
        {/if}
        {#if spec.shapeOutput.successMust.length > 0}
          <h3>Success Criteria</h3>
          <p><strong>Must:</strong></p>
          <ul>{#each spec.shapeOutput.successMust as s}<li>{s}</li>{/each}</ul>
        {/if}
        {#if spec.shapeOutput.successShould.length > 0}
          <p><strong>Should:</strong></p>
          <ul>{#each spec.shapeOutput.successShould as s}<li>{s}</li>{/each}</ul>
        {/if}
        {#if spec.shapeOutput.successWont.length > 0}
          <p><strong>Won't:</strong></p>
          <ul>{#each spec.shapeOutput.successWont as s}<li>{s}</li>{/each}</ul>
        {/if}
        {#if spec.shapeOutput.decisions.length > 0}
          <h3>Decisions</h3>
          <ul>
            {#each spec.shapeOutput.decisions as d}
              <li>
                {#if d.slug}<a href="/decision/{d.slug}">{d.title || d.slug}</a>{:else}{d.title}{/if}
                {#if d.rationale} — {d.rationale}{/if}
              </li>
            {/each}
          </ul>
        {/if}
      </AccordionSection>
    {/if}

    {#if spec.specifyOutput}
      <AccordionSection title="Specify" expanded={shouldExpand('specify')}>
        {#if spec.specifyOutput.interfaces.length > 0}
          <h3>Interfaces</h3>
          {#each spec.specifyOutput.interfaces as iface}
            <div class="interface-section">
              <strong>{iface.name}</strong>
              <pre>{iface.body}</pre>
            </div>
          {/each}
        {/if}
        {#if spec.specifyOutput.verifyCriteria.length > 0}
          <h3>Verify Criteria</h3>
          <table class="detail-table">
            <thead><tr><th>Category</th><th>Description</th></tr></thead>
            <tbody>
              {#each spec.specifyOutput.verifyCriteria as vc}
                <tr><td>{vc.category}</td><td>{vc.description}</td></tr>
              {/each}
            </tbody>
          </table>
        {/if}
        {#if spec.specifyOutput.invariants.length > 0}
          <h3>Invariants</h3>
          <ul>{#each spec.specifyOutput.invariants as inv}<li>{inv}</li>{/each}</ul>
        {/if}
        {#if spec.specifyOutput.touches.length > 0}
          <h3>File Touches</h3>
          <table class="detail-table">
            <thead><tr><th>Path</th><th>Purpose</th><th>Action</th></tr></thead>
            <tbody>
              {#each spec.specifyOutput.touches as ft}
                <tr><td><code>{ft.path}</code></td><td>{ft.purpose}</td><td>{ft.changeType}</td></tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </AccordionSection>
    {/if}

    {#if spec.decomposeOutput}
      <AccordionSection title="Decompose" badge="{spec.decomposeOutput.slices.length} slices" expanded={shouldExpand('decompose')}>
        {#if strategyLabel(spec.decomposeOutput.strategy)}
          <p><strong>Strategy:</strong> {strategyLabel(spec.decomposeOutput.strategy)}</p>
        {/if}
        {#each spec.decomposeOutput.slices as slice}
          <div class="slice-card">
            <strong>{slice.id}</strong>
            {#if slice.intent}<p>{slice.intent}</p>{/if}
            {#if slice.verify.length > 0}
              <p class="slice-label">Verify:</p>
              <ul>{#each slice.verify as v}<li>{v}</li>{/each}</ul>
            {/if}
            {#if slice.dependsOn.length > 0}
              <p class="slice-label">Depends on: {slice.dependsOn.join(', ')}</p>
            {/if}
          </div>
        {/each}
      </AccordionSection>
    {/if}

    {#if edges.length > 0}
      <AccordionSection title="Edges" badge="{edges.length}">
        {#each Object.entries(groupedEdges) as [label, targets]}
          <p><strong>{label}:</strong></p>
          <ul>
            {#each targets as target}
              <li><a href="/spec/{target}">{target}</a></li>
            {/each}
          </ul>
        {/each}
      </AccordionSection>
    {/if}

    {#if spec.conversationLogs.length > 0}
      <AccordionSection title="Conversations" badge="{spec.conversationLogs.length}">
        {#each spec.conversationLogs as log}
          <div class="conversation-log">
            <h4>{log.stage} (v{log.version}{log.isAmend ? ', amend' : ''})</h4>
            {#each log.exchanges as ex}
              <div class="exchange">
                <span class="role" class:probe={ex.role === 'probe'} class:response={ex.role === 'response'}>
                  {ex.role === 'probe' ? 'Probe' : 'User'}:
                </span>
                <span>{ex.content}</span>
                {#if ex.decisionPoint}<span class="decision-marker">decision</span>{/if}
              </div>
            {/each}
          </div>
        {/each}
      </AccordionSection>
    {/if}
  </div>
```

- [ ] **Step 2: Add styles for the new sections**

Append to the `<style>` block:

```css
  .sections {
    margin-top: 1rem;
  }

  blockquote {
    margin: 0.5rem 0;
    padding: 0.5rem 0.75rem;
    border-left: 3px solid #e2e8f0;
    color: #475569;
    font-size: 0.9rem;
  }

  h3 {
    font-size: 0.9rem;
    font-weight: 600;
    color: #374151;
    margin: 0.75rem 0 0.25rem;
  }

  h4 {
    font-size: 0.85rem;
    font-weight: 600;
    color: #475569;
    margin: 0.5rem 0 0.25rem;
  }

  ul {
    margin: 0.25rem 0 0.5rem;
    padding-left: 1.25rem;
  }

  li {
    font-size: 0.9rem;
    margin-bottom: 0.15rem;
  }

  .approach {
    padding: 0.5rem;
    margin: 0.25rem 0;
    border-radius: 4px;
    background: #f8fafc;
  }

  .approach.chosen {
    background: #eff6ff;
    border-left: 3px solid #2563eb;
  }

  .chosen-badge {
    font-size: 0.7rem;
    background: #2563eb;
    color: white;
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
    margin-left: 0.5rem;
  }

  .tradeoffs {
    font-size: 0.85rem;
    color: #64748b;
  }

  .interface-section {
    margin: 0.5rem 0;
  }

  pre {
    background: #f8fafc;
    padding: 0.5rem;
    border-radius: 4px;
    font-size: 0.8rem;
    overflow-x: auto;
    white-space: pre-wrap;
  }

  .detail-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    margin: 0.25rem 0 0.5rem;
  }

  .detail-table th {
    text-align: left;
    padding: 0.3rem 0.5rem;
    background: #f1f5f9;
    color: #475569;
    font-weight: 600;
  }

  .detail-table td {
    padding: 0.3rem 0.5rem;
    border-bottom: 1px solid #f1f5f9;
  }

  code {
    background: #f1f5f9;
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
    font-size: 0.8rem;
  }

  .slice-card {
    padding: 0.5rem;
    margin: 0.25rem 0;
    background: #f8fafc;
    border-radius: 4px;
    border-left: 3px solid #ca8a04;
  }

  .slice-label {
    font-size: 0.85rem;
    color: #64748b;
    margin: 0.25rem 0 0;
  }

  .conversation-log {
    margin-bottom: 0.75rem;
  }

  .exchange {
    margin: 0.2rem 0;
    font-size: 0.85rem;
  }

  .role {
    font-weight: 600;
  }

  .role.probe {
    color: #7c3aed;
  }

  .role.response {
    color: #059669;
  }

  .decision-marker {
    font-size: 0.7rem;
    background: #fef3c7;
    color: #b45309;
    padding: 0.05rem 0.3rem;
    border-radius: 3px;
    margin-left: 0.3rem;
  }
```

- [ ] **Step 3: Remove old notes section**

Remove the old `{#if spec.notes}` block that was replaced (the one without AccordionSection). The new version is inside the `.sections` div.

- [ ] **Step 4: Verify build**

Run: `cd web && pnpm svelte-check && pnpm build`

Expected: Both pass.

- [ ] **Step 5: Commit**

```text
feat(web): expand spec detail page with stage outputs, edges, and conversations (spgr-zn1)
```

---

## Chunk 3: Quality Gates

### Task 4: Run full quality gates

- [ ] **Step 1: Run task check**

Run: `task check`

Expected: PASS

- [ ] **Step 2: Run task pr-prep**

Run: `task pr-prep`

Expected: PASS (includes e2e/ui tests which load the spec detail page)

- [ ] **Step 3: Fix any issues**

- [ ] **Step 4: Commit fixes if needed**

```text
fix: address lint and formatting issues (spgr-zn1)
```

---

## Summary

| Chunk | Tasks | Focus |
|-------|-------|-------|
| 1 | Task 1 | AccordionSection component |
| 2 | Tasks 2-3 | Page expansion (imports, helpers, template, styles) |
| 3 | Task 4 | Quality gates |

**Total:** 4 tasks, ~15 steps. Pure frontend — no Go/proto changes.
