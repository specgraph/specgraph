// Spec detail universal load() (D-01/D-02/D-09). Moves the getSpec fetch out of
// the component onMount so a project switch's invalidateAll() re-runs it with the
// new X-Specgraph-Project header, and a param-only slug change re-runs it too —
// keyed on params.slug (Pitfall 6: await parent() resolves the project default
// first). This replaces the manual `activeSlug` stale-guard (T-05-05): load-driven
// data can never retain a prior-project/prior-slug spec. The RPC is streamed (the
// returned `detail` promise is NOT awaited) so {#await} in +page.svelte re-suspends
// to the Skeleton state on every invalidateAll() (Pitfall 3: $navigating is NOT set
// on invalidate). A NotFound RPC error becomes the `notFound` empty-state sentinel;
// any other error becomes a `loadError` for the inline Retry card, never
// +error.svelte (RESEARCH L279, T-05-15). The three non-critical secondary fetches
// (edges, findings, slices) are each individually try/caught with an [] fallback so
// a secondary failure never loses the primary spec.
import type { PageLoad } from './$types';
import { ConnectError, Code } from '@connectrpc/connect';
import { specClient, graphClient, analyticalPassClient, sliceClient } from '$lib/api/client';
import type { Spec } from '$lib/api/gen/specgraph/v1/spec_pb';
import type { Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
import type { AnalyticalFinding } from '$lib/api/gen/specgraph/v1/analytical_pass_pb';
import type { Slice } from '$lib/api/gen/specgraph/v1/slice_pb';

export interface SpecDetailData {
  spec: Spec | null;
  edges: Edge[];
  findings: AnalyticalFinding[];
  slices: Slice[];
  notFound: boolean;
  loadError: string | null;
}

async function loadSpecDetail(slug: string): Promise<SpecDetailData> {
  let spec: Spec | null;
  try {
    const resp = await specClient.getSpec({ slug });
    spec = resp.spec ?? null;
  } catch (e) {
    // NotFound → empty state; anything else → inline Retry error card (T-05-15).
    if (e instanceof ConnectError && e.code === Code.NotFound) {
      return { spec: null, edges: [], findings: [], slices: [], notFound: true, loadError: null };
    }
    return {
      spec: null,
      edges: [],
      findings: [],
      slices: [],
      notFound: false,
      loadError: e instanceof Error ? e.message : 'Failed to load spec',
    };
  }

  if (!spec) {
    return { spec: null, edges: [], findings: [], slices: [], notFound: true, loadError: null };
  }

  // Non-critical secondary fetches — each try/caught with [] fallback so a
  // failure never loses the primary spec (mirror the pre-load L42-63 pattern).
  let edges: Edge[] = [];
  try {
    edges = (await graphClient.listEdges({ slug })).edges ?? [];
  } catch {
    edges = [];
  }

  let findings: AnalyticalFinding[] = [];
  try {
    findings = (await analyticalPassClient.listFindings({ slug })).findings ?? [];
  } catch {
    findings = [];
  }

  let slices: Slice[] = [];
  try {
    if (spec.decomposeOutput) {
      slices = (await sliceClient.listSlices({ parentSlug: slug })).slices ?? [];
    }
  } catch {
    slices = [];
  }

  return { spec, edges, findings, slices, notFound: false, loadError: null };
}

export const load: PageLoad = async ({ params, parent, depends }) => {
  await parent(); // resolve +layout.ts project default before RPCs issue (Pitfall 6)
  depends('app:project'); // cheap targeted-invalidate insurance (D-01 tradeoff)
  // Stream the promise (do NOT await) so {#await} re-suspends on invalidateAll().
  return { detail: loadSpecDetail(params.slug) };
};
