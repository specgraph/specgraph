// Dashboard universal load() (D-01/D-02/D-09). Replaces the in-component
// onMount fetch so a project switch's invalidateAll() re-runs this with the new
// X-Specgraph-Project header (Pitfall 6: await parent() resolves the project
// default first). The RPCs are kicked off but NOT awaited here — the returned
// `dashboard` promise is streamed so {#await} in +page.svelte re-suspends to the
// Skeleton state on every invalidateAll() (Pitfall 3: $navigating is NOT set on
// invalidate). Errors are caught INSIDE the promise and surfaced as a loadError
// sentinel so they render an inline Retry card, not +error.svelte (RESEARCH L279).
import type { PageLoad } from './$types';
import { specClient, graphClient, decisionClient, lifecycleClient } from '$lib/api/client';
import type { Spec } from '$lib/api/gen/specgraph/v1/spec_pb';
import type { GraphNode, Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
import type { Decision } from '$lib/api/gen/specgraph/v1/decision_pb';

export interface DashboardData {
  loadError: string | null;
  specs: Spec[];
  totalSpecs: number;
  readyCount: number;
  driftCount: number;
  decisionCount: number;
  stageCounts: Record<string, number>;
  nodes: GraphNode[];
  edges: Edge[];
  decisions: Decision[];
}

function emptyDashboard(loadError: string | null): DashboardData {
  return {
    loadError,
    specs: [],
    totalSpecs: 0,
    readyCount: 0,
    driftCount: 0,
    decisionCount: 0,
    stageCounts: {},
    nodes: [],
    edges: [],
    decisions: [],
  };
}

async function loadDashboardData(): Promise<DashboardData> {
  try {
    const [specsRes, readyRes, graphRes, decisionsRes, driftRes] = await Promise.all([
      specClient.listSpecs({}),
      graphClient.getReady({}),
      graphClient.getFullGraph({}),
      decisionClient.listDecisions({}),
      lifecycleClient.checkDrift({ slug: '' }),
    ]);

    const specs = specsRes.specs ?? [];
    const stageCounts: Record<string, number> = {};
    for (const s of specs) {
      stageCounts[s.stage] = (stageCounts[s.stage] ?? 0) + 1;
    }

    const decisions = decisionsRes.decisions ?? [];
    const reports = driftRes.reports ?? [];

    return {
      loadError: null,
      specs,
      totalSpecs: specs.length,
      readyCount: (readyRes.ready ?? []).length,
      driftCount: reports.filter((r) => (r.items?.length ?? 0) > 0).length,
      decisionCount: decisions.length,
      stageCounts,
      nodes: graphRes.nodes ?? [],
      edges: graphRes.edges ?? [],
      decisions,
    };
  } catch (e) {
    return emptyDashboard(e instanceof Error ? e.message : 'Failed to load dashboard data');
  }
}

export const load: PageLoad = async ({ depends, parent }) => {
  await parent(); // resolve +layout.ts project default before RPCs issue (Pitfall 6)
  depends('app:project'); // cheap targeted-invalidate insurance (D-01 tradeoff)
  // Stream the promise (do NOT await) so {#await} re-suspends on invalidateAll().
  return { dashboard: loadDashboardData() };
};
