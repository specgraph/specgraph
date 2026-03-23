

export const index = 1;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/fallbacks/error.svelte.js')).default;
export const imports = ["_app/immutable/nodes/1.BGObyB1h.js","_app/immutable/chunks/B48NUe3B.js","_app/immutable/chunks/CVZI2aQa.js","_app/immutable/chunks/CFKVnMbq.js"];
export const stylesheets = [];
export const fonts = [];
