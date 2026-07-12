<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import type { Spec } from '$lib/api/gen/specgraph/v1/spec_pb';
  import type { Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
  import { DecisionStatus } from '$lib/api/gen/specgraph/v1/decision_pb';
  import { EdgeType } from '$lib/api/gen/specgraph/v1/graph_pb';
  import StatsBar from '$lib/components/StatsBar.svelte';
  import FunnelBar from '$lib/components/FunnelBar.svelte';
  import GraphMini from '$lib/components/GraphMini.svelte';
  import TabBar from '$lib/components/TabBar.svelte';
  import SpecTable from '$lib/components/SpecTable.svelte';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  let activeTab = $state('All Specs');
  const tabs = ['All Specs', 'Recent', 'By Priority', 'Decisions'];

  function statusLabel(status: DecisionStatus): string {
    switch (status) {
      case DecisionStatus.PROPOSED: return 'proposed';
      case DecisionStatus.ACCEPTED: return 'accepted';
      case DecisionStatus.DEPRECATED: return 'deprecated';
      case DecisionStatus.SUPERSEDED: return 'superseded';
      default: return '\u2014';
    }
  }

  function recentSpecs(specs: Spec[]): Spec[] {
    return [...specs]
      .sort((a, b) => Number(b.updatedAt?.seconds ?? 0n) - Number(a.updatedAt?.seconds ?? 0n))
      .slice(0, 10);
  }

  function priorityGroups(specs: Spec[]): { label: string; specs: Spec[] }[] {
    return ['p0', 'p1', 'p2', 'p3']
      .map((p) => ({ label: p.toUpperCase(), specs: specs.filter((s) => s.priority === p) }))
      .filter((g) => g.specs.length > 0);
  }

  function decisionSpecCounts(edges: Edge[]): Record<string, number> {
    return edges
      .filter((e) => e.edgeType === EdgeType.DECIDED_IN)
      .reduce((acc, e) => {
        acc[e.toId] = (acc[e.toId] ?? 0) + 1;
        return acc;
      }, {} as Record<string, number>);
  }
</script>

<h1 class="mb-5 text-2xl font-semibold text-foreground">Dashboard</h1>

{#await data.dashboard}
  <!-- Loading: Skeleton stat cards + table rows (State Matrix). Streamed promise
       re-suspends here on invalidateAll() so a switch returns to skeletons with
       no stale previous-project data (Pitfall 3, T-05-05). -->
  <section class="flex flex-col gap-5">
    <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
      {#each Array(6) as _}
        <Card.Root>
          <Card.Content class="p-4">
            <Skeleton class="mb-2 h-3 w-16" />
            <Skeleton class="h-7 w-12" />
          </Card.Content>
        </Card.Root>
      {/each}
    </div>
    <div class="grid grid-cols-1 gap-5 md:grid-cols-2">
      <Skeleton class="h-40 w-full" />
      <Skeleton class="h-40 w-full" />
    </div>
    <div class="flex flex-col gap-2">
      {#each Array(6) as _}
        <Skeleton class="h-8 w-full" />
      {/each}
    </div>
  </section>
{:then d}
  {#if d.loadError}
    <!-- Error: inline Retry card (do not reach +error.svelte, T-05-15). -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>Couldn't load dashboard.</Card.Title>
        <Card.Description>Check your connection and try again.</Card.Description>
      </Card.Header>
      <Card.Footer>
        <Button variant="outline" onclick={() => invalidateAll()}>Retry</Button>
      </Card.Footer>
    </Card.Root>
  {:else if d.totalSpecs === 0 && d.decisionCount === 0}
    <!-- Empty: zero specs & decisions (UI-SPEC copy). -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>Nothing here yet</Card.Title>
        <Card.Description>This project has no specs or decisions to show.</Card.Description>
      </Card.Header>
    </Card.Root>
  {:else}
    <section class="flex flex-col gap-5">
      <StatsBar
        totalSpecs={d.totalSpecs}
        readyCount={d.readyCount}
        driftCount={d.driftCount}
        decisionCount={d.decisionCount}
        amendedCount={d.stageCounts['amended'] ?? 0}
        supersededCount={d.stageCounts['superseded'] ?? 0}
      />

      <div class="grid grid-cols-1 gap-5 md:grid-cols-2">
        <div>
          <FunnelBar stageCounts={d.stageCounts} />
        </div>
        <div>
          <GraphMini nodes={d.nodes} edges={d.edges} />
        </div>
      </div>
    </section>

    <section class="mt-5">
      <TabBar {tabs} active={activeTab} onchange={(t) => (activeTab = t)} />

      {#if activeTab === 'All Specs'}
        <SpecTable specs={d.specs} />
      {:else if activeTab === 'Recent'}
        <SpecTable specs={recentSpecs(d.specs)} />
      {:else if activeTab === 'By Priority'}
        {#each priorityGroups(d.specs) as group}
          <h3 class="mt-4 mb-1 text-sm font-semibold text-muted-foreground">
            {group.label}
            <span class="font-normal text-muted-foreground/70">({group.specs.length})</span>
          </h3>
          <SpecTable specs={group.specs} showConversations={false} />
        {/each}
      {:else if activeTab === 'Decisions'}
        {@const counts = decisionSpecCounts(d.edges)}
        <table class="w-full border-collapse text-sm">
          <thead>
            <tr>
              <th class="border-b border-border bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Decision</th>
              <th class="border-b border-border bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Title</th>
              <th class="border-b border-border bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Status</th>
              <th class="border-b border-border bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Linked Specs</th>
            </tr>
          </thead>
          <tbody>
            {#each d.decisions as dec}
              <tr>
                <td class="border-b border-border/50 px-2 py-1.5">
                  <a href="/decision/{dec.slug}" class="font-medium text-primary hover:underline">{dec.slug}</a>
                </td>
                <td class="border-b border-border/50 px-2 py-1.5">{dec.title || '\u2014'}</td>
                <td class="border-b border-border/50 px-2 py-1.5">
                  <span class="inline-block rounded bg-muted px-1.5 py-0.5 text-xs font-semibold text-muted-foreground">
                    {statusLabel(dec.status)}
                  </span>
                </td>
                <td class="border-b border-border/50 px-2 py-1.5 text-center text-muted-foreground">{counts[dec.slug] ?? 0}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </section>
  {/if}
{/await}
