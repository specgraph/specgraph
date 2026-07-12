<script lang="ts">
  import type { Spec } from '$lib/api/gen/specgraph/v1/spec_pb';
  import * as Table from '$lib/components/ui/table/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { stageBadgeClass } from './badge-variants';

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

  function ariaSort(key: typeof sortKey): 'ascending' | 'descending' | 'none' {
    if (sortKey !== key) return 'none';
    return sortAsc ? 'ascending' : 'descending';
  }

  function sortIndicator(key: typeof sortKey): string {
    if (sortKey !== key) return '';
    return sortAsc ? '↑' : '↓';
  }
</script>

<Table.Root class="text-sm">
  <Table.Header>
    <Table.Row class="bg-muted">
      <Table.Head aria-sort={ariaSort('slug')}>
        <button type="button" class="w-full text-left font-medium hover:text-primary focus-visible:outline-2 focus-visible:outline-ring focus-visible:outline-offset-1 rounded-sm" onclick={() => toggleSort('slug')}>Spec {sortIndicator('slug')}</button>
      </Table.Head>
      <Table.Head aria-sort={ariaSort('stage')}>
        <button type="button" class="w-full text-left font-medium hover:text-primary focus-visible:outline-2 focus-visible:outline-ring focus-visible:outline-offset-1 rounded-sm" onclick={() => toggleSort('stage')}>Stage {sortIndicator('stage')}</button>
      </Table.Head>
      <Table.Head aria-sort={ariaSort('priority')}>
        <button type="button" class="w-full text-left font-medium hover:text-primary focus-visible:outline-2 focus-visible:outline-ring focus-visible:outline-offset-1 rounded-sm" onclick={() => toggleSort('priority')}>Pri {sortIndicator('priority')}</button>
      </Table.Head>
      {#if showConversations}<Table.Head class="text-center">💬</Table.Head>{/if}
      <Table.Head>Intent</Table.Head>
      <Table.Head aria-sort={ariaSort('updated')}>
        <button type="button" class="w-full text-left font-medium hover:text-primary focus-visible:outline-2 focus-visible:outline-ring focus-visible:outline-offset-1 rounded-sm" onclick={() => toggleSort('updated')}>Updated {sortIndicator('updated')}</button>
      </Table.Head>
    </Table.Row>
  </Table.Header>
  <Table.Body>
    {#each rows as spec}
      <Table.Row>
        <Table.Cell>
          <a href="/spec/{spec.slug}" class="font-medium text-primary hover:underline">{spec.slug}</a>
        </Table.Cell>
        <Table.Cell>
          <Badge class={stageBadgeClass(spec.stage)}>{spec.stage}</Badge>
        </Table.Cell>
        <Table.Cell>
          {#if spec.priority}
            <Badge variant="secondary">{spec.priority}</Badge>
          {:else}
            <span class="text-muted-foreground">—</span>
          {/if}
        </Table.Cell>
        {#if showConversations}<Table.Cell class="text-center text-muted-foreground">{spec.conversationCount}</Table.Cell>{/if}
        <Table.Cell class="max-w-[300px] overflow-hidden text-ellipsis whitespace-nowrap">{spec.intent}</Table.Cell>
        <Table.Cell class="whitespace-nowrap text-muted-foreground">{relativeTime(spec.updatedAt)}</Table.Cell>
      </Table.Row>
    {/each}
    {#if rows.length === 0}
      <Table.Row>
        <Table.Cell colspan={showConversations ? 6 : 5} class="py-6 text-center text-muted-foreground">No specs found</Table.Cell>
      </Table.Row>
    {/if}
  </Table.Body>
</Table.Root>
