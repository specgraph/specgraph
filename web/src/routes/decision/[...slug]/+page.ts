// Decision detail universal load() (D-01/D-02/D-09). Moves the getDecision fetch
// out of the component onMount so a project switch's invalidateAll() re-runs it
// with the new X-Specgraph-Project header — keyed on params.slug (Pitfall 6: await
// parent() resolves the project default first). Keying on params.slug ALSO fixes
// the pre-existing slug-nav bug: the old onMount(() => loadDecision(slug)) never
// re-ran on an in-project slug change (review LOW). The RPC is streamed (the
// returned `detail` promise is NOT awaited) so {#await} in +page.svelte re-suspends
// to the Skeleton state on every invalidateAll() (Pitfall 3: $navigating is NOT set
// on invalidate). A NotFound RPC error becomes the `notFound` empty-state sentinel;
// any other error becomes a `loadError` for the inline Retry card, never
// +error.svelte (RESEARCH L279, T-05-15). The non-critical linkedSpecs fetch is
// try/caught with an [] fallback so its failure never loses the primary decision.
import type { PageLoad } from './$types';
import { ConnectError, Code } from '@connectrpc/connect';
import { decisionClient, graphClient } from '$lib/api/client';
import type { Decision } from '$lib/api/gen/specgraph/v1/decision_pb';
import { EdgeType } from '$lib/api/gen/specgraph/v1/graph_pb';

export interface DecisionDetailData {
  decision: Decision | null;
  linkedSpecs: string[];
  notFound: boolean;
  loadError: string | null;
}

async function loadDecisionDetail(slug: string): Promise<DecisionDetailData> {
  let decision: Decision | null;
  try {
    const resp = await decisionClient.getDecision({ slug });
    decision = resp.decision ?? null;
  } catch (e) {
    // NotFound → empty state; anything else → inline Retry error card (T-05-15).
    if (e instanceof ConnectError && e.code === Code.NotFound) {
      return { decision: null, linkedSpecs: [], notFound: true, loadError: null };
    }
    return {
      decision: null,
      linkedSpecs: [],
      notFound: false,
      loadError: e instanceof Error ? e.message : 'Failed to load decision',
    };
  }

  if (!decision) {
    return { decision: null, linkedSpecs: [], notFound: true, loadError: null };
  }

  // Linked specs are non-critical — try/caught with [] fallback so a failure
  // never loses the primary decision (mirror the pre-load L31-37 filter).
  let linkedSpecs: string[] = [];
  try {
    const edgeResp = await graphClient.listEdges({ slug });
    linkedSpecs = edgeResp.edges
      .filter((e) => e.edgeType === EdgeType.DECIDED_IN && e.toId === slug)
      .map((e) => e.fromId);
  } catch {
    linkedSpecs = [];
  }

  return { decision, linkedSpecs, notFound: false, loadError: null };
}

export const load: PageLoad = async ({ params, parent, depends }) => {
  await parent(); // resolve +layout.ts project default before RPCs issue (Pitfall 6)
  depends('app:project'); // cheap targeted-invalidate insurance (D-01 tradeoff)
  // Stream the promise (do NOT await) so {#await} re-suspends on invalidateAll().
  return { detail: loadDecisionDetail(params.slug) };
};
