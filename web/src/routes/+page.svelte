<script lang="ts">
  import { specClient, graphClient, decisionClient, lifecycleClient } from '$lib/api/client';
  import type { GraphNode, Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
  import StatsBar from '$lib/components/StatsBar.svelte';
  import FunnelBar from '$lib/components/FunnelBar.svelte';
  import GraphMini from '$lib/components/GraphMini.svelte';

  let totalSpecs = $state(0);
  let readyCount = $state(0);
  let driftCount = $state(0);
  let decisionCount = $state(0);
  let stageCounts = $state<Record<string, number>>({});
  let graphNodes = $state<GraphNode[]>([]);
  let graphEdges = $state<Edge[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  async function loadDashboard() {
    try {
      const [specsRes, readyRes, graphRes, decisionsRes, driftRes] = await Promise.all([
        specClient.listSpecs({}),
        graphClient.getReady({}),
        graphClient.getFullGraph({}),
        decisionClient.listDecisions({}),
        lifecycleClient.checkDrift({ slug: '' }),
      ]);

      const specs = specsRes.specs ?? [];
      totalSpecs = specs.length;

      const counts: Record<string, number> = {};
      for (const s of specs) {
        counts[s.stage] = (counts[s.stage] ?? 0) + 1;
      }
      stageCounts = counts;

      readyCount = (readyRes.ready ?? []).length;
      graphNodes = graphRes.nodes ?? [];
      graphEdges = graphRes.edges ?? [];
      decisionCount = (decisionsRes.decisions ?? []).length;

      const reports = driftRes.reports ?? [];
      driftCount = reports.filter((r) => (r.items?.length ?? 0) > 0).length;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load dashboard data';
    } finally {
      loading = false;
    }
  }

  $effect(() => { loadDashboard(); });
</script>

<h1>Dashboard</h1>

{#if loading}
  <p class="status">Loading...</p>
{:else if error}
  <p class="status error">Error: {error}</p>
{:else}
  <section class="dashboard">
    <StatsBar {totalSpecs} {readyCount} {driftCount} {decisionCount} />

    <div class="row">
      <div class="col-funnel">
        <FunnelBar {stageCounts} />
      </div>
      <div class="col-graph">
        <GraphMini nodes={graphNodes} edges={graphEdges} />
      </div>
    </div>
  </section>
{/if}

<style>
  h1 {
    margin: 0 0 1.25rem;
    font-size: 1.5rem;
    color: #1a1a2e;
  }

  .status {
    color: #64748b;
    font-size: 0.95rem;
  }

  .status.error {
    color: #dc2626;
  }

  .dashboard {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.25rem;
  }

  @media (max-width: 800px) {
    .row {
      grid-template-columns: 1fr;
    }
  }
</style>
