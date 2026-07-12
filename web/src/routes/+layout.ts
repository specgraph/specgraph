// SPA mode: disable SSR and prerendering so all pages render client-side.
// Required for onMount, connect-web RPC calls, and dynamic data loading.
export const ssr = false;
export const prerender = false;

import type { LayoutLoad } from './$types';
import { auth, checkAuth } from '$lib/auth.svelte';
import { loadProjects } from '$lib/project.svelte';

// D-02 / Pitfall 6: resolve auth + project default in load() so universal page
// loads that `await parent()` see the resolved project BEFORE issuing RPCs. No
// +layout.server.ts / server load — this stays a client-only universal load to
// preserve the static SPA (ssr=false) contract. auth_error query handling stays
// in the component (it needs window/history).
export const load: LayoutLoad = async () => {
  await checkAuth();
  if (auth.authenticated) {
    await loadProjects();
  }
  return { authenticated: auth.authenticated };
};
