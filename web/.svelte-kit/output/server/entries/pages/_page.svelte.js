import "../../chunks/client.js";
import "@sveltejs/kit/internal";
import "../../chunks/exports.js";
import "../../chunks/utils2.js";
import "@sveltejs/kit/internal/server";
import "../../chunks/root.js";
import "../../chunks/state.svelte.js";
import "@dagrejs/dagre";
/* empty css                                               */
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    $$renderer2.push(`<h1 class="svelte-1uha8ag">Dashboard</h1> `);
    {
      $$renderer2.push("<!--[0-->");
      $$renderer2.push(`<p class="status svelte-1uha8ag">Loading...</p>`);
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
