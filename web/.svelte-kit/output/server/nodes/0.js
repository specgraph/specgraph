

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "ssr": false,
  "prerender": false
};
export const universal_id = "src/routes/+layout.ts";
export const imports = ["_app/immutable/nodes/0.D07AI1M6.js","_app/immutable/chunks/DQwDdmL6.js","_app/immutable/chunks/BCH0nEjg.js","_app/immutable/chunks/IkwjGrrj.js","_app/immutable/chunks/B7UOADyZ.js","_app/immutable/chunks/WkuwMv9z.js","_app/immutable/chunks/C0V0fIAM.js"];
export const stylesheets = ["_app/immutable/assets/0.BMV4_eWz.css"];
export const fonts = [];
