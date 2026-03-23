# Graph Visualization UI Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an embedded interactive graph visualization UI to the specgraph binary, served alongside the ConnectRPC API on the same port.

**Architecture:** New `GetFullGraph` RPC returns all specs, decisions, and edges in one call. A SvelteKit SPA (built with `adapter-static`) lives in `web/`, embedded via `//go:embed` into the Go binary. The Go server serves static assets and a catch-all for SPA routing alongside existing ConnectRPC handlers. Dagre provides layered graph layout; SVG renders nodes and edges with click/hover interactivity.

**Tech Stack:** Go (ConnectRPC handler, embed, CORS middleware), SvelteKit + TypeScript (SPA), `@connectrpc/connect-web` (typed RPC client), `@dagrejs/dagre` (graph layout), SVG (rendering), buf (proto codegen for TS)

**Design Spec:** `docs/superpowers/specs/2026-03-22-graph-visualization-ui-design.md`

**Beads:** spgr-p1l

---

## File Structure

### Go (Backend)

| File | Responsibility |
|------|---------------|
| `proto/specgraph/v1/graph.proto` | Add `GetFullGraph` RPC, `GraphNode` message, `GetFullGraphRequest`/`GetFullGraphResponse` |
| `gen/specgraph/v1/*.go` | Regenerated proto (via `task proto`) |
| `internal/storage/graph.go` | Add `GraphNode` domain type and `GetFullGraph` method to `GraphBackend` interface |
| `internal/storage/memgraph/graph.go` | Cypher query implementing `GetFullGraph` — returns all Spec+Decision nodes and user-facing edges |
| `internal/server/graph_handler.go` | `GetFullGraph` handler method on `GraphHandler` |
| `internal/server/graph_handler_test.go` | Unit tests for `GetFullGraph` handler |
| `internal/server/test_scoper_test.go` | Add `GetFullGraph` stub to `stubBackend` |
| `internal/server/cors.go` | CORS middleware (dev-mode only, gated on `--cors-origin` flag) |
| `internal/server/static.go` | Static file serving from embedded FS + SPA catch-all handler |
| `cmd/specgraph/serve.go` | Wire CORS middleware, static file serving, `--cors-origin` flag |
| `web/embed.go` | `//go:embed build/*` declaration exposing embedded FS |
| `Taskfile.yml` | Add `web:dev`, `web:build` tasks; make `build` depend on `web:build` |
| `buf.gen.yaml` | Add `@connectrpc/connect-es` TypeScript output plugin |
| `.gitignore` | Add `web/node_modules/`, `web/build/` |

### Frontend (SvelteKit SPA)

| File | Responsibility |
|------|---------------|
| `web/package.json` | Dependencies: SvelteKit, `@dagrejs/dagre`, `@connectrpc/connect-web`, `@connectrpc/protoc-gen-connect-es`, `@bufbuild/protoc-gen-es` |
| `web/svelte.config.js` | SvelteKit config with `adapter-static`, `fallback: 'index.html'` |
| `web/vite.config.ts` | Vite config with proxy to Go server (`:8080`) for dev mode |
| `web/tsconfig.json` | TypeScript config (extends `.svelte-kit/tsconfig.json`) |
| `web/src/app.html` | HTML shell |
| `web/src/app.d.ts` | SvelteKit ambient type declarations |
| `web/src/lib/api/client.ts` | ConnectRPC transport with project header interceptor + all service clients (Graph, Spec, Decision, Lifecycle) |
| `web/src/lib/api/gen/` | Generated TypeScript types from proto (via buf) |
| `web/src/lib/components/Graph.svelte` | Full interactive SVG graph with Dagre layout, pan/zoom, click/hover |
| `web/src/lib/components/GraphMini.svelte` | Compact dashboard graph preview |
| `web/src/lib/components/StatsBar.svelte` | Stats cards row (spec count, ready, drift, decisions) |
| `web/src/lib/components/FunnelBar.svelte` | Authoring funnel horizontal stacked bar |
| `web/src/lib/components/SearchFilter.svelte` | Client-side text filter input |
| `web/src/routes/+layout.svelte` | Nav bar, shared layout |
| `web/src/routes/+page.svelte` | Dashboard (`/`) |
| `web/src/routes/graph/+page.svelte` | Full graph view (`/graph`) |
| `web/src/routes/spec/[slug]/+page.svelte` | Spec detail page |
| `web/src/routes/decision/[slug]/+page.svelte` | Decision detail page |

### E2E Tests (Playwright in Docker)

| File | Responsibility |
|------|---------------|
| `Dockerfile.e2e` | Multi-stage build: SvelteKit → Go binary with embedded UI → Alpine runtime |
| `e2e/ui/package.json` | Playwright test dependencies |
| `e2e/ui/playwright.config.ts` | Playwright config (baseURL from env, Docker-native) |
| `e2e/ui/docker-compose.e2e.yaml` | Three-container stack: Memgraph + SpecGraph + Playwright |
| `e2e/ui/Dockerfile.playwright` | Playwright container with test files |
| `e2e/ui/config.e2e.yaml` | SpecGraph server config for Docker network |
| `e2e/ui/tests/helpers.ts` | Test data seeding via ConnectRPC |
| `e2e/ui/tests/navigation.spec.ts` | Nav bar, routing, deep links |
| `e2e/ui/tests/graph.spec.ts` | SVG graph rendering, search filter, node click |
| `e2e/ui/tests/detail.spec.ts` | Spec/decision detail pages, error states |

---

## Chunk 1: Backend — Proto, Storage, Handler, Tests

### Task 1: Add `GraphNode` domain type and `GetFullGraph` to storage interface

**Files:**

- Modify: `internal/storage/graph.go`

- [ ] **Step 1: Add `GraphNode` domain type to `internal/storage/graph.go`**

Add after the `DependencyRef` struct (line 73):

```go
// GraphNode is a rich node reference for graph visualization.
// It includes intent/priority to avoid N+1 lookups for tooltips.
type GraphNode struct {
	Slug     string
	Label    NodeLabel // "Spec" or "Decision"
	Stage    string    // authoring stage (specs) or status (decisions)
	Intent   string    // spec intent or decision title
	Priority string    // p0-p3 for specs, empty for decisions
}

// FullGraph contains all nodes and edges for graph visualization.
type FullGraph struct {
	Nodes []GraphNode
	Edges []*Edge
}
```

- [ ] **Step 2: Add `GetFullGraph` method to `GraphBackend` interface**

Add to the `GraphBackend` interface in `internal/storage/graph.go`, after `RefreshDependencyHashes`:

```go
	// GetFullGraph returns all spec and decision nodes with all user-facing edges.
	// Internal edge types (HAS_CHANGE, HAS_FINDING) are excluded.
	GetFullGraph(ctx context.Context) (*FullGraph, error)
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/storage/...`
Expected: Success (this only builds the interface file, not implementors like memgraph)

- [ ] **Step 4: Commit**

```
jj --no-pager commit -m "feat(storage): add GraphNode type and GetFullGraph interface method"
```

### Task 2: Implement `GetFullGraph` in Memgraph storage

**Files:**

- Modify: `internal/storage/memgraph/graph.go`

- [ ] **Step 1: Write the `GetFullGraph` implementation**

Add to `internal/storage/memgraph/graph.go` after the `GetCriticalPath` method (before `queryNodeRefs`):

```go
// GetFullGraph returns all spec and decision nodes with all user-facing edges.
func (s *Store) GetFullGraph(ctx context.Context) (*storage.FullGraph, error) {
	// Query 1: All nodes (Spec + Decision)
	nodeQuery := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(n)
		WHERE n:Spec OR n:Decision
		RETURN n.slug AS slug, labels(n)[0] AS label,
		       COALESCE(n.stage, n.status, "") AS stage,
		       COALESCE(n.intent, n.title, "") AS intent,
		       COALESCE(n.priority, "") AS priority
	`
	nodeRecords, err := s.executeQuery(ctx, nodeQuery, s.projectParam())
	if err != nil {
		return nil, fmt.Errorf("memgraph: get full graph nodes: %w", err)
	}

	nodes := make([]storage.GraphNode, 0, len(nodeRecords))
	for _, rec := range nodeRecords {
		slug, _ := rec.Get("slug")
		label, _ := rec.Get("label")
		stage, _ := rec.Get("stage")
		intent, _ := rec.Get("intent")
		priority, _ := rec.Get("priority")
		nodes = append(nodes, storage.GraphNode{
			Slug:     stringVal(slug),
			Label:    storage.NodeLabel(stringVal(label)),
			Stage:    stringVal(stage),
			Intent:   stringVal(intent),
			Priority: stringVal(priority),
		})
	}

	// Query 2: All user-facing edges (exclude BELONGS_TO, HAS_CHANGE, HAS_FINDING)
	edgeQuery := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a)-[r]->(b)
		WHERE type(r) <> "BELONGS_TO" AND type(r) <> "HAS_CHANGE" AND type(r) <> "HAS_FINDING"
		  AND (b:Spec OR b:Decision)
		RETURN a.slug AS from_slug, b.slug AS to_slug, type(r) AS rel_type
	`
	edgeRecords, err := s.executeQuery(ctx, edgeQuery, s.projectParam())
	if err != nil {
		return nil, fmt.Errorf("memgraph: get full graph edges: %w", err)
	}

	edges := make([]*storage.Edge, 0, len(edgeRecords))
	for _, rec := range edgeRecords {
		from, _ := rec.Get("from_slug")
		to, _ := rec.Get("to_slug")
		rt, _ := rec.Get("rel_type")
		edgeType, err := relNameToEdgeType(stringVal(rt))
		if err != nil {
			return nil, fmt.Errorf("GetFullGraph: %w", err)
		}
		edges = append(edges, &storage.Edge{
			FromID:   stringVal(from),
			ToID:     stringVal(to),
			EdgeType: edgeType,
		})
	}

	return &storage.FullGraph{Nodes: nodes, Edges: edges}, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/storage/memgraph/...`
Expected: Success

- [ ] **Step 3: Commit**

```
jj --no-pager commit -m "feat(memgraph): implement GetFullGraph Cypher queries"
```

### Task 3: Add `GetFullGraph` stub to test scoper

**Files:**

- Modify: `internal/server/test_scoper_test.go`

- [ ] **Step 1: Add `GetFullGraph` stub to `stubBackend`**

Add after the `GetCriticalPath` stub (around line 99):

```go
func (stubBackend) GetFullGraph(context.Context) (*storage.FullGraph, error) {
	return nil, errNotImplemented
}
```

- [ ] **Step 2: Verify compilation of test package**

Run: `go build ./internal/server/...`
Expected: Success (stubBackend now satisfies updated ScopedBackend)

- [ ] **Step 3: Commit**

```
jj --no-pager commit -m "test(server): add GetFullGraph stub to stubBackend"
```

### Task 4: Add `GetFullGraph` proto messages and RPC

**Files:**

- Modify: `proto/specgraph/v1/graph.proto`

- [ ] **Step 1: Add proto messages and RPC**

Add to `proto/specgraph/v1/graph.proto` — messages before the `service` block, RPC inside it.

After `GetCriticalPathResponse` (line 99), add:

```protobuf
message GetFullGraphRequest {}

message GraphNode {
  string slug = 1;
  string label = 2;      // "Spec" or "Decision"
  string stage = 3;       // authoring stage or decision status
  string intent = 4;      // spec intent or decision title
  string priority = 5;    // p0-p3 (specs only, empty for decisions)
}

message GetFullGraphResponse {
  repeated GraphNode nodes = 1;
  repeated Edge edges = 2;
}
```

Add to the `service GraphService` block, after `GetCriticalPath`:

```protobuf
  rpc GetFullGraph(GetFullGraphRequest) returns (GetFullGraphResponse);
```

- [ ] **Step 2: Regenerate Go proto code**

Run: `task proto`
Expected: Generates updated files in `gen/specgraph/v1/`

- [ ] **Step 3: Verify generated code compiles**

Run: `go build ./gen/...`
Expected: Success

- [ ] **Step 4: Commit**

```
jj --no-pager commit -m "feat(proto): add GetFullGraph RPC with GraphNode message"
```

### Task 5: Write failing test for `GetFullGraph` handler

**Files:**

- Modify: `internal/server/graph_handler_test.go`

- [ ] **Step 1: Add `GetFullGraph` mock method to `mockGraphBackend`**

Add after the `GetCriticalPath` mock method:

```go
func (m *mockGraphBackend) GetFullGraph(_ context.Context) (*storage.FullGraph, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	nodes := make([]storage.GraphNode, 0, len(m.nodes))
	for _, n := range m.nodes {
		nodes = append(nodes, storage.GraphNode{
			Slug:     n.slug,
			Label:    n.label,
			Stage:    n.stage,
			Intent:   "test intent",
			Priority: "p2",
		})
	}
	edges := make([]*storage.Edge, 0, len(m.edges))
	for _, e := range m.edges {
		edges = append(edges, &storage.Edge{
			FromID:   e.from,
			ToID:     e.to,
			EdgeType: e.edgeType,
		})
	}
	return &storage.FullGraph{Nodes: nodes, Edges: edges}, nil
}
```

- [ ] **Step 2: Write the test**

Add after the existing graph tests:

```go
func TestGraphHandler_GetFullGraph(t *testing.T) {
	client := setupGraphServer(t)
	ctx := context.Background()

	// Add an edge so we have edges in the response
	_, err := client.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
		FromSlug: "spec-a",
		ToSlug:   "spec-b",
		EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	}))
	require.NoError(t, err)

	resp, err := client.GetFullGraph(ctx, connect.NewRequest(&specv1.GetFullGraphRequest{}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Msg.Nodes), 3, "should return all mock nodes")
	require.Len(t, resp.Msg.Edges, 1, "should return the added edge")

	// Verify node fields are populated
	var found bool
	for _, n := range resp.Msg.Nodes {
		if n.Slug == "spec-a" {
			found = true
			require.Equal(t, "Spec", n.Label)
			require.Equal(t, "spark", n.Stage)
			require.Equal(t, "test intent", n.Intent)
			require.Equal(t, "p2", n.Priority)
		}
	}
	require.True(t, found, "spec-a should be in the response")

	// Verify edge
	require.Equal(t, "spec-a", resp.Msg.Edges[0].FromId)
	require.Equal(t, "spec-b", resp.Msg.Edges[0].ToId)
	require.Equal(t, specv1.EdgeType_EDGE_TYPE_DEPENDS_ON, resp.Msg.Edges[0].EdgeType)
}
```

- [ ] **Step 3: Run test — expect failure (handler not implemented yet)**

Run: `go test -run TestGraphHandler_GetFullGraph -v ./internal/server/`
Expected: Compilation error — `GetFullGraph` method not on `GraphHandler`

- [ ] **Step 4: Commit**

```
jj --no-pager commit -m "test(server): add failing test for GetFullGraph handler"
```

### Task 6: Implement `GetFullGraph` handler

**Files:**

- Modify: `internal/server/graph_handler.go`

- [ ] **Step 1: Add `graphNodeToProto` converter**

Add a helper function for converting domain `GraphNode` to proto:

```go
func graphNodesToProto(nodes []storage.GraphNode) []*specv1.GraphNode {
	result := make([]*specv1.GraphNode, len(nodes))
	for i, n := range nodes {
		result[i] = &specv1.GraphNode{
			Slug:     n.Slug,
			Label:    string(n.Label),
			Stage:    n.Stage,
			Intent:   n.Intent,
			Priority: n.Priority,
		}
	}
	return result
}
```

- [ ] **Step 2: Implement the handler method**

Add to `GraphHandler`:

```go
// GetFullGraph handles the GetFullGraph RPC.
func (h *GraphHandler) GetFullGraph(ctx context.Context, _ *connect.Request[specv1.GetFullGraphRequest]) (*connect.Response[specv1.GetFullGraphResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	graph, err := store.GetFullGraph(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pbs, err := edgesToProto(graph.Edges)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetFullGraphResponse{
		Nodes: graphNodesToProto(graph.Nodes),
		Edges: pbs,
	}), nil
}
```

- [ ] **Step 3: Run test — expect pass**

Run: `go test -run TestGraphHandler_GetFullGraph -v ./internal/server/`
Expected: PASS

- [ ] **Step 4: Run full server test suite**

Run: `go test -v ./internal/server/...`
Expected: All tests pass

- [ ] **Step 5: Commit**

```
jj --no-pager commit -m "feat(server): implement GetFullGraph handler"
```

### Task 7: CORS middleware

**Files:**

- Create: `internal/server/cors.go`

- [ ] **Step 1: Create CORS middleware**

Create `internal/server/cors.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import "net/http"

// CORSMiddleware adds CORS headers for development mode.
// Only enabled when origin is non-empty (set via --cors-origin flag).
func CORSMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Connect-Protocol-Version, X-Specgraph-Project")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/server/...`
Expected: Success

- [ ] **Step 3: Commit**

```
jj --no-pager commit -m "feat(server): add CORS middleware for dev mode"
```

### Task 8: Static file serving and SPA catch-all

**Files:**

- Create: `internal/server/static.go`

- [ ] **Step 1: Create static file handler**

Create `internal/server/static.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"io/fs"
	"net/http"
)

// StaticHandler serves embedded static files with an SPA catch-all.
// Files matching actual paths are served directly. All other paths
// return index.html so SvelteKit client-side routing works.
func StaticHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try serving the file directly
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		}
		// Check if the file exists in the embedded FS
		f, err := fsys.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA catch-all: serve index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/server/...`
Expected: Success

- [ ] **Step 3: Commit**

```
jj --no-pager commit -m "feat(server): add static file handler with SPA catch-all"
```

### Task 9: Wire serve command — embed FS, CORS flag, static serving

**Files:**

- Create: `web/embed.go`
- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Create the embed package**

Create `web/embed.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package web provides the embedded static files for the SpecGraph UI.
package web

import "embed"

// Build contains the SvelteKit production build output.
// When the web UI hasn't been built yet, this will be empty.
//
//go:embed all:build
var Build embed.FS
```

> **Note:** The `all:build` pattern includes dotfiles. The `web/build/` directory must exist for compilation. Task 14 (Taskfile) ensures `web:build` runs before `go build`. For initial development before the SvelteKit app exists, create a placeholder in Step 2. The placeholder file is intentionally committed (jj auto-tracks it) so that `go build` works after Chunk 1 merges, before Chunk 2 is complete. It will be gitignored in Chunk 2, Task 15.

- [ ] **Step 2: Create placeholder build directory**

```bash
mkdir -p web/build
echo '<!DOCTYPE html><html><head><title>SpecGraph</title></head><body><p>Run task web:build first</p></body></html>' > web/build/index.html
```

- [ ] **Step 3: Modify `cmd/specgraph/serve.go`**

Add the `--cors-origin` flag in `init()`:

```go
func init() {
	serveCmd.Flags().String("cors-origin", "", "Enable CORS for this origin (dev mode only)")
	rootCmd.AddCommand(serveCmd)
}
```

**Replace** the existing `handler := server.ProjectMiddleware(mux)` line (line 108 in `serve.go`) and add the static file serving + CORS wiring immediately before it. The result replaces one line with this block:

```go
		// Serve embedded UI static files
		webFS, err := fs.Sub(web.Build, "build")
		if err != nil {
			return fmt.Errorf("embedded web FS: %w", err)
		}
		// Register static handler as catch-all (after ConnectRPC paths)
		mux.Handle("/", server.StaticHandler(webFS))

		handler := server.ProjectMiddleware(mux)

		// Optional CORS for dev mode (Vite on :5173 → Go on :8080)
		corsOrigin, _ := cmd.Flags().GetString("cors-origin")
		if corsOrigin != "" {
			handler = server.CORSMiddleware(corsOrigin, handler)
		}
```

**Update the `runServe` function signature** (line 39) — change `_ *cobra.Command` to `cmd *cobra.Command`:

```go
// Before:
func runServe(_ *cobra.Command, _ []string) error {
// After:
func runServe(cmd *cobra.Command, _ []string) error {
```

Add these imports to `serve.go`:

```go
	"io/fs"
	"github.com/specgraph/specgraph/web"
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./cmd/specgraph/...`
Expected: Success

- [ ] **Step 5: Commit**

```
jj --no-pager commit -m "feat(serve): wire embedded UI, CORS flag, and static file serving"
```

### Task 10: Run `task check` quality gate

- [ ] **Step 1: Add license headers**

Run: `task license:add`

- [ ] **Step 2: Format code**

Run: `task fmt`

- [ ] **Step 3: Run quality gate**

Run: `task check`
Expected: All checks pass (lint, build, tests)

- [ ] **Step 4: Fix any issues and commit**

```
jj --no-pager commit -m "chore: fix lint and formatting for graph viz backend"
```

---

## Chunk 2: Proto Codegen for TypeScript + SvelteKit Scaffold

### Task 11: Add TypeScript proto codegen to `buf.gen.yaml`

**Files:**

- Modify: `buf.gen.yaml`

- [ ] **Step 1: Add connect-es plugin to `buf.gen.yaml`**

Add two new plugins for TypeScript generation:

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: paths=source_relative
  - remote: buf.build/connectrpc/go
    out: gen
    opt: paths=source_relative
  - remote: buf.build/bufbuild/es
    out: web/src/lib/api/gen
    opt: target=ts
  - remote: buf.build/connectrpc/es
    out: web/src/lib/api/gen
    opt: target=ts
```

- [ ] **Step 2: Regenerate proto (Go + TS)**

Run: `task proto`
Expected: Go files regenerated in `gen/`, TypeScript files generated in `web/src/lib/api/gen/`

- [ ] **Step 3: Verify Go still compiles**

Run: `go build ./...`
Expected: Success

- [ ] **Step 4: Commit**

```
jj --no-pager commit -m "feat(proto): add TypeScript connect-es codegen to buf.gen.yaml"
```

### Task 12: Scaffold SvelteKit project

**Files:**

- Create: `web/package.json`, `web/svelte.config.js`, `web/vite.config.ts`, `web/tsconfig.json`, `web/src/app.html`

- [ ] **Step 1: Create SvelteKit project files manually**

> **Note:** `pnpm create svelte@latest` is fully interactive and cannot be scripted. Create the files directly.

Create `web/package.json`:

```json
{
  "name": "specgraph-ui",
  "version": "0.0.1",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "preview": "vite preview"
  },
  "devDependencies": {
    "@sveltejs/adapter-static": "^3.0.0",
    "@sveltejs/kit": "^2.0.0",
    "@sveltejs/vite-plugin-svelte": "^4.0.0",
    "svelte": "^5.0.0",
    "typescript": "^5.0.0",
    "vite": "^6.0.0"
  },
  "dependencies": {
    "@bufbuild/protobuf": "^2.0.0",
    "@connectrpc/connect": "^2.0.0",
    "@connectrpc/connect-web": "^2.0.0",
    "@dagrejs/dagre": "^1.1.0"
  }
}
```

- [ ] **Step 2: Create SvelteKit config with `adapter-static`**

Create `web/svelte.config.js`:

```js
import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  kit: {
    adapter: adapter({
      fallback: 'index.html'
    })
  }
};

export default config;
```

- [ ] **Step 3: Create Vite config with dev proxy**

Create `web/vite.config.ts`:

```ts
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    proxy: {
      // Proxy ConnectRPC calls to Go server in dev mode
      '/specgraph.v1': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
});
```

- [ ] **Step 4: Create app HTML shell**

Create `web/src/app.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>SpecGraph</title>
  %sveltekit.head%
</head>
<body data-sveltekit-prerender="false">
  <div style="display: contents">%sveltekit.body%</div>
</body>
</html>
```

- [ ] **Step 5: Create TypeScript config**

Create `web/tsconfig.json`:

```json
{
  "extends": "./.svelte-kit/tsconfig.json",
  "compilerOptions": {
    "allowJs": true,
    "checkJs": true,
    "esModuleInterop": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "skipLibCheck": true,
    "sourceMap": true,
    "strict": true,
    "moduleResolution": "bundler"
  }
}
```

- [ ] **Step 6: Create SvelteKit ambient type declarations**

Create `web/src/app.d.ts`:

```ts
/// <reference types="@sveltejs/kit" />

declare namespace App {}
```

- [ ] **Step 7: Create placeholder route so the project builds**

Create `web/src/routes/+page.svelte`:

```svelte
<h1>SpecGraph</h1>
<p>Graph visualization loading...</p>
```

> This placeholder will be replaced by the real dashboard in Chunk 3, Task 19.

- [ ] **Step 8: Install dependencies**

```bash
cd web && pnpm install
```

- [ ] **Step 9: Verify the scaffold builds**

```bash
cd web && pnpm build
```

Expected: Build succeeds, `web/build/` contains `index.html`

- [ ] **Step 10: Commit**

```
jj --no-pager commit -m "feat(web): scaffold SvelteKit project with adapter-static"
```

### Task 13: Create ConnectRPC client setup

**Files:**

- Create: `web/src/lib/api/client.ts`

- [ ] **Step 1: Create the RPC client module**

Create `web/src/lib/api/client.ts`:

```ts
import { createConnectTransport } from '@connectrpc/connect-web';
import { createClient, type Interceptor } from '@connectrpc/connect';
import { GraphService } from './gen/specgraph/v1/graph_pb';
import { SpecService } from './gen/specgraph/v1/spec_pb';
import { DecisionService } from './gen/specgraph/v1/decision_pb';
import { LifecycleService } from './gen/specgraph/v1/lifecycle_pb';

// Interceptor that injects X-Specgraph-Project header on every request.
// In V1 we use a hardcoded project; future versions will have project selection.
const projectInterceptor: Interceptor = (next) => async (req) => {
  req.header.set('X-Specgraph-Project', 'default');
  return next(req);
};

// In dev mode, Vite proxies /specgraph.v1.* to the Go server.
// In production, the Go server serves both the SPA and the API.
const transport = createConnectTransport({
  baseUrl: '/',
  interceptors: [projectInterceptor],
});

export const graphClient = createClient(GraphService, transport);
export const specClient = createClient(SpecService, transport);
export const decisionClient = createClient(DecisionService, transport);
export const lifecycleClient = createClient(LifecycleService, transport);
```

> **Note:** The exact import paths for generated types depend on what `buf generate` produces. The implementer should verify the generated file structure in `web/src/lib/api/gen/` and adjust imports accordingly. Service descriptors are typically exported from `*_pb.ts` or `*_connect.ts` files.

- [ ] **Step 2: Commit**

```
jj --no-pager commit -m "feat(web): add ConnectRPC client setup"
```

### Task 14: Add Taskfile web tasks

**Files:**

- Modify: `Taskfile.yml`

- [ ] **Step 1: Add web tasks to Taskfile**

Add these tasks to `Taskfile.yml`:

```yaml
  web:dev:
    desc: Start SvelteKit dev server (proxies API to :8080)
    dir: web
    cmds:
      - pnpm dev
  web:build:
    desc: Build SvelteKit for production (output to web/build/)
    dir: web
    sources:
      - web/src/**/*
      - web/svelte.config.js
      - web/vite.config.ts
      - web/package.json
    generates:
      - web/build/**/*
    cmds:
      - pnpm build
```

Update the existing `build` task to depend on `web:build`:

```yaml
  build:
    desc: Build the specgraph binary
    deps: [generate, web:build]
    cmds:
      - go build -o {{.BINARY_NAME}} {{.MAIN_PKG}}
```

- [ ] **Step 2: Commit**

```
jj --no-pager commit -m "chore(taskfile): add web:dev, web:build tasks"
```

### Task 15: Update `.gitignore`

**Files:**

- Modify: `.gitignore`

- [ ] **Step 1: Add web entries to `.gitignore`**

Add to `.gitignore`:

```
# Web UI
web/node_modules/
web/build/
web/.svelte-kit/
```

- [ ] **Step 2: Commit**

```
jj --no-pager commit -m "chore: add web UI entries to .gitignore"
```

---

## Chunk 3: Frontend — Layout, Graph Component, Dashboard

### Task 16: Shared layout with navigation

**Files:**

- Create: `web/src/routes/+layout.svelte`

- [ ] **Step 1: Create layout component**

Create `web/src/routes/+layout.svelte`:

```svelte
<script>
  import { page } from '$app/stores';

  let { children } = $props();
</script>

<nav>
  <a href="/" class:active={$page.url.pathname === '/'}>Dashboard</a>
  <a href="/graph" class:active={$page.url.pathname === '/graph'}>Graph</a>
  <span class="spacer"></span>
  <span class="brand">SpecGraph</span>
</nav>

<main>
  {@render children()}
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    color: #1a1a2e;
    background: #f8f9fa;
  }

  nav {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.75rem 1.5rem;
    background: #1a1a2e;
    color: white;
  }

  nav a {
    color: rgba(255, 255, 255, 0.7);
    text-decoration: none;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.9rem;
  }

  nav a:hover, nav a.active {
    color: white;
    background: rgba(255, 255, 255, 0.1);
  }

  .spacer {
    flex: 1;
  }

  .brand {
    font-weight: 600;
    font-size: 0.9rem;
    opacity: 0.8;
  }

  main {
    padding: 1.5rem;
    max-width: 1400px;
    margin: 0 auto;
  }
</style>
```

- [ ] **Step 2: Verify dev server starts**

```bash
cd web && pnpm dev
```

Expected: Vite starts on `:5173`, page loads with nav bar

- [ ] **Step 3: Commit**

```
jj --no-pager commit -m "feat(web): add shared layout with navigation"
```

### Task 17: Graph component with Dagre layout

**Files:**

- Create: `web/src/lib/components/Graph.svelte`

This is the core interactive graph visualization. The implementer should build this component with:

- [ ] **Step 1: Create the Graph component**

Create `web/src/lib/components/Graph.svelte`:

```svelte
<script lang="ts">
  import dagre from '@dagrejs/dagre';
  import type { GraphNode, Edge } from '../api/gen/specgraph/v1/graph_pb';

  interface Props {
    nodes: GraphNode[];
    edges: Edge[];
    compact?: boolean;
    filterText?: string;
  }

  let { nodes, edges, compact = false, filterText = '' }: Props = $props();

  // Stage → color mapping
  const stageColors: Record<string, string> = {
    spark: '#7c3aed',
    shape: '#2563eb',
    specify: '#16a34a',
    decompose: '#d97706',
    approved: '#0d9488',
    done: '#6b7280',
    // Decision statuses
    DECISION_STATUS_PROPOSED: '#7c3aed',
    DECISION_STATUS_ACCEPTED: '#16a34a',
    DECISION_STATUS_SUPERSEDED: '#6b7280',
    DECISION_STATUS_DEPRECATED: '#9ca3af',
  };

  // Edge type → style mapping
  const edgeStyles: Record<string, { dash: string; color: string }> = {
    EDGE_TYPE_DEPENDS_ON: { dash: '', color: '#64748b' },
    EDGE_TYPE_RELATES_TO: { dash: '6,3', color: '#94a3b8' },
    EDGE_TYPE_COMPOSES: { dash: '2,2', color: '#64748b' },
    EDGE_TYPE_INFORMS: { dash: '', color: '#2563eb' },
    EDGE_TYPE_DECIDED_IN: { dash: '', color: '#7c3aed' },
    EDGE_TYPE_BLOCKS: { dash: '', color: '#dc2626' },
    EDGE_TYPE_SUPERSEDES: { dash: '6,3', color: '#9ca3af' },
  };

  const NODE_WIDTH = compact ? 100 : 160;
  const NODE_HEIGHT = compact ? 30 : 40;

  // Layout computation
  let layout = $derived.by(() => {
    const g = new dagre.graphlib.Graph();
    g.setGraph({ rankdir: 'TB', nodesep: 30, ranksep: 50 });
    g.setDefaultEdgeLabel(() => ({}));

    const lowerFilter = filterText.toLowerCase();
    const matchesFilter = (n: GraphNode) => {
      if (!lowerFilter) return true;
      return (
        n.slug.toLowerCase().includes(lowerFilter) ||
        n.intent.toLowerCase().includes(lowerFilter) ||
        n.stage.toLowerCase().includes(lowerFilter) ||
        n.priority.toLowerCase().includes(lowerFilter)
      );
    };

    for (const node of nodes) {
      g.setNode(node.slug, { width: NODE_WIDTH, height: NODE_HEIGHT });
    }
    for (const edge of edges) {
      // EdgeType enum values are numeric in generated TS
      g.setEdge(edge.fromId, edge.toId);
    }

    dagre.layout(g);

    const laidOutNodes = nodes.map((n) => {
      const pos = g.node(n.slug);
      return {
        ...n,
        x: pos?.x ?? 0,
        y: pos?.y ?? 0,
        matches: matchesFilter(n),
      };
    });

    const laidOutEdges = edges.map((e) => {
      const edgeData = g.edge(e.fromId, e.toId);
      return {
        ...e,
        points: edgeData?.points ?? [],
      };
    });

    // Compute viewBox from laid out positions
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const n of laidOutNodes) {
      minX = Math.min(minX, n.x - NODE_WIDTH / 2);
      minY = Math.min(minY, n.y - NODE_HEIGHT / 2);
      maxX = Math.max(maxX, n.x + NODE_WIDTH / 2);
      maxY = Math.max(maxY, n.y + NODE_HEIGHT / 2);
    }
    const pad = 40;

    return {
      nodes: laidOutNodes,
      edges: laidOutEdges,
      viewBox: `${minX - pad} ${minY - pad} ${maxX - minX + 2 * pad} ${maxY - minY + 2 * pad}`,
    };
  });

  let hoveredSlug = $state<string | null>(null);

  // Pan/zoom state
  let zoom = $state(1);
  let panX = $state(0);
  let panY = $state(0);
  let isPanning = $state(false);
  let panStart = $state({ x: 0, y: 0 });

  function handleWheel(e: WheelEvent) {
    if (compact) return;
    e.preventDefault();
    const factor = e.deltaY > 0 ? 0.9 : 1.1;
    zoom = Math.max(0.1, Math.min(5, zoom * factor));
  }

  function handleMouseDown(e: MouseEvent) {
    if (compact || e.button !== 0) return;
    // Only pan when clicking on background (not on nodes)
    if ((e.target as Element).closest('.node')) return;
    isPanning = true;
    panStart = { x: e.clientX - panX, y: e.clientY - panY };
  }

  function handleMouseMove(e: MouseEvent) {
    if (!isPanning) return;
    panX = e.clientX - panStart.x;
    panY = e.clientY - panStart.y;
  }

  function handleMouseUp() {
    isPanning = false;
  }

  function edgeTypeName(et: number): string {
    // Map numeric enum to string key for style lookup.
    // These values match the EdgeType proto enum in graph.proto.
    // Verify against generated TS enum if proto enum order changes.
    const names: Record<number, string> = {
      1: 'EDGE_TYPE_DEPENDS_ON',
      2: 'EDGE_TYPE_BLOCKS',
      3: 'EDGE_TYPE_COMPOSES',
      4: 'EDGE_TYPE_RELATES_TO',
      5: 'EDGE_TYPE_INFORMS',
      6: 'EDGE_TYPE_DECIDED_IN',
      7: 'EDGE_TYPE_SUPERSEDES',
    };
    return names[et] ?? 'EDGE_TYPE_DEPENDS_ON';
  }

  function pointsToPath(points: Array<{ x: number; y: number }>): string {
    if (points.length === 0) return '';
    return points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x},${p.y}`).join(' ');
  }

  function nodeHref(node: { label: string; slug: string }): string {
    return node.label === 'Decision' ? `/decision/${node.slug}` : `/spec/${node.slug}`;
  }

  function nodeColor(stage: string): string {
    return stageColors[stage] ?? '#6b7280';
  }

  function isDecision(label: string): boolean {
    return label === 'Decision';
  }
</script>

<svg
  viewBox={layout.viewBox}
  class="graph"
  class:compact
  onwheel={handleWheel}
  onmousedown={handleMouseDown}
  onmousemove={handleMouseMove}
  onmouseup={handleMouseUp}
  onmouseleave={handleMouseUp}
>
  <defs>
    <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="10" refY="3.5" orient="auto">
      <polygon points="0 0, 10 3.5, 0 7" fill="#64748b" />
    </marker>
  </defs>

  <g transform="translate({panX},{panY}) scale({zoom})">
  <!-- Edges -->
  {#each layout.edges as edge}
    {@const style = edgeStyles[edgeTypeName(edge.edgeType)] ?? edgeStyles.EDGE_TYPE_DEPENDS_ON}
    <path
      d={pointsToPath(edge.points)}
      fill="none"
      stroke={style.color}
      stroke-width="1.5"
      stroke-dasharray={style.dash}
      marker-end="url(#arrowhead)"
    />
  {/each}

  <!-- Nodes -->
  {#each layout.nodes as node}
    <a href={nodeHref(node)}>
      <g
        transform="translate({node.x},{node.y})"
        class="node"
        class:faded={filterText && !node.matches}
        onmouseenter={() => (hoveredSlug = node.slug)}
        onmouseleave={() => (hoveredSlug = null)}
      >
        {#if isDecision(node.label)}
          <!-- Diamond shape for decisions -->
          <polygon
            points="0,{-NODE_HEIGHT / 2} {NODE_WIDTH / 2},0 0,{NODE_HEIGHT / 2} {-NODE_WIDTH / 2},0"
            fill={nodeColor(node.stage)}
            opacity="0.15"
            stroke={nodeColor(node.stage)}
            stroke-width="2"
          />
        {:else}
          <!-- Rounded rectangle for specs -->
          <rect
            x={-NODE_WIDTH / 2}
            y={-NODE_HEIGHT / 2}
            width={NODE_WIDTH}
            height={NODE_HEIGHT}
            rx="6"
            fill={nodeColor(node.stage)}
            opacity="0.15"
            stroke={nodeColor(node.stage)}
            stroke-width="2"
          />
        {/if}
        <text
          text-anchor="middle"
          dominant-baseline="central"
          font-size={compact ? '10' : '12'}
          fill={nodeColor(node.stage)}
        >
          {node.slug}
        </text>

        <!-- Tooltip on hover -->
        {#if hoveredSlug === node.slug && !compact}
          <g transform="translate(0, {NODE_HEIGHT / 2 + 8})">
            <rect
              x="-100" y="0" width="200" height="60"
              rx="4" fill="white" stroke="#e2e8f0" stroke-width="1"
              filter="drop-shadow(0 2px 4px rgba(0,0,0,0.1))"
            />
            <text x="0" y="16" text-anchor="middle" font-size="11" fill="#334155">
              {node.intent || node.slug}
            </text>
            <text x="0" y="32" text-anchor="middle" font-size="10" fill="#64748b">
              {node.stage}{node.priority ? ` · ${node.priority}` : ''}
            </text>
            <text x="0" y="48" text-anchor="middle" font-size="10" fill="#94a3b8">
              {node.label}
            </text>
          </g>
        {/if}
      </g>
    </a>
  {/each}
  </g>
</svg>

<style>
  .graph {
    width: 100%;
    height: 600px;
    border: 1px solid #e2e8f0;
    border-radius: 8px;
    background: white;
  }

  .graph.compact {
    height: 300px;
    cursor: pointer;
  }

  .node {
    cursor: pointer;
    transition: opacity 0.2s;
  }

  .node.faded {
    opacity: 0.2;
  }
</style>
```

> **Note:** The exact import paths for generated protobuf types will depend on what `buf generate` produces. The implementer should check `web/src/lib/api/gen/` and adjust `GraphNode`, `Edge` type imports. The `edgeType` field may be a numeric enum or string depending on the generated code — adjust `edgeTypeName()` accordingly.

- [ ] **Step 2: Verify the component compiles**

```bash
cd web && pnpm svelte-check
```

Expected: No errors (warnings about unused imports are OK at this stage)

- [ ] **Step 3: Commit**

```
jj --no-pager commit -m "feat(web): add Graph component with Dagre layout and SVG rendering"
```

### Task 18: Stats bar, funnel bar, search filter components

**Files:**

- Create: `web/src/lib/components/StatsBar.svelte`
- Create: `web/src/lib/components/FunnelBar.svelte`
- Create: `web/src/lib/components/SearchFilter.svelte`
- Create: `web/src/lib/components/GraphMini.svelte`

- [ ] **Step 1: Create StatsBar component**

Create `web/src/lib/components/StatsBar.svelte`:

```svelte
<script lang="ts">
  interface Props {
    totalSpecs: number;
    readyCount: number;
    driftCount: number;
    decisionCount: number;
  }

  let { totalSpecs, readyCount, driftCount, decisionCount }: Props = $props();

  const cards = $derived([
    { label: 'Specs', value: totalSpecs, color: '#2563eb' },
    { label: 'Ready', value: readyCount, color: '#16a34a' },
    { label: 'Drift', value: driftCount, color: '#dc2626' },
    { label: 'Decisions', value: decisionCount, color: '#7c3aed' },
  ]);
</script>

<div class="stats-bar">
  {#each cards as card}
    <div class="card" style="border-left: 3px solid {card.color}">
      <div class="value">{card.value}</div>
      <div class="label">{card.label}</div>
    </div>
  {/each}
</div>

<style>
  .stats-bar {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1rem;
    margin-bottom: 1.5rem;
  }

  .card {
    background: white;
    padding: 1rem;
    border-radius: 6px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  }

  .value {
    font-size: 1.75rem;
    font-weight: 700;
    color: #1a1a2e;
  }

  .label {
    font-size: 0.85rem;
    color: #64748b;
    margin-top: 0.25rem;
  }
</style>
```

- [ ] **Step 2: Create FunnelBar component**

Create `web/src/lib/components/FunnelBar.svelte`:

```svelte
<script lang="ts">
  interface Props {
    stageCounts: Record<string, number>;
  }

  let { stageCounts }: Props = $props();

  const stages = ['spark', 'shape', 'specify', 'decompose', 'approved', 'done'];
  const colors: Record<string, string> = {
    spark: '#7c3aed',
    shape: '#2563eb',
    specify: '#16a34a',
    decompose: '#d97706',
    approved: '#0d9488',
    done: '#6b7280',
  };

  let total = $derived(stages.reduce((sum, s) => sum + (stageCounts[s] ?? 0), 0));
</script>

<div class="funnel">
  <h3>Authoring Funnel</h3>
  <div class="bar">
    {#each stages as stage}
      {@const count = stageCounts[stage] ?? 0}
      {#if count > 0}
        <div
          class="segment"
          style="flex: {count}; background: {colors[stage]}"
          title="{stage}: {count}"
        >
          {#if count / total > 0.08}
            <span>{stage} ({count})</span>
          {/if}
        </div>
      {/if}
    {/each}
  </div>
  <div class="legend">
    {#each stages as stage}
      <span class="legend-item">
        <span class="dot" style="background: {colors[stage]}"></span>
        {stage}: {stageCounts[stage] ?? 0}
      </span>
    {/each}
  </div>
</div>

<style>
  .funnel {
    background: white;
    padding: 1rem;
    border-radius: 6px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
    margin-bottom: 1.5rem;
  }

  h3 {
    margin: 0 0 0.75rem;
    font-size: 0.9rem;
    color: #64748b;
    font-weight: 600;
  }

  .bar {
    display: flex;
    height: 32px;
    border-radius: 4px;
    overflow: hidden;
  }

  .segment {
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
    font-size: 0.75rem;
    font-weight: 500;
    min-width: 2px;
  }

  .legend {
    display: flex;
    gap: 1rem;
    margin-top: 0.5rem;
    flex-wrap: wrap;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.8rem;
    color: #64748b;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
</style>
```

- [ ] **Step 3: Create SearchFilter component**

Create `web/src/lib/components/SearchFilter.svelte`:

```svelte
<script lang="ts">
  interface Props {
    value: string;
    onchange: (value: string) => void;
  }

  let { value, onchange }: Props = $props();
</script>

<div class="search">
  <input
    type="text"
    placeholder="Filter by slug, intent, stage, priority..."
    {value}
    oninput={(e) => onchange(e.currentTarget.value)}
  />
</div>

<style>
  .search {
    margin-bottom: 1rem;
  }

  input {
    width: 100%;
    padding: 0.5rem 0.75rem;
    border: 1px solid #e2e8f0;
    border-radius: 6px;
    font-size: 0.9rem;
    outline: none;
    box-sizing: border-box;
  }

  input:focus {
    border-color: #2563eb;
    box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.1);
  }
</style>
```

- [ ] **Step 4: Create GraphMini component**

Create `web/src/lib/components/GraphMini.svelte`:

```svelte
<script lang="ts">
  import { goto } from '$app/navigation';
  import Graph from './Graph.svelte';
  import type { GraphNode, Edge } from '../api/gen/specgraph/v1/graph_pb';

  interface Props {
    nodes: GraphNode[];
    edges: Edge[];
  }

  let { nodes, edges }: Props = $props();
</script>

<!-- Use div + click handler instead of <a> to avoid nested <a> tags
     (Graph.svelte renders <a> around each node for click-to-navigate) -->
<div class="mini-wrapper" onclick={() => goto('/graph')} role="link" tabindex="0"
     onkeydown={(e) => e.key === 'Enter' && goto('/graph')}>
  <h3>Graph Preview</h3>
  <Graph {nodes} {edges} compact />
</div>

<style>
  .mini-wrapper {
    display: block;
    cursor: pointer;
    background: white;
    padding: 1rem;
    border-radius: 6px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  }

  h3 {
    margin: 0 0 0.5rem;
    font-size: 0.9rem;
    color: #64748b;
    font-weight: 600;
  }
</style>
```

> **Note:** Same caveat about generated type imports — adjust paths based on actual `buf generate` output.

- [ ] **Step 5: Commit**

```
jj --no-pager commit -m "feat(web): add StatsBar, FunnelBar, SearchFilter, GraphMini components"
```

### Task 19: Dashboard page

**Files:**

- Create: `web/src/routes/+page.svelte`

- [ ] **Step 1: Create dashboard page**

Create `web/src/routes/+page.svelte`:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { graphClient, specClient, decisionClient, lifecycleClient } from '$lib/api/client';
  import type { GraphNode, Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
  import StatsBar from '$lib/components/StatsBar.svelte';
  import FunnelBar from '$lib/components/FunnelBar.svelte';
  import GraphMini from '$lib/components/GraphMini.svelte';

  let totalSpecs = $state(0);
  let readyCount = $state(0);
  let driftCount = $state(0);
  let decisionCount = $state(0);
  let stageCounts = $state<Record<string, number>>({});
  let graphNodes = $state<GraphNode[]>([]);
  let graphEdges = $state<Edge[]>([]);
  let loading = $state(true);

  onMount(async () => {
    try {
      const [specsResp, readyResp, graphResp, decisionsResp, driftResp] = await Promise.all([
        specClient.listSpecs({}),
        graphClient.getReady({}),
        graphClient.getFullGraph({}),
        decisionClient.listDecisions({}),
        lifecycleClient.checkDrift({ slug: '' }),
      ]);

      const specs = specsResp.specs ?? [];
      totalSpecs = specs.length;
      readyCount = (readyResp.ready ?? []).length;
      decisionCount = (decisionsResp.decisions ?? []).length;

      // Drift count: count reports that have items (actual drift detected)
      const reports = driftResp.reports ?? [];
      driftCount = reports.filter((r) => (r.items?.length ?? 0) > 0).length;

      // Count specs per stage
      const counts: Record<string, number> = {};
      for (const s of specs) {
        const stage = s.stage ?? 'unknown';
        counts[stage] = (counts[stage] ?? 0) + 1;
      }
      stageCounts = counts;

      graphNodes = graphResp.nodes ?? [];
      graphEdges = graphResp.edges ?? [];
    } catch (err) {
      console.error('Failed to load dashboard data:', err);
    } finally {
      loading = false;
    }
  });
</script>

{#if loading}
  <p>Loading...</p>
{:else}
  <StatsBar {totalSpecs} {readyCount} {driftCount} {decisionCount} />
  <FunnelBar {stageCounts} />
  <GraphMini nodes={graphNodes} edges={graphEdges} />
{/if}
```

> **Note:** The exact method names and response field names depend on the generated TypeScript client. The implementer should verify against the generated code. The `projectHeader` is no longer needed per-call because the `projectInterceptor` in `client.ts` injects it on every request automatically. The `driftResp.reports` field structure depends on the generated `DriftCheckResponse` — verify field names match.

- [ ] **Step 2: Verify dev build**

```bash
cd web && pnpm build
```

Expected: Build succeeds (static output in `web/build/`)

- [ ] **Step 3: Commit**

```
jj --no-pager commit -m "feat(web): add dashboard page with stats, funnel, and mini graph"
```

---

## Chunk 4: Frontend — Graph Page, Detail Pages, Final Wiring

### Task 20: Full graph page

**Files:**

- Create: `web/src/routes/graph/+page.svelte`

- [ ] **Step 1: Create graph page**

Create `web/src/routes/graph/+page.svelte`:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { graphClient } from '$lib/api/client';
  import type { GraphNode, Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
  import Graph from '$lib/components/Graph.svelte';
  import SearchFilter from '$lib/components/SearchFilter.svelte';

  let nodes = $state<GraphNode[]>([]);
  let edges = $state<Edge[]>([]);
  let filterText = $state('');
  let loading = $state(true);

  onMount(async () => {
    try {
      const resp = await graphClient.getFullGraph({});
      nodes = resp.nodes ?? [];
      edges = resp.edges ?? [];
    } catch (err) {
      console.error('Failed to load graph:', err);
    } finally {
      loading = false;
    }
  });
</script>

<h1>Dependency Graph</h1>

{#if loading}
  <p>Loading graph...</p>
{:else if nodes.length === 0}
  <p>No specs or decisions found. Create some specs first.</p>
{:else}
  <SearchFilter value={filterText} onchange={(v) => (filterText = v)} />
  <Graph {nodes} {edges} {filterText} />
{/if}

<style>
  h1 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0 0 1rem;
  }
</style>
```

- [ ] **Step 2: Commit**

```
jj --no-pager commit -m "feat(web): add full graph page with search filter"
```

### Task 21: Spec detail page

**Files:**

- Create: `web/src/routes/spec/[slug]/+page.svelte`

- [ ] **Step 1: Create spec detail page**

Create `web/src/routes/spec/[slug]/+page.svelte`:

```svelte
<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { specClient } from '$lib/api/client';

  let spec = $state<any>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);

  let slug = $derived($page.params.slug);

  onMount(async () => {
    try {
      const resp = await specClient.getSpec({ slug });
      spec = resp.spec ?? resp;
    } catch (err: any) {
      error = err.message ?? 'Failed to load spec';
    } finally {
      loading = false;
    }
  });
</script>

<nav class="breadcrumb">
  <a href="/">Dashboard</a> / <a href="/graph">Graph</a> / <span>{slug}</span>
</nav>

{#if loading}
  <p>Loading...</p>
{:else if error}
  <p class="error">{error}</p>
{:else if spec}
  <h1>{spec.slug}</h1>

  <table class="meta">
    <tr><td>Intent</td><td>{spec.intent}</td></tr>
    <tr><td>Stage</td><td><span class="badge">{spec.stage}</span></td></tr>
    <tr><td>Priority</td><td>{spec.priority}</td></tr>
    <tr><td>Complexity</td><td>{spec.complexity || '—'}</td></tr>
    <tr><td>Version</td><td>{spec.version}</td></tr>
  </table>

  {#if spec.notes}
    <section>
      <h2>Notes</h2>
      <p class="notes">{spec.notes}</p>
    </section>
  {/if}
{/if}

<style>
  .breadcrumb {
    font-size: 0.85rem;
    color: #64748b;
    margin-bottom: 1rem;
  }

  .breadcrumb a {
    color: #2563eb;
    text-decoration: none;
  }

  h1 {
    font-size: 1.5rem;
    font-weight: 600;
    margin: 0 0 1rem;
  }

  h2 {
    font-size: 1rem;
    font-weight: 600;
    margin: 1.5rem 0 0.5rem;
    color: #334155;
  }

  .meta {
    width: 100%;
    border-collapse: collapse;
    background: white;
    border-radius: 6px;
    overflow: hidden;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  }

  .meta td {
    padding: 0.5rem 1rem;
    border-bottom: 1px solid #f1f5f9;
  }

  .meta td:first-child {
    font-weight: 500;
    color: #64748b;
    width: 120px;
  }

  .badge {
    display: inline-block;
    padding: 0.15rem 0.5rem;
    border-radius: 9999px;
    background: #e2e8f0;
    font-size: 0.8rem;
    font-weight: 500;
  }

  .notes {
    white-space: pre-wrap;
    background: white;
    padding: 1rem;
    border-radius: 6px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  }

  .error {
    color: #dc2626;
  }
</style>
```

- [ ] **Step 2: Commit**

```
jj --no-pager commit -m "feat(web): add spec detail page"
```

### Task 22: Decision detail page

**Files:**

- Create: `web/src/routes/decision/[slug]/+page.svelte`

- [ ] **Step 1: Create decision detail page**

Create `web/src/routes/decision/[slug]/+page.svelte`:

```svelte
<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { decisionClient } from '$lib/api/client';

  let decision = $state<any>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);

  let slug = $derived($page.params.slug);

  onMount(async () => {
    try {
      const resp = await decisionClient.getDecision({ slug });
      decision = resp.decision ?? resp;
    } catch (err: any) {
      error = err.message ?? 'Failed to load decision';
    } finally {
      loading = false;
    }
  });
</script>

<nav class="breadcrumb">
  <a href="/">Dashboard</a> / <a href="/graph">Graph</a> / <span>{slug}</span>
</nav>

{#if loading}
  <p>Loading...</p>
{:else if error}
  <p class="error">{error}</p>
{:else if decision}
  <h1>{decision.slug}</h1>

  <table class="meta">
    <tr><td>Title</td><td>{decision.title}</td></tr>
    <tr><td>Status</td><td><span class="badge">{decision.status}</span></td></tr>
    <tr><td>Version</td><td>{decision.version}</td></tr>
  </table>

  {#if decision.decisionText}
    <section>
      <h2>Decision</h2>
      <p class="content">{decision.decisionText}</p>
    </section>
  {/if}

  {#if decision.rationale}
    <section>
      <h2>Rationale</h2>
      <p class="content">{decision.rationale}</p>
    </section>
  {/if}

  {#if decision.context}
    <section>
      <h2>Context</h2>
      <p class="content">{decision.context}</p>
    </section>
  {/if}
{/if}

<style>
  .breadcrumb {
    font-size: 0.85rem;
    color: #64748b;
    margin-bottom: 1rem;
  }

  .breadcrumb a {
    color: #2563eb;
    text-decoration: none;
  }

  h1 {
    font-size: 1.5rem;
    font-weight: 600;
    margin: 0 0 1rem;
  }

  h2 {
    font-size: 1rem;
    font-weight: 600;
    margin: 1.5rem 0 0.5rem;
    color: #334155;
  }

  .meta {
    width: 100%;
    border-collapse: collapse;
    background: white;
    border-radius: 6px;
    overflow: hidden;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  }

  .meta td {
    padding: 0.5rem 1rem;
    border-bottom: 1px solid #f1f5f9;
  }

  .meta td:first-child {
    font-weight: 500;
    color: #64748b;
    width: 120px;
  }

  .badge {
    display: inline-block;
    padding: 0.15rem 0.5rem;
    border-radius: 9999px;
    background: #e2e8f0;
    font-size: 0.8rem;
    font-weight: 500;
  }

  .content {
    white-space: pre-wrap;
    background: white;
    padding: 1rem;
    border-radius: 6px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  }

  .error {
    color: #dc2626;
  }
</style>
```

- [ ] **Step 2: Commit**

```
jj --no-pager commit -m "feat(web): add decision detail page"
```

### Task 23: Build and verify full SPA

- [ ] **Step 1: Run SvelteKit type check**

```bash
cd web && pnpm svelte-check
```

Expected: No errors

- [ ] **Step 2: Build production output**

```bash
cd web && pnpm build
```

Expected: `web/build/` contains `index.html`, `_app/` with JS/CSS bundles

- [ ] **Step 3: Commit**

```
jj --no-pager commit -m "chore(web): verify SvelteKit production build"
```

### Task 24: Integration test — `GetFullGraph` in e2e API suite

**Files:**

- Modify: `e2e/api/graph_test.go`

The existing file uses Ginkgo `Describe`/`It` blocks with `newSpecClient()` and `newGraphClient()` helpers. Add the new test inside the existing `Describe("graph queries", ...)` block.

- [ ] **Step 1: Add GetFullGraph e2e test**

Add a new `It` block inside the existing `Describe("graph queries", Ordered, ...)`:

```go
It("returns all nodes and edges via GetFullGraph", func() {
    // Create two specs
    _, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
        Slug: "gfg-parent", Intent: "Parent spec", Priority: "p1",
    }))
    Expect(err).NotTo(HaveOccurred())
    _, err = specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
        Slug: "gfg-child", Intent: "Child spec", Priority: "p2",
    }))
    Expect(err).NotTo(HaveOccurred())

    // Add dependency edge
    _, err = graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
        FromSlug: "gfg-child", ToSlug: "gfg-parent",
        EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
    }))
    Expect(err).NotTo(HaveOccurred())

    // GetFullGraph
    resp, err := graphClient.GetFullGraph(ctx, connect.NewRequest(&specv1.GetFullGraphRequest{}))
    Expect(err).NotTo(HaveOccurred())
    Expect(len(resp.Msg.Nodes)).To(BeNumerically(">=", 2))

    // Find our nodes
    slugs := make([]string, 0)
    for _, n := range resp.Msg.Nodes {
        slugs = append(slugs, n.Slug)
    }
    Expect(slugs).To(ContainElements("gfg-parent", "gfg-child"))

    // Verify edge exists and all edge types are user-facing
    Expect(len(resp.Msg.Edges)).To(BeNumerically(">=", 1))
    validEdgeTypes := map[specv1.EdgeType]bool{
        specv1.EdgeType_EDGE_TYPE_DEPENDS_ON:  true,
        specv1.EdgeType_EDGE_TYPE_BLOCKS:      true,
        specv1.EdgeType_EDGE_TYPE_COMPOSES:    true,
        specv1.EdgeType_EDGE_TYPE_RELATES_TO:  true,
        specv1.EdgeType_EDGE_TYPE_INFORMS:     true,
        specv1.EdgeType_EDGE_TYPE_DECIDED_IN:  true,
        specv1.EdgeType_EDGE_TYPE_SUPERSEDES:  true,
    }
    var foundEdge bool
    for _, e := range resp.Msg.Edges {
        if e.FromId == "gfg-child" && e.ToId == "gfg-parent" {
            foundEdge = true
            Expect(e.EdgeType).To(Equal(specv1.EdgeType_EDGE_TYPE_DEPENDS_ON))
        }
        // All returned edge types must be user-facing (not internal like HAS_CHANGE, HAS_FINDING)
        Expect(validEdgeTypes).To(HaveKey(e.EdgeType), "unexpected edge type: %v", e.EdgeType)
    }
    Expect(foundEdge).To(BeTrue())
})
```

- [ ] **Step 3: Run the e2e test**

Run: `go test -tags e2e -run "GetFullGraph" -v -timeout 5m ./e2e/api/...`
Expected: PASS

- [ ] **Step 4: Commit**

```
jj --no-pager commit -m "test(e2e): add GetFullGraph integration test"
```

### Task 25: Quality gate for Chunks 1–4

- [ ] **Step 1: Run `task check`**

Run: `task check`
Expected: All quality checks pass

- [ ] **Step 2: Run integration tests**

Run: `task test:integration`
Expected: All integration tests pass (including memgraph GetFullGraph)

- [ ] **Step 3: Run e2e tests**

Run: `task test:e2e:api`
Expected: All e2e API tests pass

- [ ] **Step 4: Final commit if any fixes were needed**

```
jj --no-pager commit -m "chore: fix lint and formatting for graph viz UI"
```

---

## Chunk 5: Playwright E2E Tests in Docker

The UI needs end-to-end browser tests that verify the SPA works against a real SpecGraph+Memgraph stack. All containers (Memgraph, SpecGraph server, Playwright) run in a single Docker Compose network — no host port mapping or networking issues.

> **Spec deviation:** The design spec said "No JS test framework in V1." This was overridden by explicit request — Playwright e2e tests are in scope for this plan.

### Architecture

```
docker-compose.e2e.yaml
├── memgraph        (memgraph/memgraph-platform:2.4.0)
├── specgraph       (built from repo Dockerfile + web assets)
└── playwright      (mcr.microsoft.com/playwright, runs tests)
    └── mounts: e2e/ui/ (test files)
```

All three containers share a Docker network. Playwright hits `http://specgraph:9090` directly. No ports exposed to host. `task e2e` runs all e2e suites including Playwright.

### Task 26: Create Playwright test project

**Files:**

- Create: `e2e/ui/package.json`
- Create: `e2e/ui/playwright.config.ts`
- Create: `e2e/ui/tsconfig.json`

- [ ] **Step 1: Create Playwright package.json**

Create `e2e/ui/package.json`:

```json
{
  "name": "specgraph-e2e-ui",
  "version": "0.0.1",
  "private": true,
  "scripts": {
    "test": "playwright test",
    "test:headed": "playwright test --headed"
  },
  "devDependencies": {
    "@playwright/test": "^1.50.0"
  }
}
```

- [ ] **Step 2: Create Playwright config**

Create `e2e/ui/playwright.config.ts`:

```ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  retries: 1,
  use: {
    // Inside Docker Compose, specgraph is accessible via service name.
    // Override with SPECGRAPH_BASE_URL env var for local dev.
    baseURL: process.env.SPECGRAPH_BASE_URL ?? 'http://specgraph:9090',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'test-results/html' }]],
});
```

- [ ] **Step 3: Create tsconfig**

Create `e2e/ui/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  }
}
```

- [ ] **Step 4: Install dependencies**

```bash
cd e2e/ui && pnpm install
```

- [ ] **Step 5: Commit**

```
jj --no-pager commit -m "feat(e2e): scaffold Playwright test project for UI e2e"
```

### Task 27: Write Playwright UI tests

**Files:**

- Create: `e2e/ui/tests/navigation.spec.ts`
- Create: `e2e/ui/tests/graph.spec.ts`
- Create: `e2e/ui/tests/detail.spec.ts`
- Create: `e2e/ui/tests/helpers.ts`

- [ ] **Step 1: Create test helpers**

Create `e2e/ui/tests/helpers.ts`:

```ts
import { type Page, expect } from '@playwright/test';

const PROJECT = 'e2e-ui-test';
const BASE_HEADERS = { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1', 'X-Specgraph-Project': PROJECT };

// Seed test data via ConnectRPC calls directly to the server.
// This avoids coupling UI tests to CLI tooling.
// Seed helpers use raw HTTP POST with proto JSON field names (snake_case).
export async function seedSpec(page: Page, slug: string, intent: string, priority = 'p2'): Promise<void> {
  const resp = await page.request.post('/specgraph.v1.SpecService/CreateSpec', {
    headers: BASE_HEADERS,
    data: { slug, intent, priority },
  });
  expect(resp.ok()).toBeTruthy();
}

export async function seedEdge(page: Page, fromSlug: string, toSlug: string): Promise<void> {
  const resp = await page.request.post('/specgraph.v1.GraphService/AddEdge', {
    headers: BASE_HEADERS,
    data: { from_slug: fromSlug, to_slug: toSlug, edge_type: 'EDGE_TYPE_DEPENDS_ON' },
  });
  expect(resp.ok()).toBeTruthy();
}

export async function seedDecision(page: Page, slug: string, title: string): Promise<void> {
  const resp = await page.request.post('/specgraph.v1.DecisionService/CreateDecision', {
    headers: BASE_HEADERS,
    data: { slug, title, decision: 'Test decision text', rationale: 'Test rationale' },
  });
  expect(resp.ok()).toBeTruthy();
}
```

- [ ] **Step 2: Create navigation test**

Create `e2e/ui/tests/navigation.spec.ts`:

```ts
import { test, expect } from '@playwright/test';

test.describe('Navigation', () => {
  test('loads the dashboard at /', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.locator('nav')).toContainText('Dashboard');
    await expect(page.locator('nav')).toContainText('Graph');
    await expect(page.locator('.brand')).toContainText('SpecGraph');
  });

  test('navigates from dashboard to graph view', async ({ page }) => {
    await page.goto('/');
    await page.click('nav a[href="/graph"]');
    await expect(page).toHaveURL('/graph');
    await expect(page.locator('h1')).toContainText('Dependency Graph');
  });

  test('deep links work for /graph', async ({ page }) => {
    await page.goto('/graph');
    await expect(page.locator('h1')).toContainText('Dependency Graph');
  });
});
```

- [ ] **Step 3: Create graph view test**

Create `e2e/ui/tests/graph.spec.ts`:

```ts
import { test, expect } from '@playwright/test';
import { seedSpec, seedEdge } from './helpers';

test.describe('Graph View', () => {
  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await seedSpec(page, 'ui-parent', 'Parent feature');
    await seedSpec(page, 'ui-child', 'Child feature');
    await seedEdge(page, 'ui-child', 'ui-parent');
    await page.close();
  });

  test('renders SVG graph with nodes', async ({ page }) => {
    await page.goto('/graph');
    // Wait for graph to load (SVG should appear)
    const svg = page.locator('svg.graph');
    await expect(svg).toBeVisible({ timeout: 10_000 });

    // Nodes should be rendered
    const nodes = page.locator('svg.graph .node');
    await expect(nodes).not.toHaveCount(0);
  });

  test('search filter fades non-matching nodes', async ({ page }) => {
    await page.goto('/graph');
    await expect(page.locator('svg.graph')).toBeVisible({ timeout: 10_000 });

    // Type in search filter
    await page.fill('input[placeholder*="Filter"]', 'ui-parent');

    // The matching node should not be faded, non-matching should be
    const fadedNodes = page.locator('svg.graph .node.faded');
    await expect(fadedNodes).not.toHaveCount(0);
  });

  test('clicking a node navigates to detail page', async ({ page }) => {
    await page.goto('/graph');
    await expect(page.locator('svg.graph')).toBeVisible({ timeout: 10_000 });

    // Click on a spec node link
    const nodeLink = page.locator('svg.graph a[href*="/spec/"]').first();
    await nodeLink.click();
    await expect(page).toHaveURL(/\/spec\//);
  });
});
```

- [ ] **Step 4: Create detail page test**

Create `e2e/ui/tests/detail.spec.ts`:

```ts
import { test, expect } from '@playwright/test';
import { seedSpec, seedDecision } from './helpers';

test.describe('Detail Pages', () => {
  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await seedSpec(page, 'detail-spec', 'Detail test spec', 'p1');
    await seedDecision(page, 'detail-dec', 'Detail test decision');
    await page.close();
  });

  test('spec detail page shows metadata', async ({ page }) => {
    await page.goto('/spec/detail-spec');
    await expect(page.locator('h1')).toContainText('detail-spec');
    await expect(page.locator('.meta')).toContainText('Detail test spec');
    await expect(page.locator('.meta')).toContainText('p1');

    // Breadcrumb navigation works
    await expect(page.locator('.breadcrumb')).toContainText('Dashboard');
    await expect(page.locator('.breadcrumb')).toContainText('Graph');
  });

  test('decision detail page shows metadata', async ({ page }) => {
    await page.goto('/decision/detail-dec');
    await expect(page.locator('h1')).toContainText('detail-dec');
    await expect(page.locator('.meta')).toContainText('Detail test decision');
  });

  test('spec detail 404 shows error', async ({ page }) => {
    await page.goto('/spec/nonexistent-slug-xyz');
    await expect(page.locator('.error')).toBeVisible({ timeout: 10_000 });
  });
});
```

- [ ] **Step 5: Commit**

```
jj --no-pager commit -m "test(e2e): add Playwright UI tests for navigation, graph, and detail pages"
```

### Task 28: Docker Compose for e2e (specgraph + memgraph + playwright)

**Files:**

- Create: `e2e/ui/docker-compose.e2e.yaml`
- Create: `e2e/ui/Dockerfile.playwright`

- [ ] **Step 1: Create Playwright Dockerfile**

Create `e2e/ui/Dockerfile.playwright`:

```dockerfile
FROM mcr.microsoft.com/playwright:v1.50.0-noble

WORKDIR /tests
COPY package.json pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY . .

CMD ["pnpm", "test"]
```

- [ ] **Step 2: Create e2e Docker Compose**

Create `e2e/ui/docker-compose.e2e.yaml`:

```yaml
# E2E test stack: Memgraph + SpecGraph server + Playwright
# All containers share a Docker network. No host ports needed.
# Usage: docker compose -f e2e/ui/docker-compose.e2e.yaml up --build --abort-on-container-exit
services:
  memgraph:
    image: memgraph/memgraph-platform:2.4.0
    command: ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
    environment:
      - MEMGRAPH=--storage-wal-enabled=true --log-level=WARNING
    healthcheck:
      test: ["CMD-SHELL", "echo 'RETURN 1;' | mgconsole || exit 1"]
      interval: 3s
      timeout: 10s
      retries: 10

  specgraph:
    build:
      context: ../..
      dockerfile: Dockerfile.e2e
    command: ["serve"]
    depends_on:
      memgraph:
        condition: service_healthy
    environment:
      - XDG_CONFIG_HOME=/etc
    volumes:
      - ./config.e2e.yaml:/etc/specgraph/config.yaml:ro
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O- --post-data='{}' --header='Content-Type: application/json' --header='Connect-Protocol-Version: 1' http://localhost:9090/specgraph.v1.ServerService/Health || exit 1"]
      interval: 3s
      timeout: 5s
      retries: 10

  playwright:
    build:
      context: .
      dockerfile: Dockerfile.playwright
    depends_on:
      specgraph:
        condition: service_healthy
    environment:
      - SPECGRAPH_BASE_URL=http://specgraph:9090
    volumes:
      - ./test-results:/tests/test-results
```

- [ ] **Step 3: Create e2e server config**

Create `e2e/ui/config.e2e.yaml`:

```yaml
server:
  listen: "0.0.0.0:9090"
  backend: memgraph
  memgraph:
    bolt_uri: bolt://memgraph:7687
```

- [ ] **Step 4: Create multi-stage Dockerfile for e2e**

The existing `Dockerfile` copies a pre-built `specgraph` binary. For e2e, we need to build the Go binary with the embedded SvelteKit UI inside Docker. Create a multi-stage Dockerfile:

Create `Dockerfile.e2e` at repo root:

```dockerfile
# Stage 1: Build SvelteKit UI
FROM node:22-alpine AS web-builder
RUN corepack enable
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# Stage 2: Build Go binary with embedded UI
FROM golang:1.25-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overwrite web/build with the freshly built SvelteKit output
COPY --from=web-builder /app/web/build web/build
RUN CGO_ENABLED=0 go build -o specgraph ./cmd/specgraph

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget
COPY --from=go-builder /app/specgraph /usr/local/bin/specgraph
EXPOSE 9090
ENTRYPOINT ["specgraph"]
```

> The compose file (Step 2) already references `Dockerfile.e2e`.

- [ ] **Step 5: Commit**

```
jj --no-pager commit -m "feat(e2e): add Docker Compose stack for Playwright UI e2e tests"
```

### Task 29: Add Taskfile tasks for UI e2e

**Files:**

- Modify: `Taskfile.yml`

- [ ] **Step 1: Add UI e2e tasks**

Add to `Taskfile.yml`:

```yaml
  test:e2e:ui:
    desc: Run Playwright UI E2E tests (all in Docker, no host ports needed)
    cmds:
      - |
        docker compose -f e2e/ui/docker-compose.e2e.yaml up --build --abort-on-container-exit --exit-code-from playwright
        rc=$?
        docker compose -f e2e/ui/docker-compose.e2e.yaml down -v
        exit $rc
  test:e2e:ui:clean:
    desc: Clean up UI E2E Docker resources
    cmds:
      - docker compose -f e2e/ui/docker-compose.e2e.yaml down -v --rmi local
```

- [ ] **Step 2: Update `test:e2e` to include UI tests**

Modify the existing `test:e2e` task to include the new UI e2e suite:

```yaml
  test:e2e:
    desc: Run all E2E tests (requires Docker)
    cmds:
      - task: test:e2e:api
      - task: test:e2e:cli
      - task: test:e2e:docker
      - task: test:e2e:ui
```

- [ ] **Step 3: Commit**

```
jj --no-pager commit -m "chore(taskfile): add test:e2e:ui task, include in test:e2e"
```

### Task 30: Update `.gitignore` for Playwright artifacts

**Files:**

- Modify: `.gitignore`

- [ ] **Step 1: Add Playwright entries to `.gitignore`**

Add to `.gitignore`:

```
# Playwright
e2e/ui/test-results/
e2e/ui/node_modules/
```

- [ ] **Step 2: Commit**

```
jj --no-pager commit -m "chore: add Playwright e2e artifacts to .gitignore"
```

### Task 31: Run full e2e suite

- [ ] **Step 1: Run `task check`**

Run: `task check`
Expected: All quality checks pass

- [ ] **Step 2: Run UI e2e tests**

Run: `task test:e2e:ui`
Expected: Docker Compose builds all three containers, Playwright runs tests, all pass

- [ ] **Step 3: Run full e2e suite**

Run: `task test:e2e`
Expected: All e2e suites pass (API, CLI, Docker, UI)

- [ ] **Step 4: Final commit if any fixes were needed**

```
jj --no-pager commit -m "chore: fix issues from full e2e run"
```
