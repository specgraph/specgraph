<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import Graph from '$lib/components/Graph.svelte';
  import SearchFilter from '$lib/components/SearchFilter.svelte';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  // Local UI state only — the fetch moved to +page.ts load().
  let filterText = $state('');
</script>

<h1 class="mb-4 text-xl font-semibold text-foreground">Dependency Graph</h1>

{#await data.graph}
  <!-- Loading: Skeleton Card block (graph canvas footprint). Streamed promise
       re-suspends here on invalidateAll() so a switch returns to skeleton with no
       stale previous-project graph (Pitfall 3, T-05-05). -->
  <Card.Root>
    <Card.Content class="p-4">
      <Skeleton class="h-[480px] w-full" />
    </Card.Content>
  </Card.Root>
{:then g}
  {#if g.loadError}
    <!-- Error: inline Retry card (do not reach +error.svelte, T-05-15). -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>Couldn't load graph.</Card.Title>
        <Card.Description>Check your connection and try again.</Card.Description>
      </Card.Header>
      <Card.Footer>
        <Button variant="outline" onclick={() => invalidateAll()}>Retry</Button>
      </Card.Footer>
    </Card.Root>
  {:else if g.nodes.length === 0}
    <!-- Empty: no graph nodes for this project (UI-SPEC copy). -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>Nothing here yet</Card.Title>
        <Card.Description>Nothing here yet — no graph nodes for this project.</Card.Description>
      </Card.Header>
    </Card.Root>
  {:else}
    <SearchFilter value={filterText} onchange={(v) => (filterText = v)} />
    <div class="mt-3">
      <Graph nodes={g.nodes} edges={g.edges} {filterText} />
    </div>
  {/if}
{/await}
