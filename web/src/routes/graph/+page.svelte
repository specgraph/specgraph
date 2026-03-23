<script lang="ts">
  import { graphClient } from '$lib/api/client';
  import type { GraphNode, Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
  import Graph from '$lib/components/Graph.svelte';
  import SearchFilter from '$lib/components/SearchFilter.svelte';

  let nodes = $state<GraphNode[]>([]);
  let edges = $state<Edge[]>([]);
  let filterText = $state('');
  let loading = $state(true);
  let error = $state<string | null>(null);
  let loaded = false;

  $effect(() => {
    if (!loaded) {
      loaded = true;
      loadGraph();
    }
  });

  async function loadGraph() {
    try {
      const resp = await graphClient.getFullGraph({});
      nodes = resp.nodes ?? [];
      edges = resp.edges ?? [];
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load graph';
      console.error('Failed to load graph:', err);
    } finally {
      loading = false;
    }
  }
</script>

<h1>Dependency Graph</h1>

{#if loading}
  <p class="status">Loading graph...</p>
{:else if error}
  <p class="status error">{error}</p>
{:else if nodes.length === 0}
  <p class="status">No specs or decisions found. Create some specs first.</p>
{:else}
  <SearchFilter value={filterText} onchange={(v) => (filterText = v)} />
  <div class="graph-wrap">
    <Graph {nodes} {edges} {filterText} />
  </div>
{/if}

<style>
  h1 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0 0 1rem;
    color: #1a1a2e;
  }

  .status {
    color: #64748b;
    font-size: 0.95rem;
  }

  .status.error {
    color: #dc2626;
  }

  .graph-wrap {
    margin-top: 0.75rem;
  }
</style>
