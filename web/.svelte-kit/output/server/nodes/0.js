

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "ssr": false,
  "prerender": false
};
export const universal_id = "src/routes/+layout.ts";
export const imports = ["_app/immutable/nodes/0.DJEaLW79.js","_app/immutable/chunks/E6KCdQq1.js","_app/immutable/chunks/BsYrcxWd.js","_app/immutable/chunks/BUdoU3HW.js","_app/immutable/chunks/Cba0ncyT.js","_app/immutable/chunks/DYgEGVNi.js"];
export const stylesheets = ["_app/immutable/assets/0.BMV4_eWz.css"];
export const fonts = [];
