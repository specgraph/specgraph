# Slice 7: Global Daemon & Claude Code Plugin — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform SpecGraph from a per-project tool into a global daemon with multi-project graph namespacing, then ship a Claude Code plugin with 11 skills and a SessionStart hook.

**Architecture:** The server becomes a global daemon managed as a user-level OS service (launchd/systemd). Config splits into global (`~/.config/specgraph/config.yaml`) with `server:` and `client:` sections, plus per-repo `.specgraph.yaml` for project identity. Graph isolation uses `[:BELONGS_TO]` edges from all domain nodes to a `(:Project)` node. The Claude Code plugin is a directory of SKILL.md files that shell out to CLI commands.

**Tech Stack:** Go, ConnectRPC (buf/connect-go), Memgraph (neo4j-go-driver v5), Cobra (CLI), Claude Code plugin SDK (SKILL.md, plugin.json, shell hooks)

**Design Doc:** `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md`

---

## Chunk 1: Config Rewrite & XDG Layout

### Task 1: XDG Path Resolution Package

**Files:**

- Create: `internal/xdg/xdg.go`
- Create: `internal/xdg/xdg_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/xdg/xdg_test.go
package xdg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigHome_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "specgraph"), xdg.ConfigHome())
}

func TestConfigHome_Override(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test-config")
	assert.Equal(t, "/tmp/xdg-test-config/specgraph", xdg.ConfigHome())
}

func TestDataHome_Default(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".local", "share", "specgraph"), xdg.DataHome())
}

func TestStateHome_Default(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".local", "state", "specgraph"), xdg.StateHome())
}

func TestConfigFilePath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	assert.Equal(t, "/tmp/xdg-test/specgraph/config.yaml", xdg.ConfigFile())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/xdg/ -v -count=1`
Expected: FAIL — package does not exist

- [ ] **Step 3: Write minimal implementation**

```go
// internal/xdg/xdg.go
package xdg

import (
	"os"
	"path/filepath"
)

const appName = "specgraph"

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

// ConfigHome returns XDG_CONFIG_HOME/specgraph or ~/.config/specgraph.
func ConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, appName)
	}
	return filepath.Join(homeDir(), ".config", appName)
}

// DataHome returns XDG_DATA_HOME/specgraph or ~/.local/share/specgraph.
func DataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, appName)
	}
	return filepath.Join(homeDir(), ".local", "share", appName)
}

// StateHome returns XDG_STATE_HOME/specgraph or ~/.local/state/specgraph.
func StateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return filepath.Join(v, appName)
	}
	return filepath.Join(homeDir(), ".local", "state", appName)
}

// ConfigFile returns the path to the global config file.
func ConfigFile() string {
	return filepath.Join(ConfigHome(), "config.yaml")
}

// EnsureDirs creates all XDG directories if they don't exist.
func EnsureDirs() error {
	for _, dir := range []string{ConfigHome(), DataHome(), StateHome()} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/xdg/ -v -count=1`
Expected: PASS

- [ ] **Step 5: Add license header and commit**

```bash
task license:add
git add internal/xdg/
git commit -m "feat(xdg): XDG base directory resolution package"
```

---

### Task 2: New Global Config Schema

**Files:**

- Create: `internal/config/global.go`
- Create: `internal/config/global_test.go`

This replaces the current per-project config with the new `server:` / `client:` split schema. The old `config.go` will be deprecated in a later task.

- [ ] **Step 1: Write the failing test**

```go
// internal/config/global_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadGlobal_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// File doesn't exist — should write defaults and return them.
	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:7890", cfg.Server.Listen)
	assert.Equal(t, "service", cfg.Server.Mode)
	assert.Equal(t, "memgraph", cfg.Server.Backend)
	assert.Equal(t, "bolt://localhost:7687", cfg.Server.Memgraph.BoltURI)
	assert.True(t, cfg.Server.Docker)
	assert.Equal(t, "http://localhost:7890", cfg.Client.DefaultServer)
	assert.Empty(t, cfg.Client.Routes)

	// File should now exist on disk.
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestLoadGlobal_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `
server:
  listen: "0.0.0.0:9999"
  mode: manual
  backend: memgraph
  memgraph:
    bolt_uri: "bolt://db:7687"
  docker: false
client:
  default_server: "http://remote:9999"
  routes:
    - project: "org-*"
      server: "https://shared:7890"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o644))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:9999", cfg.Server.Listen)
	assert.Equal(t, "manual", cfg.Server.Mode)
	assert.False(t, cfg.Server.Docker)
	assert.Equal(t, "http://remote:9999", cfg.Client.DefaultServer)
	require.Len(t, cfg.Client.Routes, 1)
	assert.Equal(t, "org-*", cfg.Client.Routes[0].Project)
	assert.Equal(t, "https://shared:7890", cfg.Client.Routes[0].Server)
}

func TestResolveServer_RepoOverride(t *testing.T) {
	cfg := &config.GlobalConfig{
		Client: config.ClientConfig{
			DefaultServer: "http://localhost:7890",
		},
	}
	url := cfg.ResolveServer("myproject", "https://team-server:7890")
	assert.Equal(t, "https://team-server:7890", url)
}

func TestResolveServer_RouteMatch(t *testing.T) {
	cfg := &config.GlobalConfig{
		Client: config.ClientConfig{
			DefaultServer: "http://localhost:7890",
			Routes: []config.Route{
				{Project: "org-*", Server: "https://shared:7890"},
			},
		},
	}
	assert.Equal(t, "https://shared:7890", cfg.ResolveServer("org-frontend", ""))
	assert.Equal(t, "http://localhost:7890", cfg.ResolveServer("my-project", ""))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadGlobal -v -count=1`
Expected: FAIL — types and functions don't exist

- [ ] **Step 3: Write minimal implementation**

```go
// internal/config/global.go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfig is the new top-level config at ~/.config/specgraph/config.yaml.
type GlobalConfig struct {
	Server ServerSection `yaml:"server"`
	Client ClientConfig  `yaml:"client"`
}

// ServerSection configures the specgraph server daemon.
type ServerSection struct {
	Listen   string         `yaml:"listen"`
	Mode     string         `yaml:"mode"`     // "service" | "manual"
	Backend  string         `yaml:"backend"`  // "memgraph"
	Memgraph MemgraphConfig `yaml:"memgraph"`
	Docker   bool           `yaml:"docker"`
}

// ClientConfig configures how CLI commands connect to the server.
type ClientConfig struct {
	DefaultServer string  `yaml:"default_server"`
	Routes        []Route `yaml:"routes,omitempty"`
}

// Route maps a project slug glob to a server URL.
type Route struct {
	Project string `yaml:"project"` // glob pattern (filepath.Match semantics)
	Server  string `yaml:"server"`
}

// LoadGlobal loads the global config from path. If the file doesn't exist,
// writes defaults and returns them.
func LoadGlobal(path string) (*GlobalConfig, error) {
	cfg := globalDefaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// File doesn't exist — write defaults.
		if writeErr := writeGlobal(path, cfg); writeErr != nil {
			return nil, fmt.Errorf("write default config: %w", writeErr)
		}
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// ResolveServer determines the server URL for a given project slug.
// Priority: repoOverride > route match > default.
func (c *GlobalConfig) ResolveServer(projectSlug, repoOverride string) string {
	if repoOverride != "" {
		return repoOverride
	}
	for _, r := range c.Client.Routes {
		if matched, _ := filepath.Match(r.Project, projectSlug); matched {
			return r.Server
		}
	}
	return c.Client.DefaultServer
}

func globalDefaults() *GlobalConfig {
	return &GlobalConfig{
		Server: ServerSection{
			Listen:  "0.0.0.0:7890",
			Mode:    "service",
			Backend: "memgraph",
			Memgraph: MemgraphConfig{
				BoltURI: "bolt://localhost:7687",
			},
			Docker: true,
		},
		Client: ClientConfig{
			DefaultServer: "http://localhost:7890",
		},
	}
}

func writeGlobal(path string, cfg *GlobalConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run "TestLoadGlobal|TestResolveServer" -v -count=1`
Expected: PASS

- [ ] **Step 5: Add license header and commit**

```bash
task license:add
git add internal/config/global.go internal/config/global_test.go
git commit -m "feat(config): global config schema with server/client split and route resolution"
```

---

### Task 3: Per-Repo `.specgraph.yaml` Reader

**Files:**

- Create: `internal/config/project.go`
- Create: `internal/config/project_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/config/project_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProject_ExplicitSlug(t *testing.T) {
	dir := t.TempDir()
	yaml := "project: my-cool-project\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o644))

	p, err := config.LoadProject(dir)
	require.NoError(t, err)
	assert.Equal(t, "my-cool-project", p.Slug)
	assert.Empty(t, p.Server)
}

func TestLoadProject_WithServerOverride(t *testing.T) {
	dir := t.TempDir()
	yaml := "project: foo\nserver: https://remote:7890\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o644))

	p, err := config.LoadProject(dir)
	require.NoError(t, err)
	assert.Equal(t, "foo", p.Slug)
	assert.Equal(t, "https://remote:7890", p.Server)
}

func TestLoadProject_NoFile_DeriveFromDir(t *testing.T) {
	dir := t.TempDir()
	// No .specgraph.yaml — should derive slug from directory name.
	p, err := config.LoadProject(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Base(dir), p.Slug)
}

func TestFindProjectRoot_WalksUp(t *testing.T) {
	// Create nested dirs with .specgraph.yaml at root.
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".specgraph.yaml"), []byte("project: root-proj\n"), 0o644))
	child := filepath.Join(root, "src", "pkg")
	require.NoError(t, os.MkdirAll(child, 0o755))

	found, err := config.FindProjectRoot(child)
	require.NoError(t, err)
	assert.Equal(t, root, found)
}

func TestNormalizeSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"owner/repo", "owner-repo"},
		{"git@github.com:owner/repo.git", "owner-repo"},
		{"https://github.com/owner/repo.git", "owner-repo"},
		{"simple", "simple"},
		{"UPPER-Case", "upper-case"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, config.NormalizeSlug(tt.input))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run "TestLoadProject|TestFindProjectRoot|TestNormalizeSlug" -v -count=1`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// internal/config/project.go
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProjectConfig is the per-repo .specgraph.yaml.
type ProjectConfig struct {
	Slug   string `yaml:"project,omitempty"`
	Server string `yaml:"server,omitempty"`
}

const projectFileName = ".specgraph.yaml"

// LoadProject loads project config from dir, walking up to find
// .specgraph.yaml. If no file found, derives slug from git remote or dir name.
func LoadProject(dir string) (*ProjectConfig, error) {
	root, err := FindProjectRoot(dir)
	if err != nil {
		// No .specgraph.yaml found — derive slug.
		slug := deriveSlug(dir)
		return &ProjectConfig{Slug: slug}, nil
	}

	data, err := os.ReadFile(filepath.Join(root, projectFileName))
	if err != nil {
		return nil, fmt.Errorf("read project config: %w", err)
	}

	var pc ProjectConfig
	if err := yaml.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}

	if pc.Slug == "" {
		pc.Slug = deriveSlug(root)
	}
	return &pc, nil
}

// FindProjectRoot walks from dir upward looking for .specgraph.yaml.
// Returns the directory containing it, or error if not found.
func FindProjectRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, projectFileName)); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no %s found", projectFileName)
		}
		abs = parent
	}
}

// WriteProject writes a .specgraph.yaml to dir.
func WriteProject(dir string, pc *ProjectConfig) error {
	data, err := yaml.Marshal(pc)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, projectFileName), data, 0o644)
}

// NormalizeSlug converts a remote URL or owner/repo into a kebab-case slug.
func NormalizeSlug(raw string) string {
	// Strip common git remote prefixes.
	s := raw
	s = strings.TrimSuffix(s, ".git")
	if idx := strings.LastIndex(s, ":"); idx != -1 && strings.Contains(s, "@") {
		// git@host:owner/repo
		s = s[idx+1:]
	} else if strings.Contains(s, "://") {
		// https://host/owner/repo
		parts := strings.SplitN(s, "://", 2)
		if len(parts) == 2 {
			s = parts[1]
		}
		// Remove host.
		if idx := strings.Index(s, "/"); idx != -1 {
			s = s[idx+1:]
		}
	}
	s = strings.ReplaceAll(s, "/", "-")
	return strings.ToLower(s)
}

func deriveSlug(dir string) string {
	// Try git remote first.
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err == nil {
		remote := strings.TrimSpace(string(out))
		if remote != "" {
			return NormalizeSlug(remote)
		}
	}
	return strings.ToLower(filepath.Base(dir))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run "TestLoadProject|TestFindProjectRoot|TestNormalizeSlug" -v -count=1`
Expected: PASS

- [ ] **Step 5: Add license header and commit**

```bash
task license:add
git add internal/config/project.go internal/config/project_test.go
git commit -m "feat(config): per-repo .specgraph.yaml reader with slug auto-derivation"
```

---

## Chunk 2: Project Node & Graph Namespacing

### Task 4: Project Storage Interface & Domain Type

**Files:**

- Create: `internal/storage/project.go`

- [ ] **Step 1: Define the Project domain type and backend interface**

```go
// internal/storage/project.go
package storage

import (
	"context"
	"errors"
	"time"
)

// ErrProjectNotFound is returned when no project exists with the given slug.
var ErrProjectNotFound = errors.New("project not found")

// Project represents a registered project in the spec graph.
type Project struct {
	Slug         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SyncAdapters []string // e.g. ["beads", "github"]
	GitHubRepo   string   // owner/repo for GitHub adapter (optional)
}

// ProjectBackend defines storage operations for project management.
type ProjectBackend interface {
	// GetProject returns a project by slug.
	GetProject(ctx context.Context, slug string) (*Project, error)

	// EnsureProject creates a project if it doesn't exist, or returns the existing one.
	EnsureProject(ctx context.Context, slug string) (*Project, error)

	// UpdateProject updates mutable project fields.
	UpdateProject(ctx context.Context, slug string, adapters []string, ghRepo string) (*Project, error)

	// ListProjects returns all registered projects.
	ListProjects(ctx context.Context) ([]*Project, error)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/storage/`
Expected: Success

- [ ] **Step 3: Add license header and commit**

```bash
task license:add
git add internal/storage/project.go
git commit -m "feat(storage): Project domain type and ProjectBackend interface"
```

---

### Task 5: Memgraph Project Storage Implementation

**Files:**

- Create: `internal/storage/memgraph/project.go`
- Create: `internal/storage/memgraph/project_test.go`

- [ ] **Step 1: Write the failing integration test**

```go
// internal/storage/memgraph/project_test.go
//go:build integration

package memgraph_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureProject_CreatesNew(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI, memgraph.WithProject("test-ensure"))
	require.NoError(t, err)
	defer store.Close(ctx)

	p, err := store.EnsureProject(ctx, "test-project")
	require.NoError(t, err)
	assert.Equal(t, "test-project", p.Slug)
	assert.False(t, p.CreatedAt.IsZero())
}

func TestEnsureProject_Idempotent(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI, memgraph.WithProject("test-idem"))
	require.NoError(t, err)
	defer store.Close(ctx)

	p1, err := store.EnsureProject(ctx, "idem-project")
	require.NoError(t, err)

	p2, err := store.EnsureProject(ctx, "idem-project")
	require.NoError(t, err)

	assert.Equal(t, p1.CreatedAt, p2.CreatedAt)
}

func TestGetProject_NotFound(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI, memgraph.WithProject("test-notfound"))
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err := store.GetProject(ctx, "nonexistent")
	assert.ErrorIs(t, err, storage.ErrProjectNotFound)
}

func TestUpdateProject_SyncAdapters(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI, memgraph.WithProject("test-update"))
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err := store.EnsureProject(ctx, "updatable")
	require.NoError(t, err)

	p, err := store.UpdateProject(ctx, "updatable", []string{"beads", "github"}, "owner/repo")
	require.NoError(t, err)
	assert.Equal(t, []string{"beads", "github"}, p.SyncAdapters)
	assert.Equal(t, "owner/repo", p.GitHubRepo)
}

func TestListProjects(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI, memgraph.WithProject("test-list"))
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err := store.EnsureProject(ctx, "proj-a")
	require.NoError(t, err)
	_, err = store.EnsureProject(ctx, "proj-b")
	require.NoError(t, err)

	projects, err := store.ListProjects(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(projects), 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/memgraph/ -run TestEnsureProject -tags integration -v -count=1 -timeout=120s`
Expected: FAIL — method not found

- [ ] **Step 3: Write the implementation**

```go
// internal/storage/memgraph/project.go
package memgraph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
)

func (s *Store) EnsureProject(ctx context.Context, slug string) (*storage.Project, error) {
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MERGE (p:Project {slug: $slug})
		 ON CREATE SET p.created_at = $now, p.updated_at = $now,
		               p.sync_adapters = [], p.github_repo = ""
		 RETURN p.slug, p.created_at, p.updated_at, p.sync_adapters, p.github_repo`,
		map[string]any{
			"slug": slug,
			"now":  s.now(),
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("ensure project: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("ensure project: no record returned")
	}
	return recordToProject(result.Records[0])
}

func (s *Store) GetProject(ctx context.Context, slug string) (*storage.Project, error) {
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (p:Project {slug: $slug}) RETURN p.slug, p.created_at, p.updated_at, p.sync_adapters, p.github_repo`,
		map[string]any{"slug": slug},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, storage.ErrProjectNotFound
	}
	return recordToProject(result.Records[0])
}

func (s *Store) UpdateProject(ctx context.Context, slug string, adapters []string, ghRepo string) (*storage.Project, error) {
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (p:Project {slug: $slug})
		 SET p.sync_adapters = $adapters, p.github_repo = $ghRepo, p.updated_at = $now
		 RETURN p.slug, p.created_at, p.updated_at, p.sync_adapters, p.github_repo`,
		map[string]any{
			"slug":     slug,
			"adapters": adapters,
			"ghRepo":   ghRepo,
			"now":      s.now(),
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, storage.ErrProjectNotFound
	}
	return recordToProject(result.Records[0])
}

func (s *Store) ListProjects(ctx context.Context) ([]*storage.Project, error) {
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (p:Project) RETURN p.slug, p.created_at, p.updated_at, p.sync_adapters, p.github_repo ORDER BY p.slug`,
		nil,
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	projects := make([]*storage.Project, 0, len(result.Records))
	for _, rec := range result.Records {
		p, err := recordToProject(rec)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func recordToProject(rec *neo4j.Record) (*storage.Project, error) {
	slug, err := recordString(rec, 0, "slug")
	if err != nil {
		return nil, fmt.Errorf("project slug: %w", err)
	}
	createdStr, err := recordString(rec, 1, "created_at")
	if err != nil {
		return nil, fmt.Errorf("project created_at: %w", err)
	}
	updatedStr, err := recordString(rec, 2, "updated_at")
	if err != nil {
		return nil, fmt.Errorf("project updated_at: %w", err)
	}
	created, err := parseRFC3339("created_at", createdStr)
	if err != nil {
		return nil, fmt.Errorf("project created_at parse: %w", err)
	}
	updated, err := parseRFC3339("updated_at", updatedStr)
	if err != nil {
		return nil, fmt.Errorf("project updated_at parse: %w", err)
	}

	// Memgraph returns lists as []any — convert to []string.
	var adapters []string
	if raw, ok := rec.Values[3].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				adapters = append(adapters, s)
			}
		}
	}

	ghRepo, _ := rec.Values[4].(string)

	return &storage.Project{
		Slug:         slug,
		CreatedAt:    created,
		UpdatedAt:    updated,
		SyncAdapters: adapters,
		GitHubRepo:   ghRepo,
	}, nil
}
```

- [ ] **Step 4: Run integration tests**

Run: `go test ./internal/storage/memgraph/ -run "TestEnsureProject|TestGetProject|TestUpdateProject|TestListProjects" -tags integration -v -count=1 -timeout=120s`
Expected: PASS

- [ ] **Step 5: Add license header and commit**

```bash
task license:add
git add internal/storage/memgraph/project.go internal/storage/memgraph/project_test.go
git commit -m "feat(memgraph): Project node CRUD with MERGE-based idempotent creation"
```

---

### Task 6: Add WithProject Option to Memgraph Store

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go` (Store struct, New constructor, Option)

This task adds the `project` field to the Store and the `WithProject` option. It does **not** yet modify queries — that's Task 7.

- [ ] **Step 1: Write the failing test**

```go
// Add to internal/storage/memgraph/memgraph_test.go (or project_test.go)
func TestNew_RequiresProject(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	// Store without project slug should error.
	_, err := newStore(ctx, boltURI)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project slug required")
}

func TestNew_WithProject(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI, memgraph.WithProject("test-proj"))
	require.NoError(t, err)
	defer store.Close(ctx)
	assert.NotNil(t, store)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/memgraph/ -run "TestNew_RequiresProject|TestNew_WithProject" -tags integration -v -count=1 -timeout=120s`
Expected: FAIL

- [ ] **Step 3: Modify the Store struct and constructor**

Add to `internal/storage/memgraph/memgraph.go`:

```go
// In the Store struct, add:
project string // project slug for graph namespacing

// Add the WithProject option:
func WithProject(slug string) Option {
	return func(s *Store) { s.project = slug }
}

// In New(), after applying options, add validation:
if s.project == "" {
	return nil, fmt.Errorf("project slug required: use memgraph.WithProject(slug)")
}

// Also: after verifying connectivity, ensure the Project node exists:
_, err = neo4j.ExecuteQuery(ctx, s.driver,
	`MERGE (p:Project {slug: $slug})
	 ON CREATE SET p.created_at = $now, p.updated_at = $now,
	               p.sync_adapters = [], p.github_repo = ""`,
	map[string]any{"slug": s.project, "now": s.now()},
	neo4j.EagerResultTransformer,
)
if err != nil {
	return nil, fmt.Errorf("ensure project node: %w", err)
}
```

- [ ] **Step 4: Fix all existing test helpers to pass WithProject**

Every test that calls `newStore(t)` or `memgraph.New()` needs `memgraph.WithProject("test")`. Update the test helper function used across all test files.

Run: `go test ./internal/storage/memgraph/ -tags integration -v -count=1 -timeout=120s`
Expected: PASS (all existing tests still pass with the project parameter)

- [ ] **Step 5: Add license header and commit**

```bash
task license:add
git add internal/storage/memgraph/memgraph.go
git commit -m "feat(memgraph): add WithProject option and require project slug on Store"
```

---

### Task 7: Add BELONGS_TO Edges to All Cypher Queries

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go` (spec CRUD queries)
- Modify: `internal/storage/memgraph/graph.go` (edge queries)
- Modify: `internal/storage/memgraph/decision.go` (decision queries)
- Modify: `internal/storage/memgraph/claim.go` (claim queries)
- Modify: `internal/storage/memgraph/constitution.go` (constitution queries)
- Modify: `internal/storage/memgraph/authoring.go` (authoring queries)
- Modify: `internal/storage/memgraph/execution.go` (execution queries)
- Modify: `internal/storage/memgraph/lifecycle.go` (lifecycle queries)
- Modify: `internal/storage/memgraph/sync.go` (sync queries)

This is the largest single task. Every Cypher query that creates or reads nodes must be updated to scope through the Project node via `[:BELONGS_TO]` edges.

**Pattern for CREATE queries:**

```cypher
-- Before:
CREATE (s:Spec {slug: $slug, ...})

-- After:
MATCH (p:Project {slug: $project})
CREATE (p)<-[:BELONGS_TO]-(s:Spec {slug: $slug, ...})
```

**Pattern for READ queries:**

```cypher
-- Before:
MATCH (s:Spec {slug: $slug})

-- After:
MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
```

**Pattern for edge queries between nodes:**

```cypher
-- Before:
MATCH (a:Spec {slug: $from}), (b:Spec {slug: $to})
CREATE (a)-[:DEPENDS_ON]->(b)

-- After:
MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a:Spec {slug: $from}),
      (p)<-[:BELONGS_TO]-(b:Spec {slug: $to})
CREATE (a)-[:DEPENDS_ON]->(b)
```

- [ ] **Step 1: Add a helper method to Store for the project match prefix**

```go
// In memgraph.go, add a helper that all query methods can use:
func (s *Store) projectParam() map[string]any {
	return map[string]any{"project": s.project}
}

// Merge params helper:
func mergeParams(base, extra map[string]any) map[string]any {
	m := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		m[k] = v
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}
```

- [ ] **Step 2: Update CreateSpec and GetSpec in memgraph.go**

Update `CreateSpec` to create the `BELONGS_TO` edge. Update `GetSpec`, `ListSpecs`, `BatchGetSpecs`, `UpdateSpec` to match through Project node. Add `$project` to all param maps.

- [ ] **Step 3: Update graph.go (edge operations)**

Update `AddEdge`, `RemoveEdge`, `ListEdges`, `GetDependencies`, `GetTransitiveDeps`, `GetImpact`, `GetReady`, `GetCriticalPath` to scope through Project node.

- [ ] **Step 4: Update decision.go**

Update `CreateDecision`, `GetDecision`, `ListDecisions`, `UpdateDecision` — decisions link to Project via BELONGS_TO.

- [ ] **Step 5: Update claim.go**

Update `ClaimSpec`, `UnclaimSpec`, `Heartbeat`, `ReleaseExpiredClaims` to scope through Project.

- [ ] **Step 6: Update constitution.go**

Constitution uses `HAS_CONSTITUTION` edge to Project (not `BELONGS_TO`). Update `GetConstitution` to: `MATCH (p:Project {slug: $project})-[:HAS_CONSTITUTION]->(c:Constitution)`. Update `UpdateConstitution` to create/update the Constitution node linked to Project.

- [ ] **Step 7: Update authoring.go**

All authoring stage transitions (Spark, Shape, Specify, Decompose, Approve) scope through Project.

- [ ] **Step 8: Update execution.go**

Bundle generation, progress recording, prime data queries scope through Project.

- [ ] **Step 9: Update lifecycle.go**

Amend, Supersede, Abandon, drift operations scope through Project.

- [ ] **Step 10: Update sync.go**

Sync mapping, external ref operations scope through Project.

- [ ] **Step 11: Run full integration test suite**

Run: `go test ./internal/storage/memgraph/ -tags integration -v -count=1 -timeout=120s`
Expected: ALL PASS

- [ ] **Step 12: Run unit tests**

Run: `task test`
Expected: ALL PASS

- [ ] **Step 13: Commit**

```bash
task license:add
git add internal/storage/memgraph/
git commit -m "feat(memgraph): add BELONGS_TO edge namespacing to all Cypher queries"
```

---

### Task 8: Create Memgraph Indexes for Project Scoping

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go` (add index creation to constructor)

- [ ] **Step 1: Add index creation to New()**

After ensuring the Project node in the `New()` constructor, create indexes:

```go
indexes := []string{
	"CREATE INDEX ON :Project(slug)",
	"CREATE INDEX ON :Spec(slug)",
	"CREATE INDEX ON :Decision(slug)",
	"CREATE INDEX ON :ExternalRef(external_id)",
}
for _, idx := range indexes {
	if _, err := neo4j.ExecuteQuery(ctx, s.driver, idx, nil, neo4j.EagerResultTransformer); err != nil {
		// Memgraph returns error if index already exists — ignore.
		// The error message contains "already exists".
		if !strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf("create index: %w", err)
		}
	}
}
```

- [ ] **Step 2: Run integration tests**

Run: `go test ./internal/storage/memgraph/ -tags integration -v -count=1 -timeout=120s`
Expected: ALL PASS

- [ ] **Step 3: Commit**

```bash
git add internal/storage/memgraph/memgraph.go
git commit -m "feat(memgraph): create indexes for Project, Spec, Decision, ExternalRef"
```

---

## Chunk 3: Server Lifecycle & New CLI Commands

### Task 9: Service Manager Package (launchd/systemd)

**Files:**

- Create: `internal/service/service.go`
- Create: `internal/service/launchd.go`
- Create: `internal/service/systemd.go`
- Create: `internal/service/service_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/service/service_test.go
package service_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/specgraph/specgraph/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateDefinition_macOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	dir := t.TempDir()
	cfg := service.Config{
		BinaryPath: "/usr/local/bin/specgraph",
		ConfigPath: "/tmp/test-config.yaml",
		LogPath:    "/tmp/test-server.log",
	}
	path, err := service.Generate(dir, cfg)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "com.specgraph.server.plist"), path)
}

func TestGenerateDefinition_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := t.TempDir()
	cfg := service.Config{
		BinaryPath: "/usr/local/bin/specgraph",
		ConfigPath: "/tmp/test-config.yaml",
		LogPath:    "/tmp/test-server.log",
	}
	path, err := service.Generate(dir, cfg)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "specgraph.service"), path)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -v -count=1`
Expected: FAIL — package doesn't exist

- [ ] **Step 3: Write the implementation**

```go
// internal/service/service.go
package service

import (
	"fmt"
	"runtime"
)

// Config holds parameters for generating a service definition.
type Config struct {
	BinaryPath string
	ConfigPath string
	LogPath    string
}

// Generate writes a platform-appropriate service definition to dir.
// Returns the path to the generated file.
func Generate(dir string, cfg Config) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return generateLaunchd(dir, cfg)
	case "linux":
		return generateSystemd(dir, cfg)
	default:
		return "", fmt.Errorf("unsupported platform: %s (use server.mode=manual)", runtime.GOOS)
	}
}

// Install loads the service definition so the OS manages the process.
func Install(defPath string) error {
	switch runtime.GOOS {
	case "darwin":
		return installLaunchd(defPath)
	case "linux":
		return installSystemd(defPath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// Uninstall stops and removes the service.
func Uninstall(defPath string) error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallLaunchd(defPath)
	case "linux":
		return uninstallSystemd()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// Stop stops the service without removing it.
func Stop() error {
	switch runtime.GOOS {
	case "darwin":
		return stopLaunchd()
	case "linux":
		return stopSystemd()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
```

Write `launchd.go` (plist template generation, `launchctl bootstrap`/`bootout` calls) and `systemd.go` (unit file template, `systemctl --user` calls). Each uses `text/template` and `os/exec`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/service/ -v -count=1`
Expected: PASS

- [ ] **Step 5: Add license header and commit**

```bash
task license:add
git add internal/service/
git commit -m "feat(service): platform-aware service manager (launchd + systemd)"
```

---

### Task 10: `specgraph up` Command

**Files:**

- Create: `cmd/specgraph/up.go`
- Create: `cmd/specgraph/up_test.go`

- [ ] **Step 1: Write the `up` command**

Implements the algorithm from the design doc: load config (write defaults if missing) → check health → docker compose up → generate/install service (or foreground in manual mode) → health check loop → print status.

- [ ] **Step 2: Write a test for manual mode (foreground exec)**

Test that in manual mode, `up` runs `serve` in the foreground. Test that in service mode, `up` generates the service definition and calls install.

- [ ] **Step 3: Run tests**

Run: `go test ./cmd/specgraph/ -run TestUp -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
task license:add
git add cmd/specgraph/up.go cmd/specgraph/up_test.go
git commit -m "feat(cli): specgraph up command with service/manual modes"
```

---

### Task 11: `specgraph down` Command

**Files:**

- Create: `cmd/specgraph/down.go`

- [ ] **Step 1: Write the `down` command**

Implements: stop service → optionally remove with `--rm` → docker compose down → print status.

- [ ] **Step 2: Verify it works manually**

Run: `go run ./cmd/specgraph down --help`
Expected: Shows help with `--rm` flag

- [ ] **Step 3: Commit**

```bash
task license:add
git add cmd/specgraph/down.go
git commit -m "feat(cli): specgraph down command with --rm flag"
```

---

### Task 12: `specgraph prime` Command

**Files:**

- Create: `cmd/specgraph/prime.go`

- [ ] **Step 1: Write the `prime` command**

Implements: run up logic (idempotent) → find .specgraph.yaml → resolve server → ensure project in graph → output orientation context (project slug, constitution summary, non-terminal specs table).

- [ ] **Step 2: Write a test for orientation context output**

Test that prime outputs a table of non-terminal specs with slug, stage, priority columns.

- [ ] **Step 3: Run tests**

Run: `go test ./cmd/specgraph/ -run TestPrime -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
task license:add
git add cmd/specgraph/prime.go
git commit -m "feat(cli): specgraph prime command for session initialization"
```

---

### Task 13: Rework `specgraph init`

**Files:**

- Modify: `cmd/specgraph/init.go` (rewrite to use new config, project registration, interactive constitution)

- [ ] **Step 1: Rewrite init to use global config + project registration**

The new init: ensures server is running (calls up logic) → determines project slug → writes `.specgraph.yaml` → creates Project node in graph → optional interactive constitution setup → prints summary.

Add flags: `--yes` (non-interactive), `--constitution=<file>` (import from file).

- [ ] **Step 2: Write test for non-interactive mode**

Test that `specgraph init test-slug --yes` creates `.specgraph.yaml` and Project node without prompting.

- [ ] **Step 3: Run tests**

Run: `go test ./cmd/specgraph/ -run TestInit -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
task license:add
git add cmd/specgraph/init.go
git commit -m "feat(cli): rework init for global daemon with project registration"
```

---

### Task 14: `specgraph constitution import` Command

**Files:**

- Create: `cmd/specgraph/constitution_import.go`

- [ ] **Step 1: Write the import subcommand**

Reads YAML from file argument or stdin → parses into Constitution domain type → calls UpdateConstitution RPC → prints confirmation.

- [ ] **Step 2: Test with a sample constitution YAML**

- [ ] **Step 3: Commit**

```bash
task license:add
git add cmd/specgraph/constitution_import.go
git commit -m "feat(cli): specgraph constitution import from file or stdin"
```

---

### Task 15: Update Client Resolution to Use New Config

**Files:**

- Modify: `cmd/specgraph/client.go` (use GlobalConfig + ProjectConfig instead of old Config)

- [ ] **Step 1: Update `resolveBaseURL()` to use new config chain**

Load project config from CWD → load global config → resolve server via `GlobalConfig.ResolveServer(slug, repoOverride)`.

- [ ] **Step 2: Update all CLI commands that use the old Config to use new path**

The `cfgFile` flag on root command needs to switch to the global config path. Commands that need project context (most of them) should resolve project first.

- [ ] **Step 3: Run `task check`**

Run: `task check`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/
git commit -m "refactor(cli): switch all commands to global + project config resolution"
```

---

### Task 16: Update `specgraph serve` for New Config

**Files:**

- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Update serve to read from global config**

Replace `config.Load(cfgFile)` with `config.LoadGlobal(xdg.ConfigFile())`. Read `server.listen`, `server.backend`, `server.memgraph.bolt_uri`, `server.docker` instead of old fields. Remove `bootstrapConstitution()` call and `constitutionPath` logic.

The server needs the project slug from the request context (header), not from config. Each RPC call includes the project slug — the handler creates a project-scoped store.

- [ ] **Step 2: Remove old per-project config code**

Delete or deprecate `StorageConfig.ConstitutionPath`, `bootstrapConstitution()`, the old `Config.IsRemote()` path.

- [ ] **Step 3: Run `task check`**

Run: `task check`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/serve.go internal/config/
git commit -m "refactor(serve): use global config, remove constitution bootstrap"
```

---

### Task 17: Wire Project Slug Through RPC Context

**Files:**

- Modify: `internal/server/server.go` (add project middleware)
- Modify: All handler files to extract project from context

The server needs to know which project a request targets. The CLI sends a `X-Specgraph-Project` header; the server extracts it and passes it through context.

- [ ] **Step 1: Add project context middleware**

```go
// internal/server/project.go
package server

import (
	"context"
	"net/http"
)

type projectKey struct{}

// ProjectFromContext extracts the project slug from the request context.
func ProjectFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(projectKey{}).(string); ok {
		return v
	}
	return ""
}

// ProjectMiddleware extracts X-Specgraph-Project header and adds to context.
func ProjectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := r.Header.Get("X-Specgraph-Project")
		if project != "" {
			r = r.WithContext(context.WithValue(r.Context(), projectKey{}, project))
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: Update client to send project header**

In `cmd/specgraph/client.go`, the HTTP client needs to add the `X-Specgraph-Project` header. Update `newHTTPClient()` to return a client that injects the header.

- [ ] **Step 3: Add `Store.Scoped(ctx, project)` method**

The server holds a single Store (created at startup with a bootstrap project like `"_server"`). When a request arrives with a project header, the handler calls `store.Scoped(project)` to get a lightweight copy that shares the same neo4j driver but targets a different project.

```go
// In internal/storage/memgraph/memgraph.go, add:
// Scoped returns a new Store that shares this Store's driver but targets a different project.
// The Project node is ensured via MERGE on first use.
func (s *Store) Scoped(ctx context.Context, project string) (*Store, error) {
	if project == "" {
		return nil, fmt.Errorf("project slug required")
	}
	scoped := &Store{driver: s.driver, nowFunc: s.nowFunc, project: project}
	// Ensure the Project node exists (MERGE is idempotent).
	_, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MERGE (p:Project {slug: $slug})
		 ON CREATE SET p.created_at = $now, p.updated_at = $now,
		               p.sync_adapters = [], p.github_repo = ""`,
		map[string]any{"slug": project, "now": scoped.now()},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("ensure project node: %w", err)
	}
	return scoped, nil
}
```

This avoids calling `New()` per request (which runs `VerifyConnectivity` + index creation). The neo4j driver handles connection pooling internally. The `MERGE` for the Project node is cheap (idempotent, indexed).

In the server, `serve.go` creates the root Store at startup. The project middleware extracts the header, and each handler calls `store.Scoped(ctx, project)` at the top of each RPC method.

- [ ] **Step 4: Run `task check`**

Run: `task check`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
task license:add
git add internal/server/project.go cmd/specgraph/client.go
git commit -m "feat(server): project context middleware and per-request store scoping"
```

---

## Chunk 4: Cleanup & Verification

### Task 18: Remove Old Per-Project Config Code

**Files:**

- Modify: `internal/config/config.go` (remove or mark deprecated)
- Remove: `.specgraph/` references in docker compose template path
- Modify: `internal/docker/compose.go` (write to XDG data home instead of `.specgraph/`)

- [ ] **Step 1: Update docker compose to use XDG data home**

Change `EnsureComposeFile` to write to `~/.local/share/specgraph/docker-compose.yaml` instead of `.specgraph/docker-compose.yaml`.

- [ ] **Step 2: Remove unused config fields and methods**

Remove `StorageConfig.ConstitutionPath`, `Config.IsRemote()` path that uses old config, `LoadConstitutionYAML()` (replaced by `constitution import` command).

- [ ] **Step 3: Run `task check`**

Run: `task check`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add internal/config/ internal/docker/
git commit -m "refactor: remove old per-project config, move compose to XDG data home"
```

---

### Task 19: Integration Test — Full Prime Flow

**Files:**

- Create: `internal/storage/memgraph/prime_integration_test.go`

- [ ] **Step 1: Write integration test**

Test the full flow: create project → create specs → query non-terminal specs → verify BELONGS_TO edges.

```go
func TestPrimeFlow_Integration(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI, memgraph.WithProject("prime-test"))
	require.NoError(t, err)
	defer store.Close(ctx)

	// Project was auto-ensured by constructor.
	p, err := store.GetProject(ctx, "prime-test")
	require.NoError(t, err)
	assert.Equal(t, "prime-test", p.Slug)

	// Create a spec — should have BELONGS_TO edge.
	spec, err := store.CreateSpec(ctx, "test-spec", "Test intent", "p2", "medium")
	require.NoError(t, err)
	assert.Equal(t, "test-spec", spec.Slug)

	// Verify the spec is scoped to this project.
	fetched, err := store.GetSpec(ctx, "test-spec")
	require.NoError(t, err)
	assert.Equal(t, spec.Slug, fetched.Slug)

	// Create a second store with different project — should NOT see the spec.
	store2, err := newStore(ctx, boltURI, memgraph.WithProject("other-project"))
	require.NoError(t, err)
	defer store2.Close(ctx)

	_, err = store2.GetSpec(ctx, "test-spec")
	assert.ErrorIs(t, err, storage.ErrSpecNotFound)
}
```

- [ ] **Step 2: Run integration test**

Run: `go test ./internal/storage/memgraph/ -run TestPrimeFlow -tags integration -v -count=1 -timeout=120s`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
task license:add
git add internal/storage/memgraph/prime_integration_test.go
git commit -m "test(memgraph): integration test for project isolation via BELONGS_TO edges"
```

---

### Task 20: Run Full Quality Gate

- [ ] **Step 1: Run `task check`**

Run: `task check`
Expected: ALL PASS (fmt, license, lint, build, unit tests)

- [ ] **Step 2: Run integration tests**

Run: `go test ./internal/storage/memgraph/ -tags integration -v -count=1 -timeout=120s`
Expected: ALL PASS

- [ ] **Step 3: Run e2e tests**

Run: `go test ./e2e/... -tags e2e -v -count=1 -timeout=120s`
Expected: PASS (or expected failures documented)

- [ ] **Step 4: Commit any fixups**

---

## Chunk 5: Claude Code Plugin

### Task 21: Plugin Manifest and SessionStart Hook

**Files:**

- Create: `plugin/specgraph/plugin.json`
- Create: `plugin/specgraph/hooks/session-start.sh`

- [ ] **Step 1: Write plugin.json**

Copy the plugin.json from the design doc verbatim. Validate JSON.

- [ ] **Step 2: Write session-start.sh**

```bash
#!/usr/bin/env bash
exec specgraph prime 2>&1
```

- [ ] **Step 3: Make hook executable and verify JSON**

```bash
chmod +x plugin/specgraph/hooks/session-start.sh
python3 -c "import json; json.load(open('plugin/specgraph/plugin.json'))"
```

- [ ] **Step 4: Commit**

```bash
task license:add
git add plugin/specgraph/plugin.json plugin/specgraph/hooks/
git commit -m "feat(plugin): plugin.json manifest and SessionStart hook"
```

---

### Task 22: Meta-Skill (Router)

**Files:**

- Create: `plugin/specgraph/skills/specgraph/SKILL.md`

- [ ] **Step 1: Write the meta-skill**

The meta-skill routes to sub-skills based on user intent. Uses the routing logic from the design doc: slug → show, keyword → invoke sub-skill, vague idea → spark, status → list, empty → show commands.

- [ ] **Step 2: Commit**

```bash
git add plugin/specgraph/skills/specgraph/SKILL.md
git commit -m "feat(plugin): meta-skill for SpecGraph overview and routing"
```

---

### Task 23: Authoring Skills (Spark, Shape, Specify, Decompose, Approve)

**Files:**

- Create: `plugin/specgraph/skills/specgraph/spark/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/shape/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/specify/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/decompose/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/approve/SKILL.md`

Each skill follows the pattern from the design doc: Prerequisites → Workflow (load context → interactive phase → persist → next steps).

- [ ] **Step 1: Write all five authoring skills**

Use the detailed skill content from the original Slice 7 plan (`2026-02-28-slice-7-claude-code-plugin-plan.md`) as the basis, adapted for the new global daemon model (no constitution YAML file references, use `specgraph constitution show` instead).

- [ ] **Step 2: Commit**

```bash
git add plugin/specgraph/skills/specgraph/spark/ plugin/specgraph/skills/specgraph/shape/ plugin/specgraph/skills/specgraph/specify/ plugin/specgraph/skills/specgraph/decompose/ plugin/specgraph/skills/specgraph/approve/
git commit -m "feat(plugin): authoring skills (spark, shape, specify, decompose, approve)"
```

---

### Task 24: Query Skills (List, Show, Deps, Ready)

**Files:**

- Create: `plugin/specgraph/skills/specgraph/list/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/show/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/deps/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/ready/SKILL.md`

These are thin wrappers: run CLI command → format output → present to user.

- [ ] **Step 1: Write all four query skills**

- [ ] **Step 2: Commit**

```bash
git add plugin/specgraph/skills/specgraph/list/ plugin/specgraph/skills/specgraph/show/ plugin/specgraph/skills/specgraph/deps/ plugin/specgraph/skills/specgraph/ready/
git commit -m "feat(plugin): query skills (list, show, deps, ready)"
```

---

### Task 25: Bundle Skill

**Files:**

- Create: `plugin/specgraph/skills/specgraph/bundle/SKILL.md`

- [ ] **Step 1: Write the bundle skill**

Wraps `specgraph bundle <slug>` → generates execution bundle → offers to inject context via `specgraph inject <slug>`.

- [ ] **Step 2: Commit**

```bash
git add plugin/specgraph/skills/specgraph/bundle/
git commit -m "feat(plugin): bundle skill for execution context generation"
```

---

### Task 26: Final Verification

- [ ] **Step 1: Validate plugin structure**

```bash
# Verify all skill paths in plugin.json resolve to actual files.
python3 -c "
import json, os
manifest = json.load(open('plugin/specgraph/plugin.json'))
for s in manifest['skills']:
    path = os.path.join('plugin/specgraph', s['path'])
    assert os.path.exists(path), f'Missing: {path}'
print('All skill paths valid')
"
```

- [ ] **Step 2: Run full quality gate**

Run: `task check`
Expected: ALL PASS

- [ ] **Step 3: Run integration tests**

Run: `go test ./internal/storage/memgraph/ -tags integration -v -count=1 -timeout=120s`
Expected: ALL PASS

- [ ] **Step 4: Verify CLI commands**

```bash
go run ./cmd/specgraph up --help
go run ./cmd/specgraph down --help
go run ./cmd/specgraph prime --help
go run ./cmd/specgraph init --help
go run ./cmd/specgraph constitution import --help
```

- [ ] **Step 5: Update implementation tracker**

Mark Slice 7 as complete in `docs/plans/2026-02-28-implementation-tracker.md`.

- [ ] **Step 6: Final commit**

```bash
git add docs/plans/2026-02-28-implementation-tracker.md
git commit -m "docs: mark Slice 7 complete in implementation tracker"
```

---

## Summary

| Chunk | Tasks | Focus |
|-------|-------|-------|
| 1 | 1-3 | Config rewrite: XDG paths, global config schema, project reader |
| 2 | 4-8 | Graph namespacing: Project node, BELONGS_TO edges, indexes |
| 3 | 9-17 | Server lifecycle: service manager, up/down/prime/init commands, client wiring |
| 4 | 18-20 | Cleanup: remove old config, integration tests, quality gate |
| 5 | 21-26 | Plugin: manifest, hook, 11 skills, final verification |

**Total:** 26 tasks across 5 chunks. Phase A (Chunks 1-4) must complete before Phase B (Chunk 5).
