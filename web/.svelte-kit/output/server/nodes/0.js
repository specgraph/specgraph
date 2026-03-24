

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "ssr": false,
  "prerender": false
};
export const universal_id = "src/routes/+layout.ts";
export const imports = ["_app/immutable/nodes/0.CM9R3J_J.js","_app/immutable/chunks/DiPZ6AcG.js","_app/immutable/chunks/jIRfIB8n.js","_app/immutable/chunks/CNGvqCsZ.js","_app/immutable/chunks/CFKVnMbq.js","_app/immutable/chunks/-R577DTC.js"];
export const stylesheets = ["_app/immutable/assets/0.BwJdOFPD.css"];
export const fonts = [];
