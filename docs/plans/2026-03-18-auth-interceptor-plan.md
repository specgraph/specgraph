# Auth Interceptor Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add authentication and authorization to all SpecGraph ConnectRPC services via a single interceptor with API key support and implicit local identity fallback.

**Architecture:** A ConnectRPC unary interceptor resolves identity from Bearer tokens (API keys now, JWT/OIDC later), checks fine-grained `service:action` permissions against the RPC procedure name, and injects the identity into request context. When no API keys are configured, the system creates an implicit admin identity from the OS user.

**Tech Stack:** Go, ConnectRPC interceptors, `crypto/subtle` for constant-time key comparison, `log/slog` for audit logging, `gopkg.in/yaml.v3` for config.

**Spec:** `docs/plans/2026-03-18-auth-interceptor-design.md`

---

## File Structure

### New files (`internal/auth/`)

| File | Responsibility |
|------|---------------|
| `internal/auth/auth.go` | `Identity` type, `Role` constants, `HasPermission()` helper |
| `internal/auth/context.go` | `IdentityFromContext()` / `WithIdentity()` context helpers |
| `internal/auth/store.go` | `IdentityStore` interface, `ErrUnknownKey` sentinel |
| `internal/auth/config_store.go` | Config-file-backed `IdentityStore` implementation |
| `internal/auth/permissions.go` | `rpcPermissions` table, `exemptProcedures` set, default role bundles |
| `internal/auth/interceptor.go` | `NewAuthInterceptor()` — the ConnectRPC unary interceptor |

### New test files

| File | Tests |
|------|-------|
| `internal/auth/auth_test.go` | `HasPermission()` wildcard resolution |
| `internal/auth/config_store_test.go` | Config store: resolve, HasKeys, duplicates, constant-time |
| `internal/auth/permissions_test.go` | Permission table completeness, role resolution |
| `internal/auth/interceptor_test.go` | Full interceptor flow (8 scenarios from spec) |

### Modified files

| File | Change |
|------|--------|
| `internal/config/global.go` | Add `Auth AuthConfig` to `GlobalConfig`, add `AuthConfig`/`APIKeyConfig`/`RoleConfig` types |
| `internal/server/server.go` | `NewMux()` accepts `...connect.HandlerOption`, passes to handler constructors |
| `internal/server/health_handler.go` | `RegisterHealthService()` accepts `...connect.HandlerOption` |
| `internal/server/decision_handler.go` | `RegisterDecisionService()` accepts `...connect.HandlerOption` |
| `internal/server/graph_handler.go` | `RegisterGraphService()` accepts `...connect.HandlerOption` |
| `internal/server/claim_handler.go` | `RegisterClaimService()` accepts `...connect.HandlerOption` |
| `internal/server/constitution_handler.go` | `RegisterConstitutionService()` accepts `...connect.HandlerOption` |
| `internal/server/authoring_handler.go` | `RegisterAuthoringService()` accepts `...connect.HandlerOption` |
| `internal/server/execution_handler.go` | `RegisterExecutionService()` accepts `...connect.HandlerOption` |
| `internal/server/lifecycle_handler.go` | `RegisterLifecycleService()` accepts `...connect.HandlerOption` |
| `internal/server/sync_handler.go` | `RegisterSyncService()` accepts `...connect.HandlerOption` |
| `cmd/specgraph/serve.go` | Create interceptor from config, pass as handler option |
| `e2e/testutil/server.go` | Wire interceptor in E2E test server |

---

## Chunk 1: Identity Model & Permission Helpers

### Task 1: Identity Type and HasPermission

**Files:**

- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`

- [ ] **Step 1: Write failing tests for HasPermission wildcard resolution**

```go
// internal/auth/auth_test.go
package auth_test

import (
	"testing"

	"github.com/seanb4t/specgraph/internal/auth"
)

func TestHasPermission_ExactMatch(t *testing.T) {
	perms := map[string]bool{"spec:read": true, "spec:write": true}
	if !auth.HasPermission(perms, "spec:read") {
		t.Fatal("expected spec:read to match")
	}
	if auth.HasPermission(perms, "spec:delete") {
		t.Fatal("expected spec:delete to not match")
	}
}

func TestHasPermission_FullWildcard(t *testing.T) {
	perms := map[string]bool{"*:*": true}
	if !auth.HasPermission(perms, "spec:read") {
		t.Fatal("expected *:* to match spec:read")
	}
	if !auth.HasPermission(perms, "lifecycle:delete") {
		t.Fatal("expected *:* to match lifecycle:delete")
	}
}

func TestHasPermission_ActionWildcard(t *testing.T) {
	perms := map[string]bool{"*:read": true}
	if !auth.HasPermission(perms, "spec:read") {
		t.Fatal("expected *:read to match spec:read")
	}
	if auth.HasPermission(perms, "spec:write") {
		t.Fatal("expected *:read to not match spec:write")
	}
}

func TestHasPermission_ServiceWildcard(t *testing.T) {
	perms := map[string]bool{"spec:*": true}
	if !auth.HasPermission(perms, "spec:read") {
		t.Fatal("expected spec:* to match spec:read")
	}
	if !auth.HasPermission(perms, "spec:delete") {
		t.Fatal("expected spec:* to match spec:delete")
	}
	if auth.HasPermission(perms, "decision:read") {
		t.Fatal("expected spec:* to not match decision:read")
	}
}

func TestHasPermission_EmptyPerms(t *testing.T) {
	if auth.HasPermission(nil, "spec:read") {
		t.Fatal("nil perms should deny everything")
	}
	if auth.HasPermission(map[string]bool{}, "spec:read") {
		t.Fatal("empty perms should deny everything")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test ./internal/auth/ -run TestHasPermission -v`
Expected: FAIL — package does not exist yet

- [ ] **Step 3: Write Identity type, Role constants, and HasPermission**

```go
// internal/auth/auth.go
package auth

import "strings"

// Role represents a named authorization role.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleWriter Role = "writer"
	RoleReader Role = "reader"
)

// Identity represents an authenticated principal.
type Identity struct {
	Subject     string          // "local:<user>" | "apikey:<id>" | "oidc:<sub>"
	DisplayName string          // human-friendly name
	Role        Role            // role name (built-in or custom)
	Permissions map[string]bool // raw entries from role definition
	Source      string          // "local" | "apikey" | "oidc"
}

// HasPermission checks whether perms satisfies the required permission.
// Supports wildcards: "*:*" (full), "*:read" (action), "spec:*" (service).
func HasPermission(perms map[string]bool, required string) bool {
	if len(perms) == 0 {
		return false
	}
	if perms["*:*"] {
		return true
	}
	if perms[required] {
		return true
	}
	parts := strings.SplitN(required, ":", 2)
	if len(parts) != 2 {
		return false
	}
	service, action := parts[0], parts[1]
	if perms[service+":*"] {
		return true
	}
	if perms["*:"+action] {
		return true
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test ./internal/auth/ -run TestHasPermission -v`
Expected: PASS (all 5 tests)

- [ ] **Step 5: Commit**

```text
feat(auth): add Identity type and HasPermission with wildcard resolution
```

### Task 2: Context Helpers

**Files:**

- Create: `internal/auth/context.go`

- [ ] **Step 1: Write context.go with IdentityFromContext and WithIdentity**

```go
// internal/auth/context.go
package auth

import "context"

type contextKey struct{}

// WithIdentity returns a new context carrying the given identity.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// IdentityFromContext extracts the identity from the context.
// Returns nil, false if no identity is present (e.g., exempt RPCs).
func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(contextKey{}).(*Identity)
	return id, ok
}
```

No test needed — trivial context value wrapper, tested transitively by interceptor tests.

- [ ] **Step 2: Commit**

```text
feat(auth): add context helpers for Identity injection/extraction
```

### Task 3: IdentityStore Interface

**Files:**

- Create: `internal/auth/store.go`

- [ ] **Step 1: Write store.go with interface and sentinel error**

```go
// internal/auth/store.go
package auth

import (
	"context"
	"errors"
)

// ErrUnknownKey is returned when an API key is not recognized.
var ErrUnknownKey = errors.New("unknown API key")

// IdentityStore resolves authentication tokens to identities.
type IdentityStore interface {
	// ResolveAPIKey returns the identity for the given API key.
	// Returns ErrUnknownKey if the key is not recognized.
	ResolveAPIKey(ctx context.Context, key string) (*Identity, error)

	// HasKeys reports whether any API keys are configured.
	// When false, unauthenticated requests fall back to the implicit local identity.
	HasKeys() bool
}
```

- [ ] **Step 2: Commit**

```text
feat(auth): add IdentityStore interface and ErrUnknownKey sentinel
```

---

## Chunk 2: Config Store Implementation

### Task 4: Auth Config Types

**Files:**

- Modify: `internal/config/global.go`

- [ ] **Step 1: Add AuthConfig types to global.go**

Add to `GlobalConfig`:

```go
type GlobalConfig struct {
	Server ServerSection `yaml:"server"`
	Client ClientConfig  `yaml:"client"`
	Auth   AuthConfig    `yaml:"auth"`
}
```

Add new types:

```go
// AuthConfig configures authentication and authorization.
type AuthConfig struct {
	APIKeys []APIKeyConfig        `yaml:"api_keys"`
	Roles   map[string]RoleConfig `yaml:"roles"`
}

// APIKeyConfig defines a single API key and its associated role.
type APIKeyConfig struct {
	ID   string `yaml:"id"`
	Key  string `yaml:"key"`
	Name string `yaml:"name"`
	Role string `yaml:"role"`
}

// RoleConfig defines a custom role with explicit permissions.
type RoleConfig struct {
	Permissions []string `yaml:"permissions"`
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```text
feat(config): add AuthConfig types for API keys and custom roles
```

### Task 5: Config-File IdentityStore

**Files:**

- Create: `internal/auth/config_store.go`
- Create: `internal/auth/config_store_test.go`

- [ ] **Step 1: Write failing tests for ConfigStore**

```go
// internal/auth/config_store_test.go
package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/seanb4t/specgraph/internal/auth"
	"github.com/seanb4t/specgraph/internal/config"
)

func TestConfigStore_ResolveAPIKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Test Key", Role: "admin"},
		},
	}
	store, err := auth.NewConfigStore(cfg)
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	id, err := store.ResolveAPIKey(context.Background(), "spgr_sk_abc")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if id.Subject != "apikey:k1" {
		t.Errorf("subject = %q, want apikey:k1", id.Subject)
	}
	if id.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want admin", id.Role)
	}
	if id.Source != "apikey" {
		t.Errorf("source = %q, want apikey", id.Source)
	}
}

func TestConfigStore_UnknownKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Test Key", Role: "admin"},
		},
	}
	store, err := auth.NewConfigStore(cfg)
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	_, err = store.ResolveAPIKey(context.Background(), "wrong_key")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("err = %v, want ErrUnknownKey", err)
	}
}

func TestConfigStore_HasKeys(t *testing.T) {
	empty, err := auth.NewConfigStore(config.AuthConfig{})
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if empty.HasKeys() {
		t.Error("HasKeys() = true for empty config")
	}

	withKey, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Key", Role: "reader"},
		},
	})
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if !withKey.HasKeys() {
		t.Error("HasKeys() = false for config with keys")
	}
}

func TestConfigStore_DuplicateKeyID(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Key 1", Role: "admin"},
			{ID: "k1", Key: "spgr_sk_def", Name: "Key 2", Role: "reader"},
		},
	}
	_, err := auth.NewConfigStore(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate key ID")
	}
}

func TestConfigStore_DuplicateKeyValue(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_same", Name: "Key 1", Role: "admin"},
			{ID: "k2", Key: "spgr_sk_same", Name: "Key 2", Role: "reader"},
		},
	}
	_, err := auth.NewConfigStore(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate key value")
	}
}

func TestConfigStore_UnknownRole(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Key", Role: "nonexistent"},
		},
	}
	_, err := auth.NewConfigStore(cfg)
	if err == nil {
		t.Fatal("expected error for unknown role")
	}
}

func TestConfigStore_CustomRole(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "CI", Role: "ci-readonly"},
		},
		Roles: map[string]config.RoleConfig{
			"ci-readonly": {Permissions: []string{"spec:read", "decision:read"}},
		},
	}
	store, err := auth.NewConfigStore(cfg)
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	id, err := store.ResolveAPIKey(context.Background(), "spgr_sk_abc")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if !auth.HasPermission(id.Permissions, "spec:read") {
		t.Error("expected spec:read permission")
	}
	if auth.HasPermission(id.Permissions, "spec:write") {
		t.Error("unexpected spec:write permission")
	}
}

func TestConfigStore_BuiltinRolePermissions(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_r", Name: "Reader", Role: "reader"},
			{ID: "k2", Key: "spgr_sk_w", Name: "Writer", Role: "writer"},
			{ID: "k3", Key: "spgr_sk_a", Name: "Admin", Role: "admin"},
		},
	}
	store, err := auth.NewConfigStore(cfg)
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	reader, _ := store.ResolveAPIKey(context.Background(), "spgr_sk_r")
	if !auth.HasPermission(reader.Permissions, "spec:read") {
		t.Error("reader should have spec:read")
	}
	if auth.HasPermission(reader.Permissions, "spec:write") {
		t.Error("reader should not have spec:write")
	}

	writer, _ := store.ResolveAPIKey(context.Background(), "spgr_sk_w")
	if !auth.HasPermission(writer.Permissions, "spec:write") {
		t.Error("writer should have spec:write")
	}
	if auth.HasPermission(writer.Permissions, "graph:delete") {
		t.Error("writer should not have graph:delete")
	}

	admin, _ := store.ResolveAPIKey(context.Background(), "spgr_sk_a")
	if !auth.HasPermission(admin.Permissions, "graph:delete") {
		t.Error("admin should have graph:delete via *:*")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test ./internal/auth/ -run TestConfigStore -v`
Expected: FAIL — `NewConfigStore` not defined

- [ ] **Step 3: Write ConfigStore implementation**

```go
// internal/auth/config_store.go
package auth

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/seanb4t/specgraph/internal/config"
)

// DefaultRolePermissions defines the built-in role permission bundles.
var DefaultRolePermissions = map[Role][]string{
	RoleReader: {"*:read"},
	RoleWriter: {"*:read", "*:write"},
	RoleAdmin:  {"*:*"},
}

// ConfigStore implements IdentityStore backed by config file data.
// It is immutable after construction.
type ConfigStore struct {
	identities map[string]*Identity // key value → identity
	hasKeys    bool
}

// NewConfigStore creates an IdentityStore from the auth config.
// Returns an error if config is invalid (duplicate IDs, duplicate key values,
// unknown roles).
func NewConfigStore(cfg config.AuthConfig) (*ConfigStore, error) {
	// Build role lookup: built-in defaults + custom roles (custom overrides built-in)
	roles := make(map[string][]string)
	for role, perms := range DefaultRolePermissions {
		roles[string(role)] = perms
	}
	for name, rc := range cfg.Roles {
		roles[name] = rc.Permissions
	}

	identities := make(map[string]*Identity, len(cfg.APIKeys))
	seenIDs := make(map[string]bool, len(cfg.APIKeys))

	for _, ak := range cfg.APIKeys {
		if seenIDs[ak.ID] {
			return nil, fmt.Errorf("duplicate API key ID: %s", ak.ID)
		}
		seenIDs[ak.ID] = true

		if _, exists := identities[ak.Key]; exists {
			return nil, fmt.Errorf("duplicate API key value for IDs: %s", ak.ID)
		}

		perms, ok := roles[ak.Role]
		if !ok {
			return nil, fmt.Errorf("API key %s references unknown role: %s", ak.ID, ak.Role)
		}

		permMap := make(map[string]bool, len(perms))
		for _, p := range perms {
			permMap[p] = true
		}

		identities[ak.Key] = &Identity{
			Subject:     "apikey:" + ak.ID,
			DisplayName: ak.Name,
			Role:        Role(ak.Role),
			Permissions: permMap,
			Source:      "apikey",
		}
	}

	return &ConfigStore{
		identities: identities,
		hasKeys:    len(identities) > 0,
	}, nil
}

// ResolveAPIKey resolves an API key to an identity using constant-time comparison.
func (s *ConfigStore) ResolveAPIKey(_ context.Context, key string) (*Identity, error) {
	// Use constant-time comparison to prevent timing side-channels.
	for storedKey, id := range s.identities {
		if subtle.ConstantTimeCompare([]byte(storedKey), []byte(key)) == 1 {
			return id, nil
		}
	}
	return nil, ErrUnknownKey
}

// HasKeys reports whether any API keys are configured.
func (s *ConfigStore) HasKeys() bool {
	return s.hasKeys
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test ./internal/auth/ -run TestConfigStore -v`
Expected: PASS (all 8 tests)

- [ ] **Step 5: Commit**

```text
feat(auth): add ConfigStore with constant-time key comparison and validation
```

---

## Chunk 3: Permission Table & Interceptor

### Task 6: Permission Table

**Files:**

- Create: `internal/auth/permissions.go`
- Create: `internal/auth/permissions_test.go`

- [ ] **Step 1: Write the permission table completeness test**

This test imports all generated `*Procedure` constants and asserts coverage.

```go
// internal/auth/permissions_test.go
package auth_test

import (
	"testing"

	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/auth"
)

// allProcedures lists every ConnectRPC procedure from generated code.
var allProcedures = []string{
	// SpecService
	specgraphv1connect.SpecServiceCreateSpecProcedure,
	specgraphv1connect.SpecServiceGetSpecProcedure,
	specgraphv1connect.SpecServiceListSpecsProcedure,
	specgraphv1connect.SpecServiceUpdateSpecProcedure,
	// DecisionService
	specgraphv1connect.DecisionServiceCreateDecisionProcedure,
	specgraphv1connect.DecisionServiceGetDecisionProcedure,
	specgraphv1connect.DecisionServiceListDecisionsProcedure,
	specgraphv1connect.DecisionServiceUpdateDecisionProcedure,
	// GraphService
	specgraphv1connect.GraphServiceAddEdgeProcedure,
	specgraphv1connect.GraphServiceRemoveEdgeProcedure,
	specgraphv1connect.GraphServiceListEdgesProcedure,
	specgraphv1connect.GraphServiceGetDependenciesProcedure,
	specgraphv1connect.GraphServiceGetTransitiveDepsProcedure,
	specgraphv1connect.GraphServiceGetImpactProcedure,
	specgraphv1connect.GraphServiceGetReadyProcedure,
	specgraphv1connect.GraphServiceGetCriticalPathProcedure,
	// ClaimService
	specgraphv1connect.ClaimServiceClaimSpecProcedure,
	specgraphv1connect.ClaimServiceUnclaimSpecProcedure,
	specgraphv1connect.ClaimServiceHeartbeatProcedure,
	// ConstitutionService
	specgraphv1connect.ConstitutionServiceGetConstitutionProcedure,
	specgraphv1connect.ConstitutionServiceUpdateConstitutionProcedure,
	specgraphv1connect.ConstitutionServiceCheckViolationProcedure,
	specgraphv1connect.ConstitutionServiceEmitToolFilesProcedure,
	// AuthoringService
	specgraphv1connect.AuthoringServiceSparkProcedure,
	specgraphv1connect.AuthoringServiceShapeProcedure,
	specgraphv1connect.AuthoringServiceSpecifyProcedure,
	specgraphv1connect.AuthoringServiceDecomposeProcedure,
	specgraphv1connect.AuthoringServiceApproveProcedure,
	specgraphv1connect.AuthoringServiceAmendProcedure,
	specgraphv1connect.AuthoringServiceSupersedeProcedure,
	specgraphv1connect.AuthoringServiceGetPromptsProcedure,
	// ExecutionService
	specgraphv1connect.ExecutionServiceGenerateBundleProcedure,
	specgraphv1connect.ExecutionServiceGetPrimeProcedure,
	specgraphv1connect.ExecutionServiceReportProgressProcedure,
	specgraphv1connect.ExecutionServiceReportBlockerProcedure,
	specgraphv1connect.ExecutionServiceReportCompletionProcedure,
	specgraphv1connect.ExecutionServiceGetExecutionEventsProcedure,
	// LifecycleService
	specgraphv1connect.LifecycleServiceTransitionAmendProcedure,
	specgraphv1connect.LifecycleServiceTransitionSupersedeProcedure,
	specgraphv1connect.LifecycleServiceTransitionAbandonProcedure,
	specgraphv1connect.LifecycleServiceCheckDriftProcedure,
	specgraphv1connect.LifecycleServiceAcknowledgeDriftProcedure,
	specgraphv1connect.LifecycleServiceLintProcedure,
	// SyncService
	specgraphv1connect.SyncServiceSyncBeadsProcedure,
	specgraphv1connect.SyncServiceSyncGitHubProcedure,
	specgraphv1connect.SyncServiceGetSyncStatusProcedure,
	specgraphv1connect.SyncServiceInjectProcedure,
	// ServerService (exempt)
	specgraphv1connect.ServerServiceHealthProcedure,
}

func TestPermissionTable_Completeness(t *testing.T) {
	for _, proc := range allProcedures {
		if auth.IsExempt(proc) {
			continue
		}
		if _, ok := auth.RPCPermission(proc); !ok {
			t.Errorf("procedure %s is not in rpcPermissions and not exempt", proc)
		}
	}
}

func TestPermissionTable_NoExemptInPermissions(t *testing.T) {
	for _, proc := range allProcedures {
		if !auth.IsExempt(proc) {
			continue
		}
		if _, ok := auth.RPCPermission(proc); ok {
			t.Errorf("exempt procedure %s should not be in rpcPermissions", proc)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test ./internal/auth/ -run TestPermissionTable -v`
Expected: FAIL — `IsExempt`, `RPCPermission` not defined

- [ ] **Step 3: Write permissions.go**

```go
// internal/auth/permissions.go
package auth

import (
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// rpcPermissions maps ConnectRPC procedure names to required permissions.
var rpcPermissions = map[string]string{
	// SpecService
	specgraphv1connect.SpecServiceGetSpecProcedure:    "spec:read",
	specgraphv1connect.SpecServiceListSpecsProcedure:   "spec:read",
	specgraphv1connect.SpecServiceCreateSpecProcedure:  "spec:write",
	specgraphv1connect.SpecServiceUpdateSpecProcedure:  "spec:write",

	// DecisionService
	specgraphv1connect.DecisionServiceGetDecisionProcedure:    "decision:read",
	specgraphv1connect.DecisionServiceListDecisionsProcedure:   "decision:read",
	specgraphv1connect.DecisionServiceCreateDecisionProcedure:  "decision:write",
	specgraphv1connect.DecisionServiceUpdateDecisionProcedure:  "decision:write",

	// GraphService
	specgraphv1connect.GraphServiceGetDependenciesProcedure:    "graph:read",
	specgraphv1connect.GraphServiceGetTransitiveDepsProcedure:  "graph:read",
	specgraphv1connect.GraphServiceGetImpactProcedure:          "graph:read",
	specgraphv1connect.GraphServiceGetReadyProcedure:           "graph:read",
	specgraphv1connect.GraphServiceGetCriticalPathProcedure:    "graph:read",
	specgraphv1connect.GraphServiceListEdgesProcedure:          "graph:read",
	specgraphv1connect.GraphServiceAddEdgeProcedure:            "graph:write",
	specgraphv1connect.GraphServiceRemoveEdgeProcedure:         "graph:delete",

	// ClaimService
	specgraphv1connect.ClaimServiceClaimSpecProcedure:   "claim:write",
	specgraphv1connect.ClaimServiceHeartbeatProcedure:   "claim:write",
	specgraphv1connect.ClaimServiceUnclaimSpecProcedure: "claim:write",

	// ConstitutionService
	specgraphv1connect.ConstitutionServiceGetConstitutionProcedure:    "constitution:read",
	specgraphv1connect.ConstitutionServiceCheckViolationProcedure:     "constitution:read",
	specgraphv1connect.ConstitutionServiceUpdateConstitutionProcedure: "constitution:write",
	specgraphv1connect.ConstitutionServiceEmitToolFilesProcedure:      "constitution:write",

	// AuthoringService
	specgraphv1connect.AuthoringServiceGetPromptsProcedure:  "authoring:read",
	specgraphv1connect.AuthoringServiceSparkProcedure:       "authoring:write",
	specgraphv1connect.AuthoringServiceShapeProcedure:       "authoring:write",
	specgraphv1connect.AuthoringServiceSpecifyProcedure:     "authoring:write",
	specgraphv1connect.AuthoringServiceDecomposeProcedure:   "authoring:write",
	specgraphv1connect.AuthoringServiceApproveProcedure:     "authoring:write",
	specgraphv1connect.AuthoringServiceAmendProcedure:       "authoring:write",
	specgraphv1connect.AuthoringServiceSupersedeProcedure:   "authoring:write",

	// ExecutionService
	specgraphv1connect.ExecutionServiceGenerateBundleProcedure:     "execution:read",
	specgraphv1connect.ExecutionServiceGetPrimeProcedure:           "execution:read",
	specgraphv1connect.ExecutionServiceGetExecutionEventsProcedure: "execution:read",
	specgraphv1connect.ExecutionServiceReportProgressProcedure:     "execution:write",
	specgraphv1connect.ExecutionServiceReportBlockerProcedure:      "execution:write",
	specgraphv1connect.ExecutionServiceReportCompletionProcedure:   "execution:write",

	// LifecycleService
	specgraphv1connect.LifecycleServiceCheckDriftProcedure:          "lifecycle:read",
	specgraphv1connect.LifecycleServiceLintProcedure:                "lifecycle:read",
	specgraphv1connect.LifecycleServiceAcknowledgeDriftProcedure:    "lifecycle:write",
	specgraphv1connect.LifecycleServiceTransitionAmendProcedure:     "lifecycle:write",
	specgraphv1connect.LifecycleServiceTransitionSupersedeProcedure: "lifecycle:write",
	specgraphv1connect.LifecycleServiceTransitionAbandonProcedure:   "lifecycle:write",

	// SyncService
	specgraphv1connect.SyncServiceGetSyncStatusProcedure: "sync:read",
	specgraphv1connect.SyncServiceSyncBeadsProcedure:     "sync:write",
	specgraphv1connect.SyncServiceSyncGitHubProcedure:    "sync:write",
	specgraphv1connect.SyncServiceInjectProcedure:        "sync:write",
}

// exemptProcedures are RPCs that skip authentication entirely.
var exemptProcedures = map[string]bool{
	specgraphv1connect.ServerServiceHealthProcedure: true,
}

// RPCPermission returns the required permission for a procedure.
func RPCPermission(procedure string) (string, bool) {
	perm, ok := rpcPermissions[procedure]
	return perm, ok
}

// IsExempt reports whether a procedure is exempt from authentication.
func IsExempt(procedure string) bool {
	return exemptProcedures[procedure]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test ./internal/auth/ -run TestPermissionTable -v`
Expected: PASS (both tests)

- [ ] **Step 5: Commit**

```text
feat(auth): add RPC permission table with completeness test
```

### Task 7: Auth Interceptor

**Files:**

- Create: `internal/auth/interceptor.go`
- Create: `internal/auth/interceptor_test.go`

**Important:** ConnectRPC's `AnyRequest` interface has unexported methods, so it
cannot be faked directly. The interceptor tests use `httptest.Server` with a
real ConnectRPC handler (the Health service is simplest) and make HTTP requests
with appropriate Authorization headers. The interceptor is wired as a handler
option, so it runs on real requests through the full ConnectRPC stack.

- [ ] **Step 1: Write failing interceptor tests using httptest**

```go
// internal/auth/interceptor_test.go
package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	specgraphv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/auth"
	"github.com/seanb4t/specgraph/internal/config"
)

// stubHealthHandler returns a minimal Health handler for testing the interceptor.
type stubHealthHandler struct {
	specgraphv1connect.UnimplementedServerServiceHandler
}

func (h *stubHealthHandler) Health(_ context.Context, _ *connect.Request[specgraphv1.HealthRequest]) (*connect.Response[specgraphv1.HealthResponse], error) {
	return connect.NewResponse(&specgraphv1.HealthResponse{}), nil
}

// stubSpecHandler returns unimplemented for all but captures that the handler was reached.
type stubSpecHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
	called bool
}

func (h *stubSpecHandler) GetSpec(_ context.Context, _ *connect.Request[specgraphv1.GetSpecRequest]) (*connect.Response[specgraphv1.Spec], error) {
	h.called = true
	return connect.NewResponse(&specgraphv1.Spec{}), nil
}

func (h *stubSpecHandler) CreateSpec(_ context.Context, _ *connect.Request[specgraphv1.CreateSpecRequest]) (*connect.Response[specgraphv1.Spec], error) {
	h.called = true
	return connect.NewResponse(&specgraphv1.Spec{}), nil
}

// newTestServer creates an httptest.Server with Health and Spec services wired
// with the given auth config. Returns the server, a SpecService client, and
// a ServerService (Health) client.
func newTestServer(t *testing.T, authCfg config.AuthConfig) (*httptest.Server, specgraphv1connect.SpecServiceClient, specgraphv1connect.ServerServiceClient) {
	t.Helper()
	store, err := auth.NewConfigStore(authCfg)
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	interceptor := auth.NewAuthInterceptor(store)
	opts := connect.WithInterceptors(interceptor)

	mux := http.NewServeMux()
	path, handler := specgraphv1connect.NewServerServiceHandler(&stubHealthHandler{}, opts)
	mux.Handle(path, handler)
	path, handler = specgraphv1connect.NewSpecServiceHandler(&stubSpecHandler{}, opts)
	mux.Handle(path, handler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	specClient := specgraphv1connect.NewSpecServiceClient(http.DefaultClient, srv.URL)
	healthClient := specgraphv1connect.NewServerServiceClient(http.DefaultClient, srv.URL)
	return srv, specClient, healthClient
}

func withBearer(token string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(
		func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set("Authorization", "Bearer "+token)
				return next(ctx, req)
			}
		},
	))
}

func newSpecClientWithAuth(url, token string) specgraphv1connect.SpecServiceClient {
	return specgraphv1connect.NewSpecServiceClient(http.DefaultClient, url, withBearer(token))
}

func TestInterceptor_ValidAPIKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	srv, _, _ := newTestServer(t, cfg)
	client := newSpecClientWithAuth(srv.URL, "spgr_sk_abc")
	_, err := client.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInterceptor_InvalidAPIKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	srv, _, _ := newTestServer(t, cfg)
	client := newSpecClientWithAuth(srv.URL, "wrong_key")
	_, err := client.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err == nil {
		t.Fatal("expected error")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want Unauthenticated", connect.CodeOf(err))
	}
}

func TestInterceptor_NoToken_NoKeys(t *testing.T) {
	_, specClient, _ := newTestServer(t, config.AuthConfig{})
	// No auth header, no keys configured → local identity (admin)
	_, err := specClient.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err != nil {
		t.Fatalf("expected success with local identity, got: %v", err)
	}
}

func TestInterceptor_NoToken_WithKeys(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	_, specClient, _ := newTestServer(t, cfg)
	// No auth header, but keys ARE configured → Unauthenticated
	_, err := specClient.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err == nil {
		t.Fatal("expected error")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want Unauthenticated", connect.CodeOf(err))
	}
}

func TestInterceptor_InsufficientPermission(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_r", Name: "Reader", Role: "reader"},
		},
	}
	srv, _, _ := newTestServer(t, cfg)
	client := newSpecClientWithAuth(srv.URL, "spgr_sk_r")
	// Reader can GetSpec (spec:read)
	_, err := client.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err != nil {
		t.Fatalf("reader should be able to GetSpec: %v", err)
	}
	// Reader cannot CreateSpec (spec:write)
	_, err = client.CreateSpec(context.Background(), connect.NewRequest(&specgraphv1.CreateSpecRequest{}))
	if err == nil {
		t.Fatal("expected error for reader calling CreateSpec")
	}
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("code = %v, want PermissionDenied", connect.CodeOf(err))
	}
}

func TestInterceptor_ExemptRPC(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	_, _, healthClient := newTestServer(t, cfg)
	// Health is exempt — no auth header required even with keys configured
	_, err := healthClient.Health(context.Background(), connect.NewRequest(&specgraphv1.HealthRequest{}))
	if err != nil {
		t.Fatalf("Health should be exempt: %v", err)
	}
}

func TestInterceptor_JWTToken(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	srv, _, _ := newTestServer(t, cfg)
	// JWT has 3 dot-separated segments
	client := newSpecClientWithAuth(srv.URL, "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature")
	_, err := client.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err == nil {
		t.Fatal("expected error for JWT token")
	}
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("code = %v, want Unimplemented", connect.CodeOf(err))
	}
}
```

**Note:** The "unknown procedure" test case cannot be tested through the
ConnectRPC client (it only calls known procedures). That scenario is covered
by the runtime safety net + the permission table completeness test. If a
procedure is registered but not in the permission table, the completeness
test catches it.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test ./internal/auth/ -run TestInterceptor -v`
Expected: FAIL — `NewAuthInterceptor` not defined

- [ ] **Step 3: Write interceptor implementation**

```go
// internal/auth/interceptor.go
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/user"
	"strings"

	"connectrpc.com/connect"
)

// NewAuthInterceptor returns a ConnectRPC unary interceptor that enforces
// authentication and authorization on all non-exempt RPCs.
func NewAuthInterceptor(store IdentityStore) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure

			// 1. Exempt RPCs pass through with no identity.
			if IsExempt(procedure) {
				return next(ctx, req)
			}

			// 2. Resolve identity from Authorization header.
			id, authErr := resolveIdentity(ctx, store, req.Header())
			if authErr != nil {
				slog.Warn("auth: authentication failed",
					"procedure", procedure,
					"error", authErr.Error(),
				)
				return nil, authErr
			}

			// 3. Check permission for this RPC.
			required, ok := RPCPermission(procedure)
			if !ok {
				slog.Error("auth: unconfigured RPC permission",
					"procedure", procedure,
				)
				return nil, connect.NewError(connect.CodeInternal, nil)
			}

			if !HasPermission(id.Permissions, required) {
				slog.Warn("auth: permission denied",
					"subject", id.Subject,
					"procedure", procedure,
					"required", required,
				)
				return nil, connect.NewError(connect.CodePermissionDenied, nil)
			}

			// 4. Inject identity into context and proceed.
			slog.Info("auth: authenticated",
				"subject", id.Subject,
				"procedure", procedure,
			)
			return next(WithIdentity(ctx, id), req)
		}
	}
}

// resolveIdentity extracts and validates the identity from request headers.
func resolveIdentity(ctx context.Context, store IdentityStore, headers http.Header) (*Identity, error) {
	authHeader := headers.Get("Authorization")

	if authHeader == "" {
		// No token: fall back to local identity if no keys configured.
		if !store.HasKeys() {
			return localIdentity(), nil
		}
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Parse "Bearer <token>"
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		// No "Bearer " prefix
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Try API key resolution first — keys are opaque strings that may contain dots.
	id, err := store.ResolveAPIKey(ctx, token)
	if err == nil {
		return id, nil
	}
	// Unknown key: check if it looks like a JWT (future OIDC support).
	if errors.Is(err, ErrUnknownKey) && strings.Count(token, ".") == 2 {
		return nil, connect.NewError(connect.CodeUnimplemented, nil)
	}
	if errors.Is(err, ErrUnknownKey) {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	// Non-ErrUnknownKey failures (I/O, store outage) are internal errors.
	return nil, connect.NewError(connect.CodeInternal, nil)
}

// localIdentity creates the implicit admin identity from the OS user.
func localIdentity() *Identity {
	username := "unknown"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	return &Identity{
		Subject:     "local:" + username,
		DisplayName: username,
		Role:        RoleAdmin,
		Permissions: map[string]bool{"*:*": true},
		Source:      "local",
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test ./internal/auth/ -run TestInterceptor -v`
Expected: PASS (all 7 tests)

- [ ] **Step 5: Commit**

```text
feat(auth): add ConnectRPC auth interceptor with full flow
```

---

## Chunk 4: Server Wiring

### Task 8: Add HandlerOption to Register Functions

**Files:**

- Modify: `internal/server/server.go`
- Modify: `internal/server/health_handler.go`
- Modify: `internal/server/decision_handler.go`
- Modify: `internal/server/graph_handler.go`
- Modify: `internal/server/claim_handler.go`
- Modify: `internal/server/constitution_handler.go`
- Modify: `internal/server/authoring_handler.go`
- Modify: `internal/server/execution_handler.go`
- Modify: `internal/server/lifecycle_handler.go`
- Modify: `internal/server/sync_handler.go`

Each `Register*Service` function needs to accept `...connect.HandlerOption` and pass it to the generated `New*ServiceHandler` call.

- [ ] **Step 1: Update NewMux in server.go**

Change signature to accept and propagate handler options:

```go
func NewMux(scoper storage.Scoper, opts ...connect.HandlerOption) *http.ServeMux {
	mux := http.NewServeMux()
	specHandler := NewSpecHandler(scoper)
	path, handler := specgraphv1connect.NewSpecServiceHandler(specHandler, opts...)
	mux.Handle(path, handler)
	return mux
}
```

Add `"connectrpc.com/connect"` to imports.

- [ ] **Step 2: Update RegisterHealthService**

```go
func RegisterHealthService(mux *http.ServeMux, opts ...connect.HandlerOption) {
	path, handler := specgraphv1connect.NewServerServiceHandler(&HealthHandler{}, opts...)
	mux.Handle(path, handler)
}
```

- [ ] **Step 3: Update all other Register functions**

Apply the same pattern to each:

- `RegisterDecisionService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption)`
- `RegisterGraphService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption)`
- `RegisterClaimService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption)`
- `RegisterConstitutionService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption)`
- `RegisterAuthoringService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption)`
- `RegisterExecutionService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption)`
- `RegisterLifecycleService(mux *http.ServeMux, scoper storage.Scoper, dc DriftChecker, l SpecLinter, logger *slog.Logger, opts ...connect.HandlerOption)`
- `RegisterSyncService(mux *http.ServeMux, scoper storage.Scoper, allowedOutputRoot string, opts ...connect.HandlerOption) *SyncHandler`

In each, pass `opts...` to the corresponding `specgraphv1connect.New*ServiceHandler()` call.

- [ ] **Step 4: Verify build**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go build ./...`
Expected: PASS — the variadic `...connect.HandlerOption` is backward compatible (callers not passing opts still compile)

- [ ] **Step 5: Run existing tests to verify no regressions**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test ./internal/server/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```text
refactor(server): accept connect.HandlerOption in all Register functions
```

### Task 9: Wire Interceptor in serve.go

**Files:**

- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Create interceptor from config and pass as handler option**

After `store` is created (around line 78), add:

```go
authStore, err := auth.NewConfigStore(cfg.Auth)
if err != nil {
	return fmt.Errorf("auth config: %w", err)
}
interceptor := auth.NewAuthInterceptor(authStore)
opts := connect.WithInterceptors(interceptor)
```

Then pass `opts` to every `NewMux` and `Register*Service` call:

```go
mux := server.NewMux(store, opts)
server.RegisterHealthService(mux, opts)
server.RegisterDecisionService(mux, store, opts)
server.RegisterGraphService(mux, store, opts)
server.RegisterClaimService(mux, store, opts)
server.RegisterConstitutionService(mux, store, opts)
server.RegisterAuthoringService(mux, store, opts)
server.RegisterExecutionService(mux, store, opts)
// ...
server.RegisterLifecycleService(mux, store, driftEngine, lintEngine, nil, opts)
syncHandler := server.RegisterSyncService(mux, store, "", opts)
```

Add imports: `"github.com/seanb4t/specgraph/internal/auth"`, `"connectrpc.com/connect"`.

- [ ] **Step 2: Verify build**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go build ./cmd/specgraph/`
Expected: PASS

- [ ] **Step 3: Commit**

```text
feat(server): wire auth interceptor into server startup
```

### Task 10: Wire Interceptor in E2E Test Server

**Files:**

- Modify: `e2e/testutil/server.go`

- [ ] **Step 1: Update StartServer to accept and propagate handler options**

The E2E test server needs to optionally receive `connect.HandlerOption` so tests can configure auth. Update `StartServer` (or add a parameter) to pass opts through to all `Register*Service` calls, same pattern as `serve.go`.

For tests that don't need auth (existing tests), pass no opts (empty variadic). For auth-specific tests, pass the interceptor.

- [ ] **Step 2: Verify E2E tests still pass without auth**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test -tags e2e ./e2e/api/ -v --ginkgo.label-filter='!auth' -count=1`
Expected: PASS (existing tests unaffected — no opts passed = no interceptor)

- [ ] **Step 3: Commit**

```text
refactor(e2e): propagate handler options through test server setup
```

---

## Chunk 5: E2E Auth Tests

### Task 11: E2E Auth Test Scenarios

**Files:**

- Create: `e2e/api/auth_test.go`

- [ ] **Step 1: Write E2E auth tests**

Create Ginkgo test file with `//go:build e2e` tag and `auth` label:

```go
// e2e/api/auth_test.go
//go:build e2e

package api_test

// Tests covering:
// - No keys configured → all requests succeed (local identity)
// - Keys configured → no Bearer → Unauthenticated on non-Health RPCs
// - Keys configured → valid reader key → can GetSpec, cannot CreateSpec
// - Keys configured → valid admin key → can CreateSpec
// - Keys configured → custom role with specific permissions
// - Health always responds regardless of auth config
```

Each scenario starts a fresh server with a specific auth config (using the E2E test helpers).

**Key approach:** The E2E tests need separate server instances with different auth configs. Use `BeforeEach` to start a server with the config for that `Describe` block, or use table-driven Ginkgo `Entry` patterns.

- [ ] **Step 2: Run E2E auth tests**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && go test -tags e2e ./e2e/api/ -v --ginkgo.label-filter='auth' -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```text
test(e2e): add auth interceptor E2E test scenarios
```

### Task 12: Run Full Quality Gate

- [ ] **Step 1: Run task check**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && task check`
Expected: PASS (fmt, license, lint, build, unit tests)

- [ ] **Step 2: Run task pr-prep**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && task pr-prep`
Expected: PASS (check + integration + e2e)

- [ ] **Step 3: Add license headers to new files if missing**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && task license:add`

- [ ] **Step 4: Fix any lint issues**

Run: `cd /Volumes/Code/github.com/seanb4t/specgraph && task lint`
Fix any issues found.

- [ ] **Step 5: Final commit if any fixes needed**

```text
chore: fix lint and license headers for auth package
```

### Task 13: Close Beads

- [ ] **Step 1: Close resolved beads**

```bash
bd close spgr-dpi spgr-ous spgr-4r8 --reason="Resolved by auth interceptor (PR #XX)"
```

- [ ] **Step 2: Update OIDC bead with progress note**

```bash
bd update spgr-0az --notes="JWT detection hook and IdentityStore interface in place. Ready for OIDC provider implementation."
```
