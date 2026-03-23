

export const index = 1;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/fallbacks/error.svelte.js')).default;
export const imports = ["_app/immutable/nodes/1.DgmkQWtm.js","_app/immutable/chunks/BIaOwp2W.js","_app/immutable/chunks/D3z4wsy3.js","_app/immutable/chunks/KZkYDI-x.js"];
export const stylesheets = [];
export const fonts = [];
