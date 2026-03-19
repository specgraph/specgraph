# Authentication & Authorization Design

**Date:** 2026-03-18
**Epic:** spgr-5wb (Authentication & Authorization)
**Status:** Approved

## Overview

Add authentication and authorization to SpecGraph via a single ConnectRPC
interceptor. Local-first design: when no API keys are configured, the system
creates an implicit admin identity from the OS user. Adding any API key
immediately enforces auth on all RPCs except Health.

## Goals

- Single enforcement point for all RPC auth (interceptor, not per-handler)
- Zero-config local dev experience (implicit OS-user admin identity)
- API keys as the first auth primitive, transmitted via `Authorization: Bearer`
- JWT/OIDC-ready: interceptor detects JWT-shaped tokens, rejects with
  `Unimplemented` until OIDC support is added
- Fine-grained permissions (`service:action`) with roles as convenience bundles
- Default-deny: unknown RPCs without permission mappings are rejected

## Non-Goals

- OIDC provider integration (Entra ID, GitHub) — future work (spgr-0az)
- Memgraph connection auth/TLS — infrastructure concern (spgr-fn3)
- BeadsAdapter auth semantics — depends on bd/dolt (spgr-sau)
- Per-project authorization scoping — designed for but not implemented
- Dynamic API key management (create/revoke at runtime)

## Identity Model

### Identity Type

```go
type Identity struct {
    Subject     string          // "local:<os-user>" | "apikey:<key-id>" | future "oidc:<sub>"
    DisplayName string          // human-friendly name
    Role        Role            // role name (built-in or custom)
    Permissions map[string]bool // resolved: "spec:write" → true
    Source      string          // "local" | "apikey" | future "oidc"
}
```

The `Subject` field uses a `source:identifier` namespace to prevent collisions
between local users, API keys, and future OIDC subjects.

### Roles

Three built-in roles. Custom roles can be defined in config.

| Role | Permissions |
|------|------------|
| `reader` | `*:read` |
| `writer` | `*:read`, `*:write` |
| `admin` | `*:*` (wildcard — matches any permission) |

The `writer` role explicitly excludes `delete`. `RemoveEdge` is the only RPC
that currently requires a `delete` permission. `UnclaimSpec` remains a `write`
operation because claims are transient leases, not persistent data removal.
State transitions (Abandon, Supersede, Amend) are also `write` operations.

## Permission Model

### Actions

Two common permission actions per service: `read` and `write`. A third action
`delete` exists only where data is physically removed (not state transitions).

### Wildcard Resolution

Role permission bundles support wildcards. Permission checking algorithm
(evaluated at request time, not load time):

```text
func hasPermission(perms map[string]bool, required string) bool:
    if perms["*:*"]           → true   // full wildcard
    if perms[required]        → true   // exact match
    service, action = split(required, ":")
    if perms[service+":*"]   → true   // service wildcard (e.g., "spec:*")
    if perms["*:"+action]    → true   // action wildcard (e.g., "*:read")
    return false
```

This avoids expanding wildcards into the map (which would require knowing all
permissions up front). The `Permissions` map stores the raw entries from the
role definition; the check function handles wildcard matching.

Custom roles MAY use wildcards in their permission lists.

### RPC → Permission Mapping (in code)

Static table mapping each ConnectRPC procedure to its required permission.
Defined in code because the mapping is a fact about the API, not a deployment
policy. A missing entry is a bug caught by tests and rejected at runtime.

```text
# SpecService
/specgraph.v1.SpecService/GetSpec         → spec:read
/specgraph.v1.SpecService/ListSpecs       → spec:read
/specgraph.v1.SpecService/CreateSpec      → spec:write
/specgraph.v1.SpecService/UpdateSpec      → spec:write

# DecisionService
/specgraph.v1.DecisionService/GetDecision      → decision:read
/specgraph.v1.DecisionService/ListDecisions    → decision:read
/specgraph.v1.DecisionService/CreateDecision   → decision:write
/specgraph.v1.DecisionService/UpdateDecision   → decision:write

# GraphService
/specgraph.v1.GraphService/GetDependencies     → graph:read
/specgraph.v1.GraphService/GetTransitiveDeps   → graph:read
/specgraph.v1.GraphService/GetImpact           → graph:read
/specgraph.v1.GraphService/GetReady            → graph:read
/specgraph.v1.GraphService/GetCriticalPath     → graph:read
/specgraph.v1.GraphService/ListEdges           → graph:read
/specgraph.v1.GraphService/AddEdge             → graph:write
/specgraph.v1.GraphService/RemoveEdge          → graph:delete

# ClaimService
/specgraph.v1.ClaimService/ClaimSpec           → claim:write
/specgraph.v1.ClaimService/Heartbeat           → claim:write
/specgraph.v1.ClaimService/UnclaimSpec         → claim:write

# ConstitutionService
/specgraph.v1.ConstitutionService/GetConstitution    → constitution:read
/specgraph.v1.ConstitutionService/CheckViolation     → constitution:read
/specgraph.v1.ConstitutionService/UpdateConstitution → constitution:write
/specgraph.v1.ConstitutionService/EmitToolFiles      → constitution:write

# AuthoringService
/specgraph.v1.AuthoringService/GetPrompts    → authoring:read
/specgraph.v1.AuthoringService/Spark         → authoring:write
/specgraph.v1.AuthoringService/Shape         → authoring:write
/specgraph.v1.AuthoringService/Specify       → authoring:write
/specgraph.v1.AuthoringService/Decompose     → authoring:write
/specgraph.v1.AuthoringService/Approve       → authoring:write
/specgraph.v1.AuthoringService/Amend         → authoring:write
/specgraph.v1.AuthoringService/Supersede     → authoring:write

# ExecutionService
/specgraph.v1.ExecutionService/GenerateBundle     → execution:read
/specgraph.v1.ExecutionService/GetPrime           → execution:read
/specgraph.v1.ExecutionService/GetExecutionEvents → execution:read
/specgraph.v1.ExecutionService/ReportProgress     → execution:write
/specgraph.v1.ExecutionService/ReportBlocker      → execution:write
/specgraph.v1.ExecutionService/ReportCompletion   → execution:write

# LifecycleService
/specgraph.v1.LifecycleService/CheckDrift          → lifecycle:read
/specgraph.v1.LifecycleService/Lint                → lifecycle:read
/specgraph.v1.LifecycleService/AcknowledgeDrift    → lifecycle:write
/specgraph.v1.LifecycleService/TransitionAmend     → lifecycle:write
/specgraph.v1.LifecycleService/TransitionSupersede → lifecycle:write
/specgraph.v1.LifecycleService/TransitionAbandon   → lifecycle:write

# SyncService
/specgraph.v1.SyncService/GetSyncStatus   → sync:read
/specgraph.v1.SyncService/SyncBeads       → sync:write
/specgraph.v1.SyncService/SyncGitHub      → sync:write
/specgraph.v1.SyncService/Inject          → sync:write
```

Note: Lifecycle transitions (Amend, Supersede, Abandon) are `write` operations
because they change spec state — they do not remove data. Claims (including
UnclaimSpec) are also `write` — a claim is a transient lease, not persistent
data. The `delete` action is reserved for actual data removal (RemoveEdge).

### Role → Permission Bundles (in config)

Defined in the config file as operational policy. Operators can create custom
roles or override defaults per deployment.

```yaml
auth:
  roles:
    ci-readonly:
      permissions: ["spec:read", "decision:read", "graph:read"]
    deployer:
      permissions: ["spec:read", "spec:write", "sync:write"]
```

When the `roles` section is omitted, the three built-in defaults are used.
Custom roles are **additive** — they extend the built-in set. The three
built-in roles (`reader`, `writer`, `admin`) are always available. If an
operator defines a custom role with the same name as a built-in, the custom
definition wins (explicit override).

### Exempt RPCs

```text
/specgraph.v1.ServerService/Health — always unauthenticated
```

No other RPCs are exempt. OTel is a separate endpoint outside ConnectRPC.

## IdentityStore Interface

```go
type IdentityStore interface {
    ResolveAPIKey(ctx context.Context, key string) (*Identity, error)
    HasKeys() bool
}
```

Initial implementation: config-file backed. The interface allows future
implementations backed by a database, OIDC provider, or external identity
service.

## Auth Interceptor Flow

```text
1. Exempt RPC? → pass through (no identity on context)
2. Extract Bearer token from Authorization header
3. Token present?
   a. store.ResolveAPIKey(token)
      - Found → identity from store
      - Unknown + JWT-shaped (3 dot-separated segments) → Unimplemented
      - Unknown + non-JWT → Unauthenticated
4. No token?
   a. store.HasKeys() == false → implicit local identity (OS user, admin)
   b. store.HasKeys() == true → Unauthenticated
5. Check permission: rpcPermissions[procedure] against identity.Permissions
   - Allowed → inject identity into context, call handler
   - Denied → PermissionDenied
   - Unknown procedure → Internal ("unconfigured RPC permission")
```

## Configuration

### Config Schema

```go
type AuthConfig struct {
    APIKeys []APIKeyConfig        `yaml:"api_keys"`
    Roles   map[string]RoleConfig `yaml:"roles"`
}

type APIKeyConfig struct {
    ID   string `yaml:"id"`
    Key  string `yaml:"key"`
    Name string `yaml:"name"`
    Role string `yaml:"role"`
}

type RoleConfig struct {
    Permissions []string `yaml:"permissions"`
}
```

Added to the existing `Config` struct as `Auth AuthConfig`.

### Example Config

```yaml
auth:
  api_keys:
    - id: "key-1"
      key: "<example-api-key-1>"
      name: "CI Pipeline"
      role: writer
    - id: "key-2"
      key: "<example-api-key-2>"
      name: "Sean's dev key"
      role: admin
  roles:
    # Custom roles (optional — built-in reader/writer/admin used if omitted)
    ci-readonly:
      permissions: ["spec:read", "decision:read", "graph:read"]
```

### Local Identity Fallback

When no API keys are configured (`auth.api_keys` absent or empty):

- `Subject`: `"local:<os/user.Current().Username>"`
- `DisplayName`: OS username
- `Role`: `admin`
- `Source`: `"local"`

This is not a "mode" — it is the natural behavior of an empty config. The
moment any API key is added, unauthenticated requests begin failing.

### Known Limitations

- **Plaintext API keys in config:** v1 stores keys as plaintext in the YAML
  config file. Acceptable for local-first tooling. Future iterations may add
  hashed key comparison or environment variable substitution
  (`key: ${SPECGRAPH_API_KEY_1}`).
- **ConfigStore is immutable after construction:** No hot-reload of auth config.
  Changing keys or roles requires a server restart. This avoids TOCTOU races
  between `HasKeys()` and key resolution. Hot-reload is out of scope.
- **Constant-time key comparison:** `ResolveAPIKey` MUST use `subtle.ConstantTimeCompare`
  for key matching, even for plaintext v1. This prevents timing side-channels
  and is the right habit before hashed keys are added.
- **Duplicate key values are a startup error:** Two API keys with the same secret
  string but different IDs/roles would be ambiguous. Both duplicate IDs and
  duplicate key values are rejected at config load time.

## Server Wiring

The auth interceptor is passed to each service handler registration via
ConnectRPC handler options. The current `Register*Service()` functions need to
accept `connect.HandlerOption` varargs and propagate them to handler
constructors.

```go
authCfg := config.Auth
store := auth.NewConfigStore(authCfg)
interceptor := auth.NewAuthInterceptor(store)
opts := connect.WithInterceptors(interceptor)

mux := server.NewMux(scoper, opts)
// Register*Service calls pass opts to New*ServiceHandler
```

### Middleware Ordering

```text
HTTP Request
  → ProjectMiddleware (HTTP level: extracts X-Specgraph-Project, sets ctx)
  → ConnectRPC dispatch
    → Auth Interceptor (ConnectRPC level: validates token, checks permission)
    → Handler (has both project and identity on context)
```

ProjectMiddleware remains at the HTTP level. The auth interceptor runs inside
ConnectRPC dispatch where it has access to the procedure name for permission
mapping.

### Audit Logging

The interceptor logs auth decisions at structured log levels:

- `slog.Info`: successful authentication (subject, procedure)
- `slog.Warn`: authentication failure (reason, procedure, remote addr)
- `slog.Warn`: permission denied (subject, procedure, required permission)

## Testing Strategy

### Unit Tests (internal/auth/)

**Interceptor tests** with fake IdentityStore:

- Valid API key → identity on context, handler called
- Invalid API key → Unauthenticated, handler not called
- No token + no keys configured → local identity (admin)
- No token + keys configured → Unauthenticated
- Valid key, insufficient permission → PermissionDenied
- Exempt RPC (Health) → passes without auth
- Unknown RPC not in permission table → Internal error
- JWT-shaped token → Unimplemented

**Permission table completeness test:**
Iterates all registered ConnectRPC procedures, asserts each is either in
`rpcPermissions` or `exemptProcedures`. Catches missing mappings at test time.

**Role resolution tests:**

- Built-in roles expand correctly
- Custom roles from config load correctly
- `*:*` wildcard matches any permission
- API key referencing unknown role → startup error

**Config store tests:**

- Valid config → correct identities
- Empty config → HasKeys() false
- Duplicate key IDs → startup error
- Duplicate key values (same secret, different IDs) → startup error
- API key uses constant-time comparison

### E2E Tests (e2e/api/)

Add auth scenarios to existing Ginkgo suite:

- Keys configured: no Bearer → Unauthenticated on all RPCs except Health
- Keys configured: reader key can GetSpec, cannot CreateSpec
- Keys configured: custom role with specific permissions works correctly
- No keys configured: all requests succeed (local identity)
- Health always responds regardless of auth config

### Not Tested

- OIDC flows (not implemented)
- Per-project authorization (not implemented)
- Dynamic key management (config-file store is static)

## Epic Impact

### Beads Resolved

| Bead | Title | Coverage |
|------|-------|----------|
| spgr-dpi (P1) | LifecycleService auth middleware | Interceptor covers all LifecycleService RPCs |
| spgr-ous (P2) | Destructive ops auth middleware | Same interceptor; TransitionAbandon/TransitionSupersede → lifecycle:write |
| spgr-4r8 (P2) | AuthoringService auth | Same interceptor; all authoring RPCs get permissions |

### Partially Advanced

| Bead | Title | Status |
|------|-------|--------|
| spgr-0az (P2) | OIDC providers | JWT detection hook in place; IdentityStore interface ready for OIDC impl |

### Correctly Deferred

| Bead | Title | Reason |
|------|-------|--------|
| spgr-fn3 (P4) | Memgraph auth + TLS | Infrastructure concern, independent of RPC auth |
| spgr-sau (P3) | BeadsAdapter auth | Depends on bd/dolt auth semantics, not SpecGraph auth |

## Future Evolution

1. **OIDC support:** Add `OIDCIdentityStore` implementing `IdentityStore`.
   Interceptor's JWT detection branch calls it instead of rejecting. No
   interceptor changes needed.
2. **Per-project authorization:** Identity resolution function receives project
   context (already available on ctx from ProjectMiddleware). Role/permission
   lookup becomes project-scoped.
3. **Per-service roles (C-level granularity):** Operators define custom roles in
   config with specific `service:action` permissions. The interceptor already
   checks at this granularity — only the role definitions change.
4. **Dynamic key management:** Replace `ConfigStore` with a database-backed
   `IdentityStore`. Interface unchanged.

## Package Layout

```text
internal/auth/
├── auth.go          # Identity type, Role constants, permission helpers
├── context.go       # IdentityFromContext / WithIdentity
├── interceptor.go   # NewAuthInterceptor, permission table, exempt set
├── permissions.go   # rpcPermissions table, role default bundles
├── store.go         # IdentityStore interface
└── config_store.go  # Config-file IdentityStore implementation
```
