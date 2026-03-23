

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "ssr": false,
  "prerender": false
};
export const universal_id = "src/routes/+layout.ts";
export const imports = ["_app/immutable/nodes/0.CFgr-X1m.js","_app/immutable/chunks/EpO1WUHy.js","_app/immutable/chunks/BsYRNPo4.js","_app/immutable/chunks/CZq8_XXc.js","_app/immutable/chunks/B7q04Q7N.js","_app/immutable/chunks/DKDOksBq.js"];
export const stylesheets = ["_app/immutable/assets/0.BMV4_eWz.css"];
export const fonts = [];
