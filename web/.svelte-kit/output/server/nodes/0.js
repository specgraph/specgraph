

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const imports = ["_app/immutable/nodes/0.5fTaW4Yg.js","_app/immutable/chunks/gxbwe7u6.js","_app/immutable/chunks/B-f1ke-x.js","_app/immutable/chunks/DaB5N9rk.js"];
export const stylesheets = ["_app/immutable/assets/0.BMV4_eWz.css"];
export const fonts = [];
