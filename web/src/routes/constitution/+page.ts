// Constitution universal load() (D-10/D-01/D-02/D-09). Moves the getConstitution
// fetch out of the component-body $effect (the exact anti-pattern that leaked
// stale layer badges/sections across a project switch — RESEARCH Pitfall 4) so a
// switch's invalidateAll() re-runs it with the new X-Specgraph-Project header
// (Pitfall 6: await parent() resolves the +layout.ts project default first). The
// RPC is streamed — each field is returned as a promise off the shared RPC call,
// NOT awaited here — so a $derived Promise.all in +page.svelte re-suspends {#await}
// to the Skeleton state on every invalidateAll() (Pitfall 3: $navigating is NOT
// set on invalidate). The RAW `provenance` array is returned (not pre-derived
// badges) so the component derives every badge from it — on reload the
// badges/empty-state re-derive with no retained prior-project provenance (D-10 /
// T-05-05, Pitfall 4). Errors are caught INSIDE the promise and surfaced as a
// `loadError` sentinel so they render an inline Retry card, never +error.svelte
// (RESEARCH L279, T-05-15).
import type { PageLoad } from './$types';
import { constitutionClient } from '$lib/api/client';
import type { Constitution, ProvenanceEntry } from '$lib/api/gen/specgraph/v1/constitution_pb';

export interface ConstitutionData {
  constitution: Constitution | null;
  provenance: ProvenanceEntry[];
  loadError: string | null;
}

async function loadConstitutionData(): Promise<ConstitutionData> {
  try {
    const resp = await constitutionClient.getConstitution({});
    // Empty seam (D-10): a project with no constitution → null + [] empty state,
    // never a throw (Pitfall 4: no stale prior-project badges/sections).
    return {
      constitution: resp.constitution ?? null,
      provenance: resp.provenance ?? [],
      loadError: null,
    };
  } catch (e) {
    return {
      constitution: null,
      provenance: [],
      loadError: e instanceof Error ? e.message : 'Failed to load constitution',
    };
  }
}

export const load: PageLoad = async ({ parent, depends }) => {
  await parent(); // resolve +layout.ts project default before the RPC issues (Pitfall 6)
  depends('app:project'); // the dependency invalidateAll()/invalidate('app:project') re-runs (D-01/D-02)
  // Stream each field off the shared RPC promise (do NOT await) so the $derived
  // Promise.all in +page.svelte re-suspends {#await} to Skeleton on invalidateAll().
  const result = loadConstitutionData();
  return {
    constitution: result.then((r) => r.constitution),
    provenance: result.then((r) => r.provenance),
    loadError: result.then((r) => r.loadError),
  };
};
