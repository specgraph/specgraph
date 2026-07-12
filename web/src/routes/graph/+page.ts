// Graph universal load() (D-01/D-02/D-09). Moves the graph fetch out of the
// component onMount so a project switch's invalidateAll() re-runs it with the new
// X-Specgraph-Project header (Pitfall 6: await parent() first). The RPC is
// streamed (not awaited here) so {#await} in graph/+page.svelte re-suspends to
// the Skeleton state on invalidateAll() (Pitfall 3: $navigating is NOT set). The
// then-block hands dagre the fully-resolved node/edge set, so the layout stays
// correct. Errors are caught INSIDE the promise as a loadError sentinel to render
// an inline Retry card rather than +error.svelte (RESEARCH L279, T-05-15).
import type { PageLoad } from './$types';
import { graphClient } from '$lib/api/client';
import type { GraphNode, Edge } from '$lib/api/gen/specgraph/v1/graph_pb';

export interface GraphData {
  loadError: string | null;
  nodes: GraphNode[];
  edges: Edge[];
}

async function loadGraphData(): Promise<GraphData> {
  try {
    const resp = await graphClient.getFullGraph({});
    return { loadError: null, nodes: resp.nodes ?? [], edges: resp.edges ?? [] };
  } catch (e) {
    return {
      loadError: e instanceof Error ? e.message : 'Failed to load graph',
      nodes: [],
      edges: [],
    };
  }
}

export const load: PageLoad = async ({ depends, parent }) => {
  await parent(); // resolve +layout.ts project default before RPCs issue (Pitfall 6)
  depends('app:project'); // cheap targeted-invalidate insurance (D-01 tradeoff)
  // Stream the promise (do NOT await) so {#await} re-suspends on invalidateAll().
  return { graph: loadGraphData() };
};
