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
