

export const index = 1;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/fallbacks/error.svelte.js')).default;
export const imports = ["_app/immutable/nodes/1.DJROjL4b.js","_app/immutable/chunks/gxbwe7u6.js","_app/immutable/chunks/DaB5N9rk.js"];
export const stylesheets = [];
export const fonts = [];
