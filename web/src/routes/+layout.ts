// SPA mode: disable SSR and prerendering so all pages render client-side.
// Required for onMount, connect-web RPC calls, and dynamic data loading.
export const ssr = false;
export const prerender = false;
