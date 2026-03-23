

export const index = 1;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/fallbacks/error.svelte.js')).default;
export const imports = ["_app/immutable/nodes/1.Cc2pemkN.js","_app/immutable/chunks/E6KCdQq1.js","_app/immutable/chunks/DYgEGVNi.js","_app/immutable/chunks/Cba0ncyT.js"];
export const stylesheets = [];
export const fonts = [];
