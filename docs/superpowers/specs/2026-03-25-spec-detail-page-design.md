# Spec Detail Page Design Spec

**Bead:** spgr-zn1 — Spec detail page: show lifecycle stage outputs, edges, and linked decisions
**Date:** 2026-03-25

## Problem

The spec detail page (`/spec/:slug`) shows only basic metadata (intent, stage, priority, complexity, version). Users cannot see authoring stage outputs, edges, or conversations without using the CLI. The web UI should match `specgraph show` output.

## Design

Expand the existing `/spec/[...slug]/+page.svelte` with collapsible accordion sections. Data comes from `GetSpec` (stage outputs, conversations already inline) and a new `listEdges` call.

### Page Structure

Sections top to bottom. Only sections with data are rendered.

1. **Metadata** (always visible, not collapsible) — slug as H1, intent as subtitle, table with stage/priority/complexity/version/lifecycle/timestamps, supersedes/superseded_by links if present
2. **Notes** (collapsible, expanded if non-empty) — free-text notes
3. **Spark** (collapsible, auto-expand if current stage) — seed (blockquote), signal (blockquote), scope sniff, kill test, questions (bulleted)
4. **Shape** (collapsible, auto-expand if current stage) — scope in/out (bullets), chosen approach highlighted, all approaches with description/tradeoffs, risks (bullets), success criteria (must/should/won't), decisions (linked to `/decision/:slug` if slug present)
5. **Specify** (collapsible, auto-expand if current stage) — interfaces (name + body), verify criteria (table: category, description), invariants (bullets), file touches (table: path, purpose, action)
6. **Decompose** (collapsible, auto-expand if current stage) — strategy label, slices (card per slice: id, intent, verify criteria, dependencies, touches)
7. **Edges** (collapsible, collapsed by default) — fetched via `graphClient.listEdges({ slug })`. Grouped by edge type. Each edge is a link to `/spec/:slug` or `/decision/:slug`.
8. **Conversations** (collapsible, collapsed by default) — from `spec.conversationLogs`. Grouped by stage. Each log shows probe/response pairs with sequence numbers, decision point markers, and amend labels.

### New Component: AccordionSection.svelte

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

<div class="accordion-section">
  <button class="accordion-header" onclick={() => open = !open}>
    <span class="chevron" class:open>{open ? '▼' : '▶'}</span>
    <span class="title">{title}</span>
    {#if badge}<span class="badge">{badge}</span>{/if}
  </button>
  {#if open}
    <div class="accordion-body">{@render children()}</div>
  {/if}
</div>
```

Props:

- `title: string` — section heading
- `expanded: boolean` (default false) — initial state
- `badge: string` (optional) — count or label shown next to title (e.g., "3 slices")

### Expand/Collapse Logic

Auto-expand based on `spec.stage`:

| Stage value | Spark | Shape | Specify | Decompose |
|-------------|-------|-------|---------|-----------|
| spark | **expanded** | hidden | hidden | hidden |
| shape | collapsed | **expanded** | hidden | hidden |
| specify | collapsed | collapsed | **expanded** | hidden |
| decompose | collapsed | collapsed | collapsed | **expanded** |
| approved+ | collapsed | collapsed | collapsed | collapsed |

Notes: expanded if non-empty. Edges, Conversations: always collapsed initially.

### Data Sources

- **Stage outputs**: `spec.sparkOutput`, `spec.shapeOutput`, `spec.specifyOutput`, `spec.decomposeOutput` — already populated by `GetSpec` response (PR #665)
- **Conversations**: `spec.conversationLogs` — already populated by `GetSpec` response (PR #664)
- **Edges**: New call to `graphClient.listEdges({ slug })` in the page's `onMount` or reactive block. Returns `Edge[]` with `fromId`, `toId`, `edgeType` (IDs contain slugs).

### Styling

Follow the existing inline Svelte `<style>` pattern used by the current page. Key additions:

- `.accordion-header` — flexbox row, cursor pointer, padding, border-bottom
- `.accordion-body` — padding-left for indent
- `.chevron` — rotate transition
- `.badge` — small pill with count
- Stage-specific content uses the same table/list patterns as the existing metadata section

### Edge Type Display

| Edge Type | Display Name | Icon/Color |
|-----------|-------------|------------|
| DEPENDS_ON | Depends on | — |
| BLOCKS | Blocks | — |
| COMPOSES | Composes | — |
| RELATES_TO | Relates to | — |
| INFORMS | Informs | — |
| DECIDED_IN | Decision | — |
| SUPERSEDES | Supersedes | — |

Each edge renders as: `[Display Name]: [target slug]` with the slug as a link.

## Files Changed

| File | Action | What |
|------|--------|------|
| `web/src/lib/components/AccordionSection.svelte` | Create | Reusable collapsible section component |
| `web/src/routes/spec/[...slug]/+page.svelte` | Modify | Add accordion sections for all stage outputs, edges, conversations |

## Testing

- **Manual**: Navigate to `/spec/:slug` for a spec that has been through spark/shape/specify. Verify each section renders correctly and collapse/expand works.
- **E2E (existing)**: The existing e2e/ui tests verify the spec detail page loads. They don't check stage output content (no specs with stage data in the test fixture), so no e2e changes needed.
- **Build verification**: `pnpm svelte-check` and `pnpm build` must pass.

## What's NOT in scope

- No edit capabilities (read-only V1)
- No mini-graph visualization in edges section (just linked list)
- No new API endpoints or RPCs
- No SSR changes
- No decision detail page expansion (separate bead)
