

export const index = 2;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_page.svelte.js')).default;
export const imports = ["_app/immutable/nodes/2.LyyVByK2.js","_app/immutable/chunks/gxbwe7u6.js","_app/immutable/chunks/DaB5N9rk.js","_app/immutable/chunks/B-f1ke-x.js"];
export const stylesheets = ["_app/immutable/assets/2.8UIxJ7Uf.css"];
export const fonts = [];
