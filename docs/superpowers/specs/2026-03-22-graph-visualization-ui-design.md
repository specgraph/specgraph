# Graph Visualization UI

**Date:** 2026-03-22
**Closes:** spgr-p1l

## Problem

SpecGraph has no visual representation of the spec dependency graph. The CLI
outputs text/markdown tables, which work for AI agents and terminal users but
don't give an at-a-glance picture of project structure. For demos, onboarding,
and non-engineering stakeholders, a visual graph explorer is essential.

## Approach

Embed an interactive SvelteKit web application in the specgraph Go binary.
The UI is read-only — a graph explorer and dashboard served from the same
port as the ConnectRPC API. Write operations stay in the CLI and external
tools (Slack, ADO, GitHub Issues).

Primary consumers: demo audiences, non-engineering stakeholders reviewing
project structure, and developers wanting a visual overview. Deep-linkable
URLs enable future Slack/ADO embedding.

## Architecture

### Embedded SPA

- SvelteKit app lives in `web/` within the repo
- `adapter-static` produces static files at build time
- `web/embed.go` at the repo root declares `//go:embed build/*` (the embed
  directive must be in the same module-relative directory as the files)
- `cmd/specgraph/serve.go` imports the embed FS from `web/` package
- `specgraph serve` serves both ConnectRPC API and UI on the same port
- During development: Vite dev server on `:5173` proxies API calls to Go on `:8080`

### Server Changes

Go server additions:

1. **CORS middleware** — wraps the outermost handler (before `ProjectMiddleware`)
   so preflight `OPTIONS` requests without `X-Specgraph-Project` are handled.
   Only enabled when a `--cors-origin` flag is set (dev mode).
2. **Static file serving** — `http.FileServer` with the embedded FS, registered
   on the mux before the catch-all.
3. **Catch-all routing** — Go's `http.ServeMux` matches longest prefix first.
   ConnectRPC services register exact paths (`/specgraph.v1.GraphService/`).
   Static assets register under a prefix (e.g., `/_app/`). A final `/` handler
   serves `index.html` for all unmatched paths, enabling SvelteKit deep links.

### New RPC: GetFullGraph

```protobuf
message GetFullGraphRequest {}

message GraphNode {
  string slug = 1;
  string label = 2;      // "Spec" or "Decision"
  string stage = 3;       // stage or status
  string intent = 4;      // spec intent or decision title
  string priority = 5;    // p0-p3 (specs only)
}

message GetFullGraphResponse {
  repeated GraphNode nodes = 1;
  repeated Edge edges = 2;  // reuses existing Edge message from graph.proto
}
```

Added to `GraphService`. Returns all specs and decisions as nodes plus all
edges in a single response. `GraphNode` is richer than `NodeRef` — includes
`intent` and `priority` to avoid N+1 `GetSpec` calls for tooltips.

**Join key:** `Edge.from_id` and `Edge.to_id` contain **slugs** (despite the
field name). This matches `GraphNode.slug`. The existing storage layer returns
slugs in these fields (see `graph.go` Cypher: `a.slug AS from_slug`). The
frontend joins edges to nodes via slug matching.

**Excluded edges:** Internal-only edge types (`HAS_CHANGE`, `HAS_FINDING`)
are excluded from the response. Only user-facing edge types in the `EdgeType`
proto enum are returned.

## Frontend Stack

| Technology | Purpose |
|-----------|---------|
| SvelteKit | Framework with file-based routing, `adapter-static` for production |
| TypeScript | Type safety, generated ConnectRPC client types |
| `@connectrpc/connect-web` | Typed RPC calls from browser to Go server |
| `@dagrejs/dagre` (~30KB) | Layered graph layout algorithm |
| SVG | Graph rendering (DOM events for click/hover on nodes) |
| Svelte scoped CSS | Styling (no Tailwind) |

### Proto Codegen

Extend `buf.gen.yaml` to generate `@connectrpc/connect-es` TypeScript stubs
into `web/src/lib/api/gen/`. Same proto sources, additional output target.

## URL Structure

| Route | View | Data Source |
|-------|------|-------------|
| `/` | Dashboard (stats + funnel + mini graph) | `ListSpecs`, `GetReady`, `CheckDrift` (empty slug = all), `GetFullGraph`, `ListDecisions` |
| `/graph` | Full interactive graph | `GetFullGraph` |
| `/spec/:slug` | Spec detail | `GetSpec` |
| `/decision/:slug` | Decision detail | `GetDecision` |

All routes are deep-linkable. The Go catch-all returns `index.html` for any
path not matching a ConnectRPC service or static asset.

## Dashboard (/)

Three sections stacked vertically:

1. **Stats bar** — cards showing total specs, ready count, drift count,
   decision count. Data derived from existing RPCs.

2. **Authoring funnel** — horizontal stacked bar showing spec count per stage
   (spark, shape, specify, decompose, approved, done). Color-coded. Derived
   from `ListSpecs` response.

3. **Mini graph preview** — compact version of the full graph in a smaller
   SVG viewport. Simplified labels, click navigates to `/graph`.

## Graph View (/graph)

### Layout

Dagre layered layout with dependency depth as the layer axis (top to bottom).
Specs and decisions as nodes, edges as directed lines between them.

### Visual Encoding

- **Node color by stage:** spark=purple, shape=blue, specify=green,
  decompose=amber, approved=teal, done=gray
- **Node shape:** rounded rectangles for specs, diamonds or hexagons for decisions
- **Edge style by type:** solid=DEPENDS_ON, dashed=RELATES_TO, dotted=COMPOSES,
  colored=INFORMS/DECIDED_IN
- **Edge direction:** arrows point from dependent to dependency (A→B means A depends on B)

### Interactions (V1)

- **Pan and zoom** — mouse wheel + drag on background
- **Click node** — navigate to `/spec/:slug` or `/decision/:slug`
- **Hover node** — tooltip showing slug, intent/title, stage, priority
- **Client-side search/filter** — text input filters nodes by slug, intent,
  stage, or priority. Matching nodes stay highlighted, non-matching fade out.

## Spec Detail (/spec/:slug)

Renders spec data from `GetSpec` response. Same content as `specgraph show`
CLI output but in a web layout. Includes metadata table, notes, and a link
back to the graph view (centered on this spec).

## Decision Detail (/decision/:slug)

Renders decision data from `GetDecision` response. Same content as
`specgraph decision show`. Metadata, decision text, rationale.

## Directory Structure

```text
web/
  src/
    lib/
      api/                        # ConnectRPC client setup
        gen/                      # Generated TS types from proto
      components/
        Graph.svelte              # Dagre-layouted SVG graph
        GraphMini.svelte          # Compact dashboard version
        StatsBar.svelte           # Stat cards row
        FunnelBar.svelte          # Authoring funnel bar
        SearchFilter.svelte       # Text filter input
    routes/
      +layout.svelte              # Nav bar, shared layout
      +page.svelte                # Dashboard (/)
      graph/+page.svelte          # Full graph (/graph)
      spec/[slug]/+page.svelte    # Spec detail
      decision/[slug]/+page.svelte # Decision detail
  static/                         # Favicon, etc.
  svelte.config.js
  vite.config.ts                  # Dev proxy to Go server
```

## Build & Dev Workflow

### Development

1. `specgraph serve` — Go API on `:8080`
2. `cd web && npm run dev` — Vite on `:5173`, proxying API calls to `:8080`
3. Svelte hot-reload on file changes

### Production

1. `task web:build` — SvelteKit builds to `web/build/`
2. `web/embed.go` provides `//go:embed build/*` FS
3. `task build` compiles Go binary (imports embed FS from `web/` package)
4. `specgraph serve` serves everything on one port

### Taskfile

- `task web:dev` — start Vite dev server
- `task web:build` — production build
- `task build` — runs `web:build` then Go build

## Testing

- **Go:** Unit test for `GetFullGraph` handler and storage method, following
  existing graph test patterns in `internal/server/graph_handler_test.go`
- **Integration:** Extend e2e API tests to cover `GetFullGraph` RPC
- **Frontend:** Manual verification via dev server. No JS test framework in V1.

## Out of Scope

- Write operations from the UI (approvals, edits, comments)
- Server-side search RPC (client-side filtering is sufficient for V1)
- Real-time updates / WebSocket push (refresh to see changes)
- Authentication UI (inherits server auth, no login page)
- Activity feed / changelog view on dashboard
- Mermaid/dot CLI export (separate follow-up task)
- Graph export as image/PDF
- Mobile responsive layout
- Slack unfurl integration (deep links enable this later)

## Affected Files

| File | Change |
|------|--------|
| `proto/specgraph/v1/graph.proto` | Add `GetFullGraph` RPC, `GraphNode` message |
| `gen/specgraph/v1/` | Regenerated Go + new TS output |
| `internal/server/graph_handler.go` | New `GetFullGraph` handler |
| `internal/server/server.go` | CORS middleware, static file serving, catch-all |
| `internal/storage/graph.go` | New `GetFullGraph` method on `GraphBackend` interface |
| `internal/storage/memgraph/graph.go` | Cypher query for all nodes + edges |
| `web/embed.go` | `//go:embed build/*` — embed FS declaration |
| `cmd/specgraph/serve.go` | Import embed FS, serve static assets, catch-all route |
| `buf.gen.yaml` | Add TypeScript/connect-es output |
| `Taskfile.yml` | Add `web:dev`, `web:build` tasks |
| `web/` | Entire SvelteKit application (new) |
| `.gitignore` | Add `web/node_modules/`, `web/build/`, `.superpowers/` |
