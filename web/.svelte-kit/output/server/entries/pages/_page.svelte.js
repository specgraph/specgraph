import "../../chunks/index-server.js";
import "../../chunks/client.js";
import "../../chunks/client2.js";
import "../../chunks/Graph.js";
//#endregion
//#region src/routes/+page.svelte
function _page($$renderer, $$props) {
	$$renderer.component(($$renderer) => {
		$$renderer.push(`<h1 class="svelte-1uha8ag">Dashboard</h1> `);
		$$renderer.push("<!--[0-->");
		$$renderer.push(`<p class="status svelte-1uha8ag">Loading...</p>`);
		$$renderer.push(`<!--]-->`);
	});
}
//#endregion
export { _page as default };
