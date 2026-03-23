

export const index = 2;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_page.svelte.js')).default;
export const imports = ["_app/immutable/nodes/2.tRzyk8Sf.js","_app/immutable/chunks/CHK4LGn_.js","_app/immutable/chunks/DqaXZ1C7.js"];
export const stylesheets = [];
export const fonts = [];
