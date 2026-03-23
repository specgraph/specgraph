

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "ssr": false,
  "prerender": false
};
export const universal_id = "src/routes/+layout.ts";
export const imports = ["_app/immutable/nodes/0.B254ftV1.js","_app/immutable/chunks/B48NUe3B.js","_app/immutable/chunks/B2kJjOjn.js","_app/immutable/chunks/D1M1f6y7.js","_app/immutable/chunks/CFKVnMbq.js"];
export const stylesheets = ["_app/immutable/assets/0.B4GZ0ARd.css"];
export const fonts = [];
