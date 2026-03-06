# E2E Test System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Ginkgo+Gomega E2E test system that covers the full specgraph CLI surface using testcontainers and Docker.

**Architecture:** Two test suites — `e2e/api/` for API-level tests (testcontainers Memgraph + in-process server) and `e2e/docker/` for Docker mode lifecycle tests. Shared helpers in `e2e/testutil/`. Build-tagged with `//go:build e2e`.

**Tech Stack:** Go, Ginkgo v2, Gomega, testcontainers-go, ConnectRPC

---

### Task 1: Add Ginkgo/Gomega Dependencies and Scaffold

**Files:**
- Modify: `go.mod`
- Create: `e2e/testutil/containers.go`
- Create: `e2e/testutil/server.go`
- Create: `e2e/testutil/cli.go`

**Step 1: Install ginkgo CLI and add deps**

Run:
```bash
go install github.com/onsi/ginkgo/v2/ginkgo@latest
go get github.com/onsi/ginkgo/v2
go get github.com/onsi/gomega
go mod tidy
```

**Step 2: Create `e2e/testutil/containers.go`**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartMemgraph launches a Memgraph container and returns the bolt URI.
// The returned cleanup function terminates the container.
func StartMemgraph(ctx context.Context) (string, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "memgraph/memgraph:latest",
		ExposedPorts: []string{"7687/tcp"},
		WaitingFor:   wait.ForListeningPort("7687/tcp").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("start memgraph container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, fmt.Errorf("get container host: %w", err)
	}
	port, err := container.MappedPort(ctx, "7687")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, fmt.Errorf("get mapped port: %w", err)
	}

	boltURI := fmt.Sprintf("bolt://%s:%s", host, port.Port())
	cleanup := func() { _ = container.Terminate(ctx) }
	return boltURI, cleanup, nil
}
```

**Step 3: Create `e2e/testutil/server.go`**

This starts the real specgraph HTTP server in-process on a random port.

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package testutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
)

// ServerInfo holds the running server's details.
type ServerInfo struct {
	BaseURL string
	Store   *memgraph.Store
}

// StartServer launches a specgraph HTTP server connected to the given Memgraph instance.
// Returns the base URL and a cleanup function that shuts down the server.
func StartServer(ctx context.Context, boltURI string) (*ServerInfo, func(), error) {
	store, err := memgraph.New(ctx, boltURI)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to memgraph: %w", err)
	}

	mux := server.NewMux(store)
	server.RegisterHealthService(mux)
	server.RegisterDecisionService(mux, store)
	server.RegisterGraphService(mux, store)
	server.RegisterClaimService(mux, store)
	server.RegisterConstitutionService(mux, store)
	server.RegisterAuthoringService(mux, store, store)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = store.Close(ctx)
		return nil, nil, fmt.Errorf("listen: %w", err)
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(listener) }()

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())
	cleanup := func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		_ = store.Close(ctx)
	}
	return &ServerInfo{BaseURL: baseURL, Store: store}, cleanup, nil
}
```

**Step 4: Create `e2e/testutil/cli.go`**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package testutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
)

// CLIRunner runs the specgraph binary with a given config.
type CLIRunner struct {
	BinaryPath string
	ConfigPath string
}

// CLIResult holds the output of a CLI command.
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// NewCLI builds the specgraph binary into a temp dir and returns a runner
// configured with the given config file path.
func NewCLI(configPath string) (*CLIRunner, error) {
	tmpDir, err := os.MkdirTemp("", "specgraph-e2e-*")
	if err != nil {
		return nil, err
	}
	binaryPath := filepath.Join(tmpDir, "specgraph")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/specgraph")
	cmd.Dir = findProjectRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, &BuildError{Output: string(out), Err: err}
	}
	return &CLIRunner{BinaryPath: binaryPath, ConfigPath: configPath}, nil
}

// Run executes the specgraph CLI with the given args.
func (c *CLIRunner) Run(args ...string) CLIResult {
	fullArgs := append([]string{"--config", c.ConfigPath}, args...)
	cmd := exec.Command(c.BinaryPath, fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = -1
	}
	return CLIResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: exitCode}
}

// BuildError wraps a failed go build.
type BuildError struct {
	Output string
	Err    error
}

func (e *BuildError) Error() string {
	return "go build failed: " + e.Output
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
```

**Step 5: Commit**

```bash
git add e2e/testutil/ go.mod go.sum
git commit -m "test(e2e): add ginkgo/gomega deps and test helpers"
```

---

### Task 2: API Suite Bootstrap and Health Test

**Files:**
- Create: `e2e/api/api_suite_test.go`
- Create: `e2e/api/health_test.go`

**Step 1: Create `e2e/api/api_suite_test.go`**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/seanb4t/specgraph/e2e/testutil"
)

var (
	serverInfo     *testutil.ServerInfo
	cleanupServer  func()
	cleanupMG      func()
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API E2E Suite")
}

var _ = BeforeSuite(func() {
	ctx := context.Background()

	var err error
	var boltURI string
	boltURI, cleanupMG, err = testutil.StartMemgraph(ctx)
	Expect(err).NotTo(HaveOccurred())

	serverInfo, cleanupServer, err = testutil.StartServer(ctx, boltURI)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if cleanupServer != nil {
		cleanupServer()
	}
	if cleanupMG != nil {
		cleanupMG()
	}
})
```

**Step 2: Create `e2e/api/health_test.go`**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"net/http"
)

var _ = Describe("health", func() {
	It("returns healthy when server is running", func() {
		client := specgraphv1connect.NewHealthServiceClient(http.DefaultClient, serverInfo.BaseURL)
		resp, err := client.Check(context.Background(), connect.NewRequest(&specv1.HealthCheckRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Status).To(Equal("ok"))
	})
})
```

**Step 3: Run test to verify it passes**

Run: `go test -tags e2e -v -timeout 5m ./e2e/api/...`
Expected: PASS — health check returns "ok"

**Step 4: Commit**

```bash
git add e2e/api/
git commit -m "test(e2e): add API suite bootstrap and health check test"
```

---

### Task 3: Spec Lifecycle Tests

**Files:**
- Create: `e2e/api/spec_test.go`

**Step 1: Create `e2e/api/spec_test.go`**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("spec lifecycle", Ordered, func() {
	var client specgraphv1connect.SpecServiceClient

	BeforeAll(func() {
		client = specgraphv1connect.NewSpecServiceClient(http.DefaultClient, serverInfo.BaseURL)
	})

	It("creates a spec with slug, intent, and priority", func() {
		resp, err := client.CreateSpec(context.Background(), connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:     "login-api",
			Intent:   "Implement OAuth2 login endpoint",
			Priority: "p1",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Slug).To(Equal("login-api"))
		Expect(resp.Msg.Intent).To(Equal("Implement OAuth2 login endpoint"))
		Expect(resp.Msg.Priority).To(Equal("p1"))
		Expect(resp.Msg.Stage).To(Equal("spark"))
		Expect(resp.Msg.Id).To(HavePrefix("spec-"))
	})

	It("lists specs and includes the created spec", func() {
		resp, err := client.ListSpecs(context.Background(), connect.NewRequest(&specv1.ListSpecsRequest{}))
		Expect(err).NotTo(HaveOccurred())
		slugs := make([]string, len(resp.Msg.Specs))
		for i, s := range resp.Msg.Specs {
			slugs[i] = s.Slug
		}
		Expect(slugs).To(ContainElement("login-api"))
	})

	It("shows spec details by slug", func() {
		resp, err := client.GetSpec(context.Background(), connect.NewRequest(&specv1.GetSpecRequest{
			Slug: "login-api",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Slug).To(Equal("login-api"))
		Expect(resp.Msg.Intent).To(Equal("Implement OAuth2 login endpoint"))
	})

	It("updates spec fields", func() {
		newIntent := "Implement OAuth2 + OIDC login endpoint"
		newPriority := "p0"
		resp, err := client.UpdateSpec(context.Background(), connect.NewRequest(&specv1.UpdateSpecRequest{
			Slug:     "login-api",
			Intent:   &newIntent,
			Priority: &newPriority,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Intent).To(Equal("Implement OAuth2 + OIDC login endpoint"))
		Expect(resp.Msg.Priority).To(Equal("p0"))
		Expect(resp.Msg.Version).To(BeNumerically(">=", 2))
	})
})
```

**Step 2: Run test**

Run: `go test -tags e2e -v -timeout 5m ./e2e/api/... -run "spec lifecycle"`
Expected: PASS

**Step 3: Commit**

```bash
git add e2e/api/spec_test.go
git commit -m "test(e2e): add spec lifecycle tests"
```

---

### Task 4: Decision Lifecycle Tests

**Files:**
- Create: `e2e/api/decision_test.go`

Tests: create a decision, list decisions, show decision by slug. Same pattern as spec tests but using `DecisionServiceClient`.

**Commit:** `test(e2e): add decision lifecycle tests`

---

### Task 5: Constitution Tests

**Files:**
- Create: `e2e/api/constitution_test.go`

Tests: show constitution (requires bootstrapping one via store in BeforeAll or via YAML file), check a spec against constitution, emit constitution.

**Commit:** `test(e2e): add constitution tests`

---

### Task 6: Claim Protocol Tests

**Files:**
- Create: `e2e/api/claim_test.go`

Tests: create a spec, advance it to approved stage, claim it, verify double-claim fails, unclaim it.

**Commit:** `test(e2e): add claim protocol tests`

---

### Task 7: Graph Edge and Query Tests

**Files:**
- Create: `e2e/api/edge_test.go`
- Create: `e2e/api/graph_test.go`

Edge tests: add edges between specs, list edges, remove edge.
Graph query tests: deps traversal, ready command, critical-path, impact analysis.

**Commit:** `test(e2e): add graph edge and query tests`

---

### Task 8: Authoring Funnel Tests

**Files:**
- Create: `e2e/api/authoring_test.go`

Tests the full funnel: spark → shape → specify → decompose → approve. Uses the AuthoringServiceClient. Each stage advances the spec and validates the stage transition.

**Commit:** `test(e2e): add authoring funnel tests`

---

### Task 9: Error Handling Tests

**Files:**
- Create: `e2e/api/errors_test.go`

Tests: missing required args (via CLI runner), nonexistent slug returns error, claim on unapproved spec fails, edge cycle detection.

**Commit:** `test(e2e): add error handling tests`

---

### Task 10: Init Command Tests

**Files:**
- Create: `e2e/api/init_test.go`

Tests: `init --yes` creates config, `init --scan` generates constitution draft, running init twice rejects. Uses CLIRunner since init is a local command, not an API call.

**Commit:** `test(e2e): add init command tests`

---

### Task 11: Docker Mode Suite

**Files:**
- Create: `e2e/docker/docker_suite_test.go`
- Create: `e2e/docker/serve_test.go`

Bootstrap: builds specgraph binary in BeforeSuite. Tests: creates temp dir, writes config with `mode: docker`, runs `specgraph serve` as a subprocess, verifies compose file written to `.specgraph/`, health check passes, sends SIGTERM to verify clean shutdown and compose teardown.

**Commit:** `test(e2e): add Docker mode lifecycle tests`

---

### Task 12: Taskfile and Cleanup

**Files:**
- Modify: `Taskfile.yml` — add `test:e2e`, `test:e2e:api`, `test:e2e:docker` targets
- Delete: `e2e/smoke_test.sh` — replaced by Go tests
- Modify: `Taskfile.yml` — update existing `test:e2e` to use new Go tests

**Step 1: Update Taskfile**

Replace the existing `test:e2e` task and add the new targets.

**Step 2: Delete old smoke test**

```bash
rm e2e/smoke_test.sh
```

**Step 3: Run full E2E suite**

Run: `task test:e2e`
Expected: All tests pass.

**Step 4: Commit**

```bash
git add Taskfile.yml
git rm e2e/smoke_test.sh
git commit -m "test(e2e): add Taskfile targets and remove old smoke test"
```

---

### Task 13: Verify and License Headers

**Step 1: Add license headers to any missing files**

Run: `task license:add`

**Step 2: Run linter**

Run: `task lint`

**Step 3: Run full test suite (unit + E2E)**

Run: `task test && task test:e2e`

**Step 4: Final commit if any fixups needed**

```bash
git commit -m "chore: license headers and lint fixes for E2E tests"
```
