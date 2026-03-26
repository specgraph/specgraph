<script lang="ts">
  import { onMount } from 'svelte';
  import { specClient, graphClient, decisionClient, lifecycleClient } from '$lib/api/client';
  import type { Spec } from '$lib/api/gen/specgraph/v1/spec_pb';
  import type { GraphNode, Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
  import type { Decision } from '$lib/api/gen/specgraph/v1/decision_pb';
  import { DecisionStatus } from '$lib/api/gen/specgraph/v1/decision_pb';
  import { EdgeType } from '$lib/api/gen/specgraph/v1/graph_pb';

  function statusLabel(status: DecisionStatus): string {
    switch (status) {
      case DecisionStatus.PROPOSED: return 'proposed';
      case DecisionStatus.ACCEPTED: return 'accepted';
      case DecisionStatus.DEPRECATED: return 'deprecated';
      case DecisionStatus.SUPERSEDED: return 'superseded';
      default: return '—';
    }
  }
  import StatsBar from '$lib/components/StatsBar.svelte';
  import FunnelBar from '$lib/components/FunnelBar.svelte';
  import GraphMini from '$lib/components/GraphMini.svelte';
  import TabBar from '$lib/components/TabBar.svelte';
  import SpecTable from '$lib/components/SpecTable.svelte';

  let totalSpecs = $state(0);
  let readyCount = $state(0);
  let driftCount = $state(0);
  let decisionCount = $state(0);
  let stageCounts = $state<Record<string, number>>({});
  let graphNodes = $state<GraphNode[]>([]);
  let graphEdges = $state<Edge[]>([]);
  let specs = $state<Spec[]>([]);
  let decisions = $state<Decision[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  let activeTab = $state('All Specs');
  const tabs = ['All Specs', 'Recent', 'By Priority', 'Decisions'];

  let recentSpecs = $derived(
    [...specs].sort((a, b) => Number(b.updatedAt?.seconds ?? 0n) - Number(a.updatedAt?.seconds ?? 0n)).slice(0, 10)
  );

  let priorityGroups = $derived(
    ['p0', 'p1', 'p2', 'p3'].map(p => ({
      label: p.toUpperCase(),
      specs: specs.filter(s => s.priority === p),
    })).filter(g => g.specs.length > 0)
  );

  let decisionSpecCounts = $derived(
    graphEdges
      .filter(e => e.edgeType === EdgeType.DECIDED_IN)
      .reduce((acc, e) => {
        acc[e.toId] = (acc[e.toId] ?? 0) + 1;
        return acc;
      }, {} as Record<string, number>)
  );

  async function loadDashboard() {
    try {
      const [specsRes, readyRes, graphRes, decisionsRes, driftRes] = await Promise.all([
        specClient.listSpecs({}),
        graphClient.getReady({}),
        graphClient.getFullGraph({}),
        decisionClient.listDecisions({}),
        lifecycleClient.checkDrift({ slug: '' }),
      ]);

      const specsList = specsRes.specs ?? [];
      specs = specsList;
      totalSpecs = specsList.length;

      const counts: Record<string, number> = {};
      for (const s of specsList) {
        counts[s.stage] = (counts[s.stage] ?? 0) + 1;
      }
      stageCounts = counts;

      readyCount = (readyRes.ready ?? []).length;
      graphNodes = graphRes.nodes ?? [];
      graphEdges = graphRes.edges ?? [];
      decisions = decisionsRes.decisions ?? [];
      decisionCount = decisions.length;

      const reports = driftRes.reports ?? [];
      driftCount = reports.filter((r) => (r.items?.length ?? 0) > 0).length;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load dashboard data';
    } finally {
      loading = false;
    }
  }

  onMount(() => { loadDashboard(); });
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
          <tr><th>Decision</th><th>Title</th><th>Status</th><th>Linked Specs</th></tr>
        </thead>
        <tbody>
          {#each decisions as d}
            <tr>
              <td><a href="/decision/{d.slug}">{d.slug}</a></td>
              <td>{d.title || '—'}</td>
              <td><span class="badge">{statusLabel(d.status)}</span></td>
              <td class="count">{decisionSpecCounts[d.slug] ?? 0}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
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

  .badge {
    display: inline-block;
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    font-size: 0.75rem;
    font-weight: 600;
    background: #f1f5f9;
    color: #475569;
  }
</style>
