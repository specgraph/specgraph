# SpecGraph Vertical Slice Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prove the client/server architecture end-to-end: init → serve → create spec → list specs → show spec, with Memgraph storage and ConnectRPC API.

**Architecture:** Go CLI client talks to a ConnectRPC server that stores specs as graph nodes in Memgraph. `specgraph serve` manages a Docker Compose stack (server + Memgraph). The CLI is a thin ConnectRPC client.

**Tech Stack:** Go 1.26, ConnectRPC (buf/connect-go), Memgraph (neo4j-go-driver v5), Cobra (CLI), buf 1.66 (proto codegen), testcontainers-go (integration tests), Docker Compose

**Design Doc:** `docs/plans/2026-02-28-client-server-architecture-design.md`

---

## Project Structure

```text
specgraph/
├── proto/
│   └── specgraph/v1/
│       └── spec.proto              # Spec messages + SpecService
├── gen/
│   └── specgraph/v1/               # Generated Go code (buf)
│       ├── spec.pb.go
│       └── specgraphv1connect/
│           └── spec.connect.go
├── cmd/
│   └── specgraph/
│       └── main.go                 # CLI entrypoint
├── internal/
│   ├── server/
│   │   ├── server.go               # HTTP server + ConnectRPC handler registration
│   │   └── spec_handler.go         # SpecService implementation
│   ├── storage/
│   │   ├── storage.go              # Backend interface
│   │   └── memgraph/
│   │       ├── memgraph.go         # Memgraph implementation
│   │       └── memgraph_test.go    # Integration tests (testcontainers)
│   ├── config/
│   │   └── config.go               # YAML config loading
│   └── docker/
│       └── compose.go              # Docker compose management
├── docker/
│   └── docker-compose.memgraph.yaml
├── Dockerfile                       # Server image
├── buf.yaml
├── buf.gen.yaml
├── go.mod
└── go.sum
```

---

## Task 1: Project Scaffold

**Files:**

- Create: `go.mod`
- Create: `buf.yaml`
- Create: `buf.gen.yaml`
- Create: `.gitignore` (update existing)
- Create: `cmd/specgraph/main.go`

**Step 1: Initialize Go module**

```bash
cd /Volumes/Code/github.com/seanb4t/beads/specgraph
go mod init github.com/seanb4t/specgraph
```

**Step 2: Create buf configuration**

`buf.yaml`:

```yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

`buf.gen.yaml`:

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: paths=source_relative
  - remote: buf.build/connectrpc/go
    out: gen
    opt: paths=source_relative
```

**Step 3: Create minimal CLI entrypoint**

`cmd/specgraph/main.go`:

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("specgraph")
	os.Exit(0)
}
```

**Step 4: Update .gitignore**

Append to existing `.gitignore`:

```text
# Go
/specgraph
*.exe

# Generated protobuf
gen/
```

**Step 5: Verify it builds**

```bash
go build -o specgraph ./cmd/specgraph
./specgraph
```

Expected: prints `specgraph`

**Step 6: Commit**

```bash
git add go.mod buf.yaml buf.gen.yaml cmd/ .gitignore
git commit -m "feat: project scaffold with go module and buf config"
```

---

## Task 2: Protobuf Schema — Spec Messages + SpecService

**Files:**

- Create: `proto/specgraph/v1/spec.proto`

**Step 1: Write the proto file**

`proto/specgraph/v1/spec.proto`:

```protobuf
syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/seanb4t/specgraph/gen/specgraph/v1;specgraphv1";

import "google/protobuf/timestamp.proto";

// --- Messages ---

message Spec {
  string id = 1;           // content-addressable, e.g. "spec-k7m3p"
  string slug = 2;         // human-readable, e.g. "oauth-refresh-rotation"
  string intent = 3;       // what this spec is about
  string stage = 4;        // spark | shape | specify | decompose | approved | in_progress | done
  string priority = 5;     // p0 | p1 | p2 | p3
  string complexity = 6;   // low | medium | high
  int32 version = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
}

message CreateSpecRequest {
  string slug = 1;
  string intent = 2;
  string priority = 3;     // optional, defaults to "p2"
  string complexity = 4;   // optional, defaults to "medium"
}

message GetSpecRequest {
  string slug = 1;
}

message ListSpecsRequest {
  string stage = 1;        // optional filter
  string priority = 2;     // optional filter
  int32 limit = 3;         // optional, defaults to 50
}

message ListSpecsResponse {
  repeated Spec specs = 1;
}

// --- Service ---

service SpecService {
  rpc CreateSpec(CreateSpecRequest) returns (Spec);
  rpc GetSpec(GetSpecRequest) returns (Spec);
  rpc ListSpecs(ListSpecsRequest) returns (ListSpecsResponse);
}
```

**Step 2: Generate Go code**

```bash
buf generate
```

Expected: generates files in `gen/specgraph/v1/`

**Step 3: Verify generated code compiles**

```bash
go mod tidy
go build ./gen/...
```

**Step 4: Commit**

```bash
git add proto/ gen/ go.mod go.sum
git commit -m "feat: protobuf schema for Spec messages and SpecService"
```

---

## Task 3: Storage Interface

**Files:**

- Create: `internal/storage/storage.go`

**Step 1: Define the storage interface**

`internal/storage/storage.go`:

```go
package storage

import (
	"context"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// Backend is the interface that all storage backends must implement.
// The vertical slice covers CRUD only. Graph queries (deps, critical path,
// impact) will be added in later tasks.
type Backend interface {
	// CreateSpec stores a new spec and returns it with generated ID and timestamps.
	CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*specv1.Spec, error)

	// GetSpec retrieves a spec by slug.
	GetSpec(ctx context.Context, slug string) (*specv1.Spec, error)

	// ListSpecs returns specs matching the given filters.
	// Empty filter values mean "no filter".
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*specv1.Spec, error)

	// Close releases any resources held by the backend.
	Close(ctx context.Context) error
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/storage/
```

**Step 3: Commit**

```bash
git add internal/storage/storage.go
git commit -m "feat: storage backend interface for spec CRUD"
```

---

## Task 4: Memgraph Storage Backend

**Files:**

- Create: `internal/storage/memgraph/memgraph.go`
- Create: `internal/storage/memgraph/memgraph_test.go`

**Step 1: Install dependencies**

```bash
go get github.com/neo4j/neo4j-go-driver/v5
go get github.com/testcontainers/testcontainers-go
```

**Step 2: Write the integration test first**

`internal/storage/memgraph/memgraph_test.go`:

```go
package memgraph_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupMemgraph(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "memgraph/memgraph:latest",
			ExposedPorts: []string{"7687/tcp"},
			WaitingFor:   wait.ForListeningPort("7687/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to start memgraph container: %v", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "7687/tcp")
	uri := "bolt://" + host + ":" + port.Port()

	cleanup := func() {
		_ = container.Terminate(ctx)
	}
	return uri, cleanup
}

func TestCreateAndGetSpec(t *testing.T) {
	uri, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, uri)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer store.Close(ctx)

	// Create
	spec, err := store.CreateSpec(ctx, "login-api", "REST endpoint for OAuth2 login", "p1", "medium")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if spec.Slug != "login-api" {
		t.Errorf("slug = %q, want %q", spec.Slug, "login-api")
	}
	if spec.Id == "" {
		t.Error("expected non-empty ID")
	}
	if spec.Stage != "spark" {
		t.Errorf("stage = %q, want %q", spec.Stage, "spark")
	}

	// Get
	got, err := store.GetSpec(ctx, "login-api")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.Id != spec.Id {
		t.Errorf("id = %q, want %q", got.Id, spec.Id)
	}
	if got.Intent != "REST endpoint for OAuth2 login" {
		t.Errorf("intent = %q, want %q", got.Intent, "REST endpoint for OAuth2 login")
	}
}

func TestListSpecs(t *testing.T) {
	uri, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, uri)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer store.Close(ctx)

	// Create two specs
	_, _ = store.CreateSpec(ctx, "spec-a", "First spec", "p1", "low")
	_, _ = store.CreateSpec(ctx, "spec-b", "Second spec", "p2", "high")

	// List all
	specs, err := store.ListSpecs(ctx, "", "", 50)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(specs) != 2 {
		t.Errorf("got %d specs, want 2", len(specs))
	}

	// List by priority
	specs, err = store.ListSpecs(ctx, "", "p1", 50)
	if err != nil {
		t.Fatalf("list with filter failed: %v", err)
	}
	if len(specs) != 1 {
		t.Errorf("got %d specs, want 1", len(specs))
	}
}

func TestGetSpec_NotFound(t *testing.T) {
	uri, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, uri)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer store.Close(ctx)

	_, err = store.GetSpec(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent spec")
	}
}
```

**Step 3: Run the test to verify it fails**

```bash
go test ./internal/storage/memgraph/ -v -count=1
```

Expected: FAIL — `memgraph` package doesn't exist yet

**Step 4: Implement the Memgraph backend**

`internal/storage/memgraph/memgraph.go`:

```go
package memgraph

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Store implements storage.Backend using Memgraph via the Bolt protocol.
type Store struct {
	driver neo4j.DriverWithContext
}

// New creates a new Memgraph store connected to the given Bolt URI.
func New(ctx context.Context, boltURI string) (*Store, error) {
	driver, err := neo4j.NewDriverWithContext(boltURI, neo4j.NoAuth())
	if err != nil {
		return nil, fmt.Errorf("memgraph connect: %w", err)
	}
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("memgraph verify: %w", err)
	}
	return &Store{driver: driver}, nil
}

func generateID(slug string) string {
	h := sha256.Sum256([]byte(slug + time.Now().String()))
	return "spec-" + hex.EncodeToString(h[:])[:7]
}

func (s *Store) CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*specv1.Spec, error) {
	id := generateID(slug)
	now := time.Now().UTC()

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx,
		`CREATE (s:Spec {
			id: $id, slug: $slug, intent: $intent,
			stage: "spark", priority: $priority, complexity: $complexity,
			version: 1, created_at: $now, updated_at: $now
		})`,
		map[string]any{
			"id": id, "slug": slug, "intent": intent,
			"priority": priority, "complexity": complexity,
			"now": now.Format(time.RFC3339),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create spec: %w", err)
	}

	return &specv1.Spec{
		Id:         id,
		Slug:       slug,
		Intent:     intent,
		Stage:      "spark",
		Priority:   priority,
		Complexity: complexity,
		Version:    1,
		CreatedAt:  timestamppb.New(now),
		UpdatedAt:  timestamppb.New(now),
	}, nil
}

func (s *Store) GetSpec(ctx context.Context, slug string) (*specv1.Spec, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		`MATCH (s:Spec {slug: $slug}) RETURN s`,
		map[string]any{"slug": slug},
	)
	if err != nil {
		return nil, fmt.Errorf("get spec: %w", err)
	}

	if !result.Next(ctx) {
		return nil, fmt.Errorf("spec %q not found", slug)
	}

	record := result.Record()
	node, _ := record.Get("s")
	return specFromNode(node.(neo4j.Node)), nil
}

func (s *Store) ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*specv1.Spec, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	query := "MATCH (s:Spec)"
	params := map[string]any{}
	conditions := []string{}

	if stage != "" {
		conditions = append(conditions, "s.stage = $stage")
		params["stage"] = stage
	}
	if priority != "" {
		conditions = append(conditions, "s.priority = $priority")
		params["priority"] = priority
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, c := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += c
		}
	}

	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(" RETURN s LIMIT %d", limit)

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}

	var specs []*specv1.Spec
	for result.Next(ctx) {
		node, _ := result.Record().Get("s")
		specs = append(specs, specFromNode(node.(neo4j.Node)))
	}
	return specs, nil
}

func (s *Store) Close(ctx context.Context) error {
	return s.driver.Close(ctx)
}

func specFromNode(node neo4j.Node) *specv1.Spec {
	props := node.Props
	spec := &specv1.Spec{
		Id:         props["id"].(string),
		Slug:       props["slug"].(string),
		Intent:     props["intent"].(string),
		Stage:      props["stage"].(string),
		Priority:   props["priority"].(string),
		Complexity: props["complexity"].(string),
		Version:    int32(props["version"].(int64)),
	}
	if ts, ok := props["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			spec.CreatedAt = timestamppb.New(t)
		}
	}
	if ts, ok := props["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			spec.UpdatedAt = timestamppb.New(t)
		}
	}
	return spec
}
```

**Step 5: Run the tests**

```bash
go mod tidy
go test ./internal/storage/memgraph/ -v -count=1 -timeout=120s
```

Expected: PASS (all three tests). This requires Docker running (testcontainers pulls and starts Memgraph).

**Step 6: Commit**

```bash
git add internal/storage/memgraph/ go.mod go.sum
git commit -m "feat: memgraph storage backend with integration tests"
```

---

## Task 5: ConnectRPC Server

**Files:**

- Create: `internal/server/server.go`
- Create: `internal/server/spec_handler.go`
- Create: `internal/server/spec_handler_test.go`

**Step 1: Write the handler test first**

`internal/server/spec_handler_test.go`:

```go
package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
)

// mockBackend is a test double for the storage interface.
type mockBackend struct {
	specs map[string]*specv1.Spec
}

func newMockBackend() *mockBackend {
	return &mockBackend{specs: make(map[string]*specv1.Spec)}
}

func (m *mockBackend) CreateSpec(_ context.Context, slug, intent, priority, complexity string) (*specv1.Spec, error) {
	spec := &specv1.Spec{
		Id: "spec-test1", Slug: slug, Intent: intent,
		Stage: "spark", Priority: priority, Complexity: complexity, Version: 1,
	}
	m.specs[slug] = spec
	return spec, nil
}

func (m *mockBackend) GetSpec(_ context.Context, slug string) (*specv1.Spec, error) {
	if s, ok := m.specs[slug]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("spec %q not found", slug)
}

func (m *mockBackend) ListSpecs(_ context.Context, stage, priority string, limit int) ([]*specv1.Spec, error) {
	var result []*specv1.Spec
	for _, s := range m.specs {
		if stage != "" && s.Stage != stage {
			continue
		}
		if priority != "" && s.Priority != priority {
			continue
		}
		result = append(result, s)
	}
	return result, nil
}

func (m *mockBackend) Close(_ context.Context) error { return nil }

// Verify mockBackend satisfies the interface
var _ storage.Backend = (*mockBackend)(nil)

func TestSpecHandler_CreateAndGet(t *testing.T) {
	backend := newMockBackend()
	mux := server.NewMux(backend)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := specgraphv1connect.NewSpecServiceClient(http.DefaultClient, srv.URL)

	// Create
	createResp, err := client.CreateSpec(context.Background(), connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   "test-spec",
		Intent: "A test spec",
	}))
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if createResp.Msg.Slug != "test-spec" {
		t.Errorf("slug = %q, want %q", createResp.Msg.Slug, "test-spec")
	}

	// Get
	getResp, err := client.GetSpec(context.Background(), connect.NewRequest(&specv1.GetSpecRequest{
		Slug: "test-spec",
	}))
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if getResp.Msg.Intent != "A test spec" {
		t.Errorf("intent = %q, want %q", getResp.Msg.Intent, "A test spec")
	}
}

func TestSpecHandler_ListSpecs(t *testing.T) {
	backend := newMockBackend()
	mux := server.NewMux(backend)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := specgraphv1connect.NewSpecServiceClient(http.DefaultClient, srv.URL)

	// Create two specs
	client.CreateSpec(context.Background(), connect.NewRequest(&specv1.CreateSpecRequest{
		Slug: "spec-a", Intent: "First",
	}))
	client.CreateSpec(context.Background(), connect.NewRequest(&specv1.CreateSpecRequest{
		Slug: "spec-b", Intent: "Second",
	}))

	// List
	listResp, err := client.ListSpecs(context.Background(), connect.NewRequest(&specv1.ListSpecsRequest{}))
	if err != nil {
		t.Fatalf("ListSpecs: %v", err)
	}
	if len(listResp.Msg.Specs) != 2 {
		t.Errorf("got %d specs, want 2", len(listResp.Msg.Specs))
	}
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/server/ -v -count=1
```

Expected: FAIL — `server` package doesn't exist

**Step 3: Implement the server and handler**

`internal/server/spec_handler.go`:

```go
package server

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

type SpecHandler struct {
	backend storage.Backend
}

var _ specgraphv1connect.SpecServiceHandler = (*SpecHandler)(nil)

func NewSpecHandler(backend storage.Backend) *SpecHandler {
	return &SpecHandler{backend: backend}
}

func (h *SpecHandler) CreateSpec(ctx context.Context, req *connect.Request[specv1.CreateSpecRequest]) (*connect.Response[specv1.Spec], error) {
	priority := req.Msg.Priority
	if priority == "" {
		priority = "p2"
	}
	complexity := req.Msg.Complexity
	if complexity == "" {
		complexity = "medium"
	}

	spec, err := h.backend.CreateSpec(ctx, req.Msg.Slug, req.Msg.Intent, priority, complexity)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(spec), nil
}

func (h *SpecHandler) GetSpec(ctx context.Context, req *connect.Request[specv1.GetSpecRequest]) (*connect.Response[specv1.Spec], error) {
	spec, err := h.backend.GetSpec(ctx, req.Msg.Slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(spec), nil
}

func (h *SpecHandler) ListSpecs(ctx context.Context, req *connect.Request[specv1.ListSpecsRequest]) (*connect.Response[specv1.ListSpecsResponse], error) {
	specs, err := h.backend.ListSpecs(ctx, req.Msg.Stage, req.Msg.Priority, int(req.Msg.Limit))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ListSpecsResponse{Specs: specs}), nil
}
```

`internal/server/server.go`:

```go
package server

import (
	"net/http"

	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

// NewMux creates an HTTP mux with all ConnectRPC services registered.
func NewMux(backend storage.Backend) *http.ServeMux {
	mux := http.NewServeMux()

	specHandler := NewSpecHandler(backend)
	path, handler := specgraphv1connect.NewSpecServiceHandler(specHandler)
	mux.Handle(path, handler)

	return mux
}
```

**Step 4: Run the tests**

```bash
go get connectrpc.com/connect
go mod tidy
go test ./internal/server/ -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/ go.mod go.sum
git commit -m "feat: ConnectRPC server with SpecService handler"
```

---

## Task 6: Configuration

**Files:**

- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the test first**

`internal/config/config_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanb4t/specgraph/internal/config"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yaml := `server:
  mode: external
  host: "0.0.0.0"
  port: 9090
storage:
  backend: memgraph
  memgraph:
    bolt_uri: "bolt://localhost:7687"
`
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Mode != "external" {
		t.Errorf("mode = %q, want %q", cfg.Server.Mode, "external")
	}
	if cfg.Storage.Backend != "memgraph" {
		t.Errorf("backend = %q, want %q", cfg.Storage.Backend, "memgraph")
	}
	if cfg.Storage.Memgraph.BoltURI != "bolt://localhost:7687" {
		t.Errorf("bolt_uri = %q, want %q", cfg.Storage.Memgraph.BoltURI, "bolt://localhost:7687")
	}
}

func TestLoadConfig_Remote(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yaml := `server:
  remote: "https://specgraph.company.com:9090"
`
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.IsRemote() {
		t.Error("expected remote mode")
	}
	if cfg.Server.Remote != "https://specgraph.company.com:9090" {
		t.Errorf("remote = %q", cfg.Server.Remote)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yaml := `storage:
  backend: memgraph
`
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("port = %d, want %d", cfg.Server.Port, 9090)
	}
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/config/ -v -count=1
```

Expected: FAIL

**Step 3: Implement config loading**

`internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
}

type ServerConfig struct {
	Mode   string `yaml:"mode"`   // docker | external
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	Remote string `yaml:"remote"` // if set, CLI-only mode (no local server)
}

type StorageConfig struct {
	Backend  string         `yaml:"backend"` // memgraph | postgres
	Memgraph MemgraphConfig `yaml:"memgraph"`
	Postgres PostgresConfig `yaml:"postgres"`
	Docker   DockerConfig   `yaml:"docker"`
}

type MemgraphConfig struct {
	BoltURI string `yaml:"bolt_uri"`
}

type PostgresConfig struct {
	URL string `yaml:"url"`
}

type DockerConfig struct {
	ComposeFile string `yaml:"compose_file"`
}

func (c *Config) IsRemote() bool {
	return c.Server.Remote != ""
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(cfg)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9090
	}
	if cfg.Server.Mode == "" && !cfg.IsRemote() {
		cfg.Server.Mode = "docker"
	}
	if cfg.Storage.Memgraph.BoltURI == "" {
		cfg.Storage.Memgraph.BoltURI = "bolt://localhost:7687"
	}
	if cfg.Storage.Docker.ComposeFile == "" {
		cfg.Storage.Docker.ComposeFile = ".specgraph/docker-compose.yaml"
	}
}
```

**Step 4: Run the tests**

```bash
go get gopkg.in/yaml.v3
go mod tidy
go test ./internal/config/ -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: YAML config loading with defaults and remote mode"
```

---

## Task 7: Docker Compose Management

**Files:**

- Create: `internal/docker/compose.go`
- Create: `docker/docker-compose.memgraph.yaml`

**Step 1: Create the Docker Compose template**

`docker/docker-compose.memgraph.yaml`:

```yaml
services:
  specgraph:
    image: ghcr.io/seanb4t/specgraph:latest
    ports:
      - "${SPECGRAPH_PORT:-9090}:9090"
    depends_on:
      memgraph:
        condition: service_healthy
    environment:
      - SPECGRAPH_STORAGE_BACKEND=memgraph
      - SPECGRAPH_STORAGE_BOLT_URI=bolt://memgraph:7687

  memgraph:
    image: memgraph/memgraph:latest
    ports:
      - "${MEMGRAPH_PORT:-7687}:7687"
    volumes:
      - specgraph-data:/var/lib/memgraph
    healthcheck:
      test: ["CMD-SHELL", "echo 'RETURN 1;' | mgconsole || exit 1"]
      interval: 5s
      timeout: 10s
      retries: 5

volumes:
  specgraph-data:
```

**Step 2: Implement compose management**

`internal/docker/compose.go`:

```go
package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ComposeUp starts the Docker Compose stack.
func ComposeUp(composeFile string) error {
	if _, err := os.Stat(composeFile); err != nil {
		return fmt.Errorf("compose file not found: %s", composeFile)
	}

	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d", "--wait")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
	}
	return nil
}

// ComposeDown stops the Docker Compose stack.
func ComposeDown(composeFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down: %w", err)
	}
	return nil
}

// EnsureComposeFile copies the bundled compose template to the project's
// .specgraph directory if it doesn't already exist.
func EnsureComposeFile(projectDir, backend string) (string, error) {
	sgDir := filepath.Join(projectDir, ".specgraph")
	if err := os.MkdirAll(sgDir, 0755); err != nil {
		return "", fmt.Errorf("create .specgraph dir: %w", err)
	}

	dest := filepath.Join(sgDir, "docker-compose.yaml")
	if _, err := os.Stat(dest); err == nil {
		return dest, nil // already exists
	}

	// Template for memgraph — the default
	template := memgraphComposeTemplate
	if backend == "postgres" {
		template = postgresComposeTemplate
	}

	if err := os.WriteFile(dest, []byte(template), 0644); err != nil {
		return "", fmt.Errorf("write compose file: %w", err)
	}
	return dest, nil
}

const memgraphComposeTemplate = `services:
  specgraph:
    image: ghcr.io/seanb4t/specgraph:latest
    ports:
      - "9090:9090"
    depends_on:
      memgraph:
        condition: service_healthy
    environment:
      - SPECGRAPH_STORAGE_BACKEND=memgraph
      - SPECGRAPH_STORAGE_BOLT_URI=bolt://memgraph:7687

  memgraph:
    image: memgraph/memgraph:latest
    ports:
      - "7687:7687"
    volumes:
      - specgraph-data:/var/lib/memgraph
    healthcheck:
      test: ["CMD-SHELL", "echo 'RETURN 1;' | mgconsole || exit 1"]
      interval: 5s
      timeout: 10s
      retries: 5

volumes:
  specgraph-data:
`

const postgresComposeTemplate = `services:
  specgraph:
    image: ghcr.io/seanb4t/specgraph:latest
    ports:
      - "9090:9090"
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      - SPECGRAPH_STORAGE_BACKEND=postgres
      - SPECGRAPH_STORAGE_POSTGRES_URL=postgres://specgraph:specgraph@postgres:5432/specgraph

  postgres:
    image: apache/age:latest
    ports:
      - "5432:5432"
    environment:
      - POSTGRES_USER=specgraph
      - POSTGRES_PASSWORD=specgraph
      - POSTGRES_DB=specgraph
    volumes:
      - specgraph-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U specgraph"]
      interval: 5s
      timeout: 10s
      retries: 5

volumes:
  specgraph-data:
`
```

**Step 3: Verify it compiles**

```bash
go build ./internal/docker/
```

**Step 4: Commit**

```bash
git add internal/docker/ docker/
git commit -m "feat: docker compose management for specgraph serve"
```

---

## Task 8: CLI — serve, create, list, show

**Files:**

- Modify: `cmd/specgraph/main.go`
- Create: `cmd/specgraph/serve.go`
- Create: `cmd/specgraph/spec.go`

**Step 1: Install Cobra**

```bash
go get github.com/spf13/cobra
```

**Step 2: Implement the CLI**

`cmd/specgraph/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "specgraph",
	Short: "Live spec-driven development framework",
}

var cfgFile string

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", ".specgraph/config.yaml", "config file path")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

`cmd/specgraph/serve.go`:

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/seanb4t/specgraph/internal/config"
	"github.com/seanb4t/specgraph/internal/docker"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SpecGraph server",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if cfg.IsRemote() {
		return fmt.Errorf("config has remote server set — no need to run serve")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Docker mode: start compose stack
	if cfg.Server.Mode == "docker" {
		composeFile, err := docker.EnsureComposeFile(".", cfg.Storage.Backend)
		if err != nil {
			return err
		}
		fmt.Println("Starting Docker Compose stack...")
		if err := docker.ComposeUp(composeFile); err != nil {
			return err
		}
		defer docker.ComposeDown(composeFile)
	}

	// Connect to storage
	var backend interface{ Close(context.Context) error }
	switch cfg.Storage.Backend {
	case "memgraph":
		store, err := memgraph.New(ctx, cfg.Storage.Memgraph.BoltURI)
		if err != nil {
			return fmt.Errorf("connect to memgraph: %w", err)
		}
		defer store.Close(ctx)
		backend = store

		mux := server.NewMux(store)
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		srv := &http.Server{Addr: addr, Handler: mux}

		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			fmt.Println("\nShutting down...")
			srv.Close()
		}()

		fmt.Printf("SpecGraph server running at http://%s\n", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
	default:
		return fmt.Errorf("unsupported backend: %s", cfg.Storage.Backend)
	}

	_ = backend
	return nil
}
```

`cmd/specgraph/spec.go`:

```go
package main

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/config"
	"github.com/spf13/cobra"
)

func specClient() (specgraphv1connect.SpecServiceClient, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, err
	}
	baseURL := cfg.Server.Remote
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	}
	return specgraphv1connect.NewSpecServiceClient(http.DefaultClient, baseURL), nil
}

var createCmd = &cobra.Command{
	Use:   "create <slug>",
	Short: "Create a new spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := specClient()
		if err != nil {
			return err
		}
		intent, _ := cmd.Flags().GetString("intent")
		priority, _ := cmd.Flags().GetString("priority")

		resp, err := client.CreateSpec(context.Background(), connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:     args[0],
			Intent:   intent,
			Priority: priority,
		}))
		if err != nil {
			return err
		}
		fmt.Printf("Created: %s (%s)\n", resp.Msg.Slug, resp.Msg.Id)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List specs",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := specClient()
		if err != nil {
			return err
		}
		stage, _ := cmd.Flags().GetString("stage")
		priority, _ := cmd.Flags().GetString("priority")

		resp, err := client.ListSpecs(context.Background(), connect.NewRequest(&specv1.ListSpecsRequest{
			Stage:    stage,
			Priority: priority,
		}))
		if err != nil {
			return err
		}
		if len(resp.Msg.Specs) == 0 {
			fmt.Println("No specs found.")
			return nil
		}
		for _, s := range resp.Msg.Specs {
			fmt.Printf("%-12s %-4s %-10s %s\n", s.Id, s.Priority, s.Stage, s.Slug)
		}
		return nil
	},
}

var showCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: "Show a spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := specClient()
		if err != nil {
			return err
		}
		resp, err := client.GetSpec(context.Background(), connect.NewRequest(&specv1.GetSpecRequest{
			Slug: args[0],
		}))
		if err != nil {
			return err
		}
		s := resp.Msg
		fmt.Printf("ID:         %s\n", s.Id)
		fmt.Printf("Slug:       %s\n", s.Slug)
		fmt.Printf("Intent:     %s\n", s.Intent)
		fmt.Printf("Stage:      %s\n", s.Stage)
		fmt.Printf("Priority:   %s\n", s.Priority)
		fmt.Printf("Complexity: %s\n", s.Complexity)
		fmt.Printf("Version:    %d\n", s.Version)
		return nil
	},
}

func init() {
	createCmd.Flags().StringP("intent", "i", "", "spec intent")
	createCmd.Flags().StringP("priority", "p", "p2", "priority (p0-p3)")
	createCmd.MarkFlagRequired("intent")

	listCmd.Flags().String("stage", "", "filter by stage")
	listCmd.Flags().String("priority", "", "filter by priority")

	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
}
```

**Step 3: Verify it builds**

```bash
go mod tidy
go build -o specgraph ./cmd/specgraph
./specgraph --help
./specgraph create --help
./specgraph serve --help
```

Expected: help output for all commands

**Step 4: Commit**

```bash
git add cmd/ go.mod go.sum
git commit -m "feat: CLI with serve, create, list, show commands"
```

---

## Task 9: Dockerfile

**Files:**

- Create: `Dockerfile`

**Step 1: Write the Dockerfile**

`Dockerfile`:

```dockerfile
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /specgraph ./cmd/specgraph

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /specgraph /usr/local/bin/specgraph
EXPOSE 9090
ENTRYPOINT ["specgraph"]
CMD ["serve", "--config", "/etc/specgraph/config.yaml"]
```

**Step 2: Verify it builds**

```bash
docker build -t specgraph:dev .
```

Expected: successful build

**Step 3: Commit**

```bash
git add Dockerfile
git commit -m "feat: multi-stage Dockerfile for specgraph server"
```

---

## Task 10: End-to-End Smoke Test

**Files:**

- Create: `e2e/smoke_test.sh`

**Step 1: Write the smoke test**

`e2e/smoke_test.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# End-to-end smoke test: starts Memgraph in Docker, runs the server locally,
# creates a spec via CLI, lists it, shows it.

echo "=== SpecGraph E2E Smoke Test ==="

# Start Memgraph
echo "Starting Memgraph..."
docker run -d --name specgraph-e2e-memgraph -p 7688:7687 memgraph/memgraph:latest
sleep 5  # wait for startup

# Write a test config
TMPDIR=$(mktemp -d)
cat > "$TMPDIR/config.yaml" <<'YAML'
server:
  mode: external
  host: "127.0.0.1"
  port: 9091
storage:
  backend: memgraph
  memgraph:
    bolt_uri: "bolt://localhost:7688"
YAML

# Build and start server
echo "Building specgraph..."
go build -o "$TMPDIR/specgraph" ./cmd/specgraph

echo "Starting server..."
"$TMPDIR/specgraph" serve --config "$TMPDIR/config.yaml" &
SERVER_PID=$!
sleep 2

# Write a client config
cat > "$TMPDIR/client.yaml" <<'YAML'
server:
  remote: "http://127.0.0.1:9091"
YAML

export SG="$TMPDIR/specgraph --config=$TMPDIR/client.yaml"

# Test: create
echo "Creating spec..."
$SG create login-api --intent "REST endpoint for OAuth2 login" --priority p1
echo "PASS: create"

# Test: list
echo "Listing specs..."
OUTPUT=$($SG list)
echo "$OUTPUT"
echo "$OUTPUT" | grep -q "login-api" || { echo "FAIL: login-api not in list"; exit 1; }
echo "PASS: list"

# Test: show
echo "Showing spec..."
OUTPUT=$($SG show login-api)
echo "$OUTPUT"
echo "$OUTPUT" | grep -q "REST endpoint for OAuth2 login" || { echo "FAIL: intent not shown"; exit 1; }
echo "PASS: show"

# Cleanup
echo "Cleaning up..."
kill $SERVER_PID 2>/dev/null || true
docker rm -f specgraph-e2e-memgraph 2>/dev/null || true
rm -rf "$TMPDIR"

echo "=== All E2E tests passed ==="
```

**Step 2: Run the smoke test**

```bash
chmod +x e2e/smoke_test.sh
./e2e/smoke_test.sh
```

Expected: All tests pass — create, list, show work end-to-end.

**Step 3: Commit**

```bash
git add e2e/
git commit -m "test: end-to-end smoke test for vertical slice"
```

---

## Summary

After completing all 10 tasks, you have a working SpecGraph with:

| What | Status |
|------|--------|
| Protobuf schema (Spec messages + SpecService) | Done |
| ConnectRPC server | Done |
| Memgraph storage backend with integration tests | Done |
| CLI (serve, create, list, show) | Done |
| Docker compose for Memgraph | Done |
| Config (docker, external, remote modes) | Done |
| Dockerfile for server image | Done |
| E2E smoke test | Done |

**Built in subsequent slices:**

- `specgraph init` (interactive setup) — Slice 2
- ConstitutionService (layered ground truth) — Slice 2
- DecisionService (ADR-style decisions as graph nodes) — Extended Services (#4)
- ClaimService (lease-based spec ownership) — Extended Services (#4)
- GraphService (deps, transitive deps, impact, ready, critical path) — Extended Services (#4)
- AuthoringService (Spark → Approve funnel) — Slice 3
- HealthService — Extended Services (#4)
- Comprehensive E2E test system (Ginkgo/Gomega, testcontainers) — PR #19

**Not yet built (future slices):**

- Postgres+AGE backend
- Sync adapters (Beads, GitHub, tool injection)
- Execution bundles + bootstrap + prime endpoint
- ExecutionService
