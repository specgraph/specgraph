

export const index = 1;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/fallbacks/error.svelte.js')).default;
export const imports = ["_app/immutable/nodes/1.sw17GbU2.js","_app/immutable/chunks/DQwDdmL6.js","_app/immutable/chunks/BCH0nEjg.js","_app/immutable/chunks/DNegTBFU.js","_app/immutable/chunks/D2guxoj4.js"];
export const stylesheets = [];
export const fonts = [];
