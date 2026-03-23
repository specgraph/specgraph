

export const index = 1;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/fallbacks/error.svelte.js')).default;
export const imports = ["_app/immutable/nodes/1.B9HTPSNc.js","_app/immutable/chunks/UzYscr8Z.js","_app/immutable/chunks/B4zpJ1zR.js","_app/immutable/chunks/cpNEi2H_.js"];
export const stylesheets = [];
export const fonts = [];
