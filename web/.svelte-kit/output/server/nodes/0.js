

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "ssr": false,
  "prerender": false
};
export const universal_id = "src/routes/+layout.ts";
export const imports = ["_app/immutable/nodes/0.vvFiN6GC.js","_app/immutable/chunks/BIaOwp2W.js","_app/immutable/chunks/Bg0mstGQ.js","_app/immutable/chunks/LrrZMaOo.js","_app/immutable/chunks/KZkYDI-x.js","_app/immutable/chunks/WfjQCFh9.js"];
export const stylesheets = ["_app/immutable/assets/0.BMV4_eWz.css"];
export const fonts = [];
