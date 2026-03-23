

export const index = 2;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_page.svelte.js')).default;
export const imports = ["_app/immutable/nodes/2.BsN8lbF_.js","_app/immutable/chunks/B0dYN30k.js","_app/immutable/chunks/B59TT3mg.js"];
export const stylesheets = [];
export const fonts = [];
