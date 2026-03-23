

export const index = 1;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/fallbacks/error.svelte.js')).default;
export const imports = ["_app/immutable/nodes/1.B-F3dL9v.js","_app/immutable/chunks/EpO1WUHy.js","_app/immutable/chunks/DKDOksBq.js","_app/immutable/chunks/B7q04Q7N.js"];
export const stylesheets = [];
export const fonts = [];
