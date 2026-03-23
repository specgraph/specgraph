

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "ssr": false,
  "prerender": false
};
export const universal_id = "src/routes/+layout.ts";
export const imports = ["_app/immutable/nodes/0.CR0W4rck.js","_app/immutable/chunks/UzYscr8Z.js","_app/immutable/chunks/D3ViSdc4.js","_app/immutable/chunks/C664QG7W.js","_app/immutable/chunks/cpNEi2H_.js","_app/immutable/chunks/Zs6rAvSr.js"];
export const stylesheets = ["_app/immutable/assets/0.BMV4_eWz.css"];
export const fonts = [];
