# OIDC Authentication Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add OIDC authentication support with multi-provider JWT validation, claims-to-role mapping, credential bootstrap, and explicit auth mode configuration.

**Architecture:** Extend `IdentityStore` interface with `ResolveJWT` and `HasAuth`. Build `OIDCStore` (per-provider JWKS validation via `go-oidc/v3`) and `CompositeStore` (routes API keys vs JWTs, enforces auth mode). Add credential file bootstrap for local mode. All routing logic is internal to `CompositeStore` — interceptor and middleware stay simple.

**Tech Stack:** Go, `github.com/coreos/go-oidc/v3`, ConnectRPC interceptors, `httptest` for mock IdP in tests

**Spec:** `docs/superpowers/specs/2026-03-28-oidc-authentication-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/config/global.go` | Add `Mode`, `DefaultRole`, `OIDCProviders` to `AuthConfig`. New `OIDCProviderConfig` and `ClaimMapping` structs. |
| `internal/auth/store.go` | Extend `IdentityStore` interface: add `ResolveJWT`, `HasAuth`. New sentinels `ErrNoOIDC`, `ErrUnknownIssuer`, `ErrInvalidToken`. |
| `internal/auth/config_store.go` | Add no-op `ResolveJWT`, delegate `HasAuth`→`HasKeys`. Signature change: accept `credentialsPath`. Load and merge credential file keys. |
| `internal/auth/oidc_store.go` | New. Per-provider OIDC verifier — JWKS fetch, token verification, claims mapping → `*Identity`. |
| `internal/auth/composite_store.go` | New. Wraps `ConfigStore` + `[]OIDCStore` + mode. Routes tokens, enforces mode gates. |
| `internal/auth/bootstrap.go` | New. Generate default admin API key, write `credentials.yaml` with `0600`. |
| `internal/auth/interceptor.go` | Replace JWT stub. No-header branch: `!store.HasAuth()`→`localIdentity()`, else `CodeUnauthenticated`. |
| `internal/auth/middleware.go` | No-header branch: `!store.HasAuth()`→`localIdentity()`, else deny. |
| `cmd/specgraph/serve.go` | Wire `CompositeStore`, pass credentials path, `HasKeys`→`HasAuth` in warning. |
| `internal/auth/oidc_store_test.go` | New. Mock OIDC provider, token verification, claims mapping tests. |
| `internal/auth/composite_store_test.go` | New. Composite routing, mode enforcement tests. |
| `internal/auth/bootstrap_test.go` | New. Key generation, idempotency, file permissions tests. |

---

## Chunk 1: Config Schema + Interface Extension + ConfigStore Updates

### Task 1: Add OIDC config types to `internal/config/global.go`

**Files:**

- Modify: `internal/config/global.go:50-67`
- Test: `internal/config/global_test.go` (extend)

- [ ] **Step 1: Write failing test for new config fields**

In `internal/config/global_test.go`, add:

```go
func TestLoadGlobal_AuthConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
auth:
  mode: oidc
  default_role: writer
  api_keys:
    - id: k1
      key: spgr_sk_test
      name: Test
      role: admin
  oidc_providers:
    - id: entra
      issuer: https://login.microsoftonline.com/tenant/v2.0
      client_id: app-id
      audience: api-audience
      claims_mapping:
        - claim: groups
          value: specgraph-admins
          role: admin
  roles:
    deployer:
      permissions: ["spec:read", "execution:*"]
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, "oidc", cfg.Auth.Mode)
	assert.Equal(t, "writer", cfg.Auth.DefaultRole)
	require.Len(t, cfg.Auth.OIDCProviders, 1)
	assert.Equal(t, "entra", cfg.Auth.OIDCProviders[0].ID)
	assert.Equal(t, "https://login.microsoftonline.com/tenant/v2.0", cfg.Auth.OIDCProviders[0].Issuer)
	assert.Equal(t, "app-id", cfg.Auth.OIDCProviders[0].ClientID)
	assert.Equal(t, "api-audience", cfg.Auth.OIDCProviders[0].Audience)
	require.Len(t, cfg.Auth.OIDCProviders[0].ClaimsMapping, 1)
	assert.Equal(t, "groups", cfg.Auth.OIDCProviders[0].ClaimsMapping[0].Claim)
	assert.Equal(t, "specgraph-admins", cfg.Auth.OIDCProviders[0].ClaimsMapping[0].Value)
	assert.Equal(t, "admin", cfg.Auth.OIDCProviders[0].ClaimsMapping[0].Role)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadGlobal_AuthConfig -v -count=1`
Expected: FAIL — `OIDCProviders` field doesn't exist on `AuthConfig`

- [ ] **Step 3: Add config types**

In `internal/config/global.go`, replace the `AuthConfig` struct and add new types after `RoleConfig`:

```go
// AuthConfig configures authentication and authorization.
type AuthConfig struct {
	Mode          string                `yaml:"mode"`
	DefaultRole   string                `yaml:"default_role"`
	APIKeys       []APIKeyConfig        `yaml:"api_keys"`
	OIDCProviders []OIDCProviderConfig  `yaml:"oidc_providers"`
	Roles         map[string]RoleConfig `yaml:"roles"`
}

// OIDCProviderConfig defines a single OIDC identity provider.
type OIDCProviderConfig struct {
	ID            string         `yaml:"id"`
	Issuer        string         `yaml:"issuer"`
	ClientID      string         `yaml:"client_id"`
	Audience      string         `yaml:"audience"`
	ClaimsMapping []ClaimMapping `yaml:"claims_mapping"`
}

// ClaimMapping maps a JWT claim value to a SpecGraph role.
type ClaimMapping struct {
	Claim string `yaml:"claim"`
	Value string `yaml:"value"`
	Role  string `yaml:"role"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestLoadGlobal_AuthConfig -v -count=1`
Expected: PASS

- [ ] **Step 5: Run full config test suite**

Run: `go test ./internal/config/ -v -count=1`
Expected: All tests PASS (existing tests unaffected — new fields have zero values by default)

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(config): add OIDC provider and auth mode config types (spgr-0az)"
jj --no-pager new -m ""
```

---

### Task 2: Extend `IdentityStore` interface and update `ConfigStore`

**Files:**

- Modify: `internal/auth/store.go:11-23`
- Modify: `internal/auth/config_store.go:30-112`
- Modify: `internal/auth/config_store_test.go`
- Modify: `internal/auth/interceptor_test.go`
- Modify: `internal/auth/middleware_test.go`
- Modify: `internal/server/api_handler_test.go`
- Modify: `e2e/api/auth_test.go`

- [ ] **Step 1: Write failing tests for new interface methods on ConfigStore**

Add to `internal/auth/config_store_test.go`:

```go
func TestConfigStore_ResolveJWT_ReturnsErrNoOIDC(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	_, err = store.ResolveJWT(context.Background(), "header.payload.signature")
	if !errors.Is(err, auth.ErrNoOIDC) {
		t.Errorf("ResolveJWT error = %v, want ErrNoOIDC", err)
	}
}

func TestConfigStore_HasAuth_NoKeys(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if store.HasAuth() {
		t.Error("HasAuth() = true, want false with no keys")
	}
}

func TestConfigStore_HasAuth_WithKeys(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if !store.HasAuth() {
		t.Error("HasAuth() = false, want true with keys")
	}
}

func TestConfigStore_AllowUnauthenticated_NoKeys(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if !store.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = false, want true with no keys")
	}
}

func TestConfigStore_AllowUnauthenticated_WithKeys(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if store.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = true, want false with keys")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run "TestConfigStore_ResolveJWT|TestConfigStore_HasAuth|TestConfigStore_AllowUnauthenticated" -v -count=1`
Expected: FAIL — methods/sentinels don't exist, `NewConfigStore` signature wrong

- [ ] **Step 3: Update `store.go` — extend interface and add sentinels**

Replace `internal/auth/store.go` contents:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
)

// ErrUnknownKey is returned when an API key is not recognized.
var ErrUnknownKey = errors.New("unknown API key")

// ErrNoOIDC is returned by stores that don't support OIDC token resolution.
var ErrNoOIDC = errors.New("OIDC not configured")

// ErrUnknownIssuer is returned when a JWT's issuer doesn't match any configured provider.
var ErrUnknownIssuer = errors.New("unknown token issuer")

// IdentityStore resolves authentication tokens to identities.
type IdentityStore interface {
	// ResolveAPIKey returns the identity for the given API key.
	// Returns ErrUnknownKey if the key is not recognized.
	ResolveAPIKey(ctx context.Context, key string) (*Identity, error)

	// ResolveJWT validates a JWT and returns the identity.
	// Returns ErrNoOIDC if the store doesn't support OIDC.
	// Returns ErrUnknownIssuer if the token's issuer doesn't match any provider.
	ResolveJWT(ctx context.Context, token string) (*Identity, error)

	// HasAuth reports whether any authentication is configured (keys or OIDC providers).
	HasAuth() bool

	// AllowUnauthenticated reports whether unauthenticated requests should
	// fall back to the local identity. True when mode is "mixed", or mode
	// is "local" with no keys configured.
	AllowUnauthenticated() bool
}
```

- [ ] **Step 4: Update `config_store.go` — new signature, implement new methods**

Change `NewConfigStore` signature and add new methods. The `credentialsPath` parameter is accepted but credential loading is deferred to Task 7 (bootstrap). For now, pass `""` in all callers.

In `internal/auth/config_store.go`, update the function signature:

```go
func NewConfigStore(cfg config.AuthConfig, credentialsPath string) (*ConfigStore, error) {
```

Add these methods after `HasKeys`:

```go
// ResolveJWT is a no-op for ConfigStore — it doesn't support OIDC.
func (s *ConfigStore) ResolveJWT(_ context.Context, _ string) (*Identity, error) {
	return nil, ErrNoOIDC
}

// HasAuth reports whether any API keys are configured.
func (s *ConfigStore) HasAuth() bool {
	return s.hasKeys
}

// AllowUnauthenticated returns true when no API keys are configured,
// preserving the existing local-identity fallback behavior.
func (s *ConfigStore) AllowUnauthenticated() bool {
	return !s.hasKeys
}
```

- [ ] **Step 5: Update all `NewConfigStore` callers to pass `""`**

All callers need the second argument added. These are mechanical changes — add `, ""` to each call:

- `internal/auth/config_store_test.go`: 13 call sites
- `internal/auth/interceptor_test.go:42`
- `internal/auth/middleware_test.go`: 5 call sites
- `internal/server/api_handler_test.go`: 3 call sites
- `cmd/specgraph/serve.go:93`
- `e2e/api/auth_test.go`: 6 call sites

Example pattern — every `auth.NewConfigStore(cfg)` becomes `auth.NewConfigStore(cfg, "")` and every `auth.NewConfigStore(config.AuthConfig{})` becomes `auth.NewConfigStore(config.AuthConfig{}, "")`.

**Important:** The `HasKeys()` method remains on `*ConfigStore` as a concrete method (not removed from the struct, only from the interface). It's still used by `HasAuth()` and `AllowUnauthenticated()` internally.

- [ ] **Step 5b: Verify e2e build**

Run: `go vet -tags e2e ./e2e/...`
Expected: Clean (catches e2e callers of NewConfigStore)

- [ ] **Step 6: Verify build compiles**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 7: Run all auth tests**

Run: `go test ./internal/auth/ -v -count=1`
Expected: All tests PASS (old + new)

- [ ] **Step 8: Run server tests**

Run: `go test ./internal/server/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 9: Commit**

```bash
jj --no-pager describe -m "feat(auth): extend IdentityStore interface with ResolveJWT, HasAuth, AllowUnauthenticated (spgr-0az)"
jj --no-pager new -m ""
```

---

### Task 3: Update interceptor and middleware for new interface

**Files:**

- Modify: `internal/auth/interceptor.go:64-95`
- Modify: `internal/auth/middleware.go:30-50`

- [ ] **Step 1: Update existing JWT test**

The existing `TestInterceptor_JWTToken` at `internal/auth/interceptor_test.go:169` expects `CodeUnimplemented`. Update it to expect `CodeUnauthenticated`:

In `internal/auth/interceptor_test.go`, change line 181-182:

```go
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want Unauthenticated", connect.CodeOf(err))
	}
```

Also update `newTestServer` (line 42) to pass the new `""` credentials path arg:

```go
	store, err := auth.NewConfigStore(authCfg, "")
```

- [ ] **Step 2: Update `interceptor.go` — replace JWT stub, update no-header branch**

In `internal/auth/interceptor.go`, update `resolveIdentity`:

```go
func resolveIdentity(ctx context.Context, store IdentityStore, headers http.Header) (*Identity, error) {
	authHeader := headers.Get("Authorization")

	if authHeader == "" {
		if store.AllowUnauthenticated() {
			return localIdentity(), nil
		}
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Parse "Bearer <token>" — scheme is case-insensitive per RFC 7235.
	scheme, token, ok := strings.Cut(authHeader, " ")
	token = strings.TrimSpace(token)
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// ResolveAPIKey handles all token routing:
	// - API keys matched directly by ConfigStore
	// - JWT-shaped tokens delegated to OIDCStore via CompositeStore
	id, err := store.ResolveAPIKey(ctx, token)
	if err == nil {
		return id, nil
	}
	if errors.Is(err, ErrUnknownKey) {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	// Non-ErrUnknownKey failures (I/O, store outage) are internal errors.
	return nil, connect.NewError(connect.CodeInternal, nil)
}
```

- [ ] **Step 3: Update `middleware.go` — update no-header branch**

In `internal/auth/middleware.go`, update `authenticate`:

```go
func authenticate(ctx context.Context, store IdentityStore, authHeader string) (*Identity, bool) {
	if authHeader == "" {
		if store.AllowUnauthenticated() {
			return localIdentity(), true
		}
		return nil, false
	}

	scheme, token, ok := strings.Cut(authHeader, " ")
	token = strings.TrimSpace(token)
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return nil, false
	}

	id, err := store.ResolveAPIKey(ctx, token)
	if err != nil {
		return nil, false
	}
	return id, true
}
```

- [ ] **Step 4: Run all auth tests**

Run: `go test ./internal/auth/ -v -count=1`
Expected: All PASS. The existing test for JWT tokens should now return `CodeUnauthenticated` instead of `CodeUnimplemented`. If any test expected `CodeUnimplemented`, update it.

- [ ] **Step 5: Run full build + lint**

Run: `go build ./... && go vet ./...`
Expected: Clean

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "refactor(auth): update interceptor and middleware for AllowUnauthenticated (spgr-0az)"
jj --no-pager new -m ""
```

---

## Chunk 2: OIDCStore + CompositeStore

### Task 4: Add `go-oidc/v3` dependency and implement `OIDCStore`

**Files:**

- Create: `internal/auth/oidc_store.go`
- Create: `internal/auth/oidc_store_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 0: Add go-oidc dependency**

```bash
go get github.com/coreos/go-oidc/v3@latest
```

(Don't run `go mod tidy` yet — it would remove the unused dep. The next steps add the import.)

- [ ] **Step 1: Write failing tests for OIDCStore**

Create `internal/auth/oidc_store_test.go`. The test needs a mock OIDC provider — an `httptest.Server` serving a `.well-known/openid-configuration` and a JWKS endpoint, plus a local RSA key to sign test JWTs.

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

// mockOIDCServer starts a httptest.Server that serves OIDC discovery + JWKS.
// Returns the server and the RSA key used to sign tokens.
func mockOIDCServer(t *testing.T) (*httptest.Server, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	mux := http.NewServeMux()

	// JWKS endpoint
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: "test-key-1", Algorithm: "RS256", Use: "sig"}
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	})

	srv := httptest.NewServer(mux)

	// Discovery endpoint — must be at /.well-known/openid-configuration
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		disc := map[string]interface{}{
			"issuer":                 srv.URL,
			"jwks_uri":              srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(disc)
	})

	return srv, key
}

// signToken creates a signed JWT with the given claims.
func signToken(t *testing.T, key *rsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithHeader("kid", "test-key-1"),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return raw
}

func TestOIDCStore_ValidToken(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "groups", Value: "admins", Role: "admin"},
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss":    srv.URL,
		"aud":    "test-client",
		"sub":    "user-123",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"groups": []string{"admins", "users"},
	})

	id, err := store.ResolveJWT(ctx, token)
	if err != nil {
		t.Fatalf("ResolveJWT: %v", err)
	}
	if id.Subject != "oidc:user-123" {
		t.Errorf("subject = %q, want oidc:user-123", id.Subject)
	}
	if id.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want admin", id.Role)
	}
	if id.Source != "oidc" {
		t.Errorf("source = %q, want oidc", id.Source)
	}
}

func TestOIDCStore_DefaultRole(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "groups", Value: "admins", Role: "admin"},
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	// Token with no matching claim → default role
	token := signToken(t, key, map[string]interface{}{
		"iss":    srv.URL,
		"aud":    "test-client",
		"sub":    "user-456",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"groups": []string{"users"},
	})

	id, err := store.ResolveJWT(ctx, token)
	if err != nil {
		t.Fatalf("ResolveJWT: %v", err)
	}
	if id.Role != auth.RoleReader {
		t.Errorf("role = %q, want reader (default)", id.Role)
	}
}

func TestOIDCStore_ExpiredToken(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss": srv.URL,
		"aud": "test-client",
		"sub": "user-789",
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	})

	_, err = store.ResolveJWT(ctx, token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestOIDCStore_WrongAudience(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "correct-client",
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss": srv.URL,
		"aud": "wrong-client",
		"sub": "user-000",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	_, err = store.ResolveJWT(ctx, token)
	if err == nil {
		t.Fatal("expected error for wrong audience")
	}
}

func TestOIDCStore_StringClaim(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "repository_owner", Value: "specgraph", Role: "writer"},
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss":              srv.URL,
		"aud":              "test-client",
		"sub":              "repo-actor",
		"exp":              time.Now().Add(time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"repository_owner": "specgraph",
	})

	id, err := store.ResolveJWT(ctx, token)
	if err != nil {
		t.Fatalf("ResolveJWT: %v", err)
	}
	if id.Role != auth.RoleWriter {
		t.Errorf("role = %q, want writer", id.Role)
	}
}

func TestOIDCStore_FirstMatchWins(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "groups", Value: "admins", Role: "admin"},
			{Claim: "groups", Value: "admins", Role: "reader"}, // should NOT match
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss":    srv.URL,
		"aud":    "test-client",
		"sub":    "user-first",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"groups": []string{"admins"},
	})

	id, err := store.ResolveJWT(ctx, token)
	if err != nil {
		t.Fatalf("ResolveJWT: %v", err)
	}
	if id.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want admin (first match)", id.Role)
	}
}
```

Also add tests for bad signature and wrong issuer (required by spec):

```go
func TestOIDCStore_BadSignature(t *testing.T) {
	srv, _ := mockOIDCServer(t)
	defer srv.Close()

	// Generate a DIFFERENT key (not the one served by JWKS).
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	// Token signed with wrong key → signature mismatch.
	token := signToken(t, otherKey, map[string]interface{}{
		"iss": srv.URL,
		"aud": "test-client",
		"sub": "user-bad-sig",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	_, err = store.ResolveJWT(ctx, token)
	if err == nil {
		t.Fatal("expected error for bad signature")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run "TestOIDCStore" -v -count=1`
Expected: FAIL — `NewOIDCStore` doesn't exist

- [ ] **Step 3: Implement `OIDCStore`**

Create `internal/auth/oidc_store.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/specgraph/specgraph/internal/config"
)

// OIDCStore verifies JWTs against a single OIDC provider.
type OIDCStore struct {
	providerID    string
	issuer        string
	verifier      *oidc.IDTokenVerifier
	claimsMapping []config.ClaimMapping
	defaultRole   string
	rolePerms     map[Role][]string
}

// NewOIDCStore creates an OIDCStore by discovering the provider's OIDC configuration.
// The context should include a 10-second deadline for startup.
// For test issuers using HTTP, pass oidc.InsecureIssuerURLContext.
func NewOIDCStore(ctx context.Context, cfg config.OIDCProviderConfig, defaultRole string, rolePerms map[Role][]string) (*OIDCStore, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider %s: %w", cfg.ID, err)
	}

	audience := cfg.Audience
	if audience == "" {
		audience = cfg.ClientID
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: audience,
	})

	return &OIDCStore{
		providerID:    cfg.ID,
		issuer:        cfg.Issuer,
		verifier:      verifier,
		claimsMapping: cfg.ClaimsMapping,
		defaultRole:   defaultRole,
		rolePerms:     rolePerms,
	}, nil
}

// Issuer returns the OIDC issuer URL for this store.
func (s *OIDCStore) Issuer() string {
	return s.issuer
}

// ResolveJWT verifies the token and maps claims to an Identity.
func (s *OIDCStore) ResolveJWT(ctx context.Context, rawToken string) (*Identity, error) {
	idToken, err := s.verifier.Verify(ctx, rawToken)
	if err != nil {
		slog.Warn("auth: OIDC token verification failed",
			"provider", s.providerID,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("verify token: %w", err)
	}

	// Extract all claims as raw JSON for flexible mapping.
	var claims map[string]json.RawMessage
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}

	role := s.mapClaims(claims)

	perms, ok := s.rolePerms[Role(role)]
	if !ok {
		slog.Warn("auth: OIDC mapped role not found, falling back to default",
			"provider", s.providerID,
			"role", role,
			"default_role", s.defaultRole,
		)
		role = s.defaultRole
		perms = s.rolePerms[Role(role)]
	}

	permMap := make(map[string]bool, len(perms))
	for _, p := range perms {
		permMap[p] = true
	}

	slog.Debug("auth: OIDC authenticated",
		"provider", s.providerID,
		"subject", idToken.Subject,
		"role", role,
	)

	return &Identity{
		Subject:     "oidc:" + idToken.Subject,
		DisplayName: idToken.Subject,
		Role:        Role(role),
		Permissions: permMap,
		Source:      "oidc",
	}, nil
}

// mapClaims evaluates claims_mapping rules in order, returning the first matching role.
// Falls back to defaultRole if no mapping matches.
func (s *OIDCStore) mapClaims(claims map[string]json.RawMessage) string {
	for _, m := range s.claimsMapping {
		raw, ok := claims[m.Claim]
		if !ok {
			continue
		}
		if matchClaimValue(raw, m.Value) {
			return m.Role
		}
	}
	return s.defaultRole
}

// matchClaimValue checks if the raw JSON claim value matches the target.
// Supports string claims ("value") and string array claims (["a","b"]).
func matchClaimValue(raw json.RawMessage, target string) bool {
	// Try string first.
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str == target
	}

	// Try string array.
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, v := range arr {
			if v == target {
				return true
			}
		}
	}

	return false
}
```

- [ ] **Step 4: Add license header and run tests**

Run: `go test ./internal/auth/ -run "TestOIDCStore" -v -count=1`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
jj --no-pager describe -m "feat(auth): add OIDCStore with JWKS validation and claims mapping (spgr-0az)"
jj --no-pager new -m ""
```

---

### Task 5: Implement `CompositeStore`

**Files:**

- Create: `internal/auth/composite_store.go`
- Create: `internal/auth/composite_store_test.go`

- [ ] **Step 1: Write failing tests for CompositeStore**

Create `internal/auth/composite_store_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

func TestCompositeStore_APIKey(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	cs := auth.NewCompositeStore(cfgStore, nil, "local")
	id, err := cs.ResolveAPIKey(context.Background(), "spgr_sk_test")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if id.Subject != "apikey:k1" {
		t.Errorf("subject = %q, want apikey:k1", id.Subject)
	}
}

func TestCompositeStore_LocalMode_RejectsJWT(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	cs := auth.NewCompositeStore(cfgStore, nil, "local")
	// JWT-shaped token in local mode → ErrUnknownKey (no OIDC routing)
	_, err = cs.ResolveAPIKey(context.Background(), "header.payload.signature")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("error = %v, want ErrUnknownKey", err)
	}
}

func TestCompositeStore_OIDCMode_RoutesJWT(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "groups", Value: "writers", Role: "writer"},
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	oidcStore, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	cs := auth.NewCompositeStore(cfgStore, []*auth.OIDCStore{oidcStore}, "oidc")
	token := signToken(t, key, map[string]interface{}{
		"iss":    srv.URL,
		"aud":    "test-client",
		"sub":    "user-oidc",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"groups": []string{"writers"},
	})

	id, err := cs.ResolveAPIKey(ctx, token)
	if err != nil {
		t.Fatalf("ResolveAPIKey (JWT route): %v", err)
	}
	if id.Source != "oidc" {
		t.Errorf("source = %q, want oidc", id.Source)
	}
	if id.Role != auth.RoleWriter {
		t.Errorf("role = %q, want writer", id.Role)
	}
}

func TestCompositeStore_UnknownIssuer(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	oidcStore, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	cs := auth.NewCompositeStore(cfgStore, []*auth.OIDCStore{oidcStore}, "oidc")

	// Token with different issuer
	token := signToken(t, key, map[string]interface{}{
		"iss": "https://unknown-issuer.example.com",
		"aud": "test-client",
		"sub": "user-unknown",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	_, err = cs.ResolveAPIKey(ctx, token)
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("error = %v, want ErrUnknownKey (unknown issuer)", err)
	}
}

func TestCompositeStore_AllowUnauthenticated_MixedMode(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	cs := auth.NewCompositeStore(cfgStore, nil, "mixed")
	if !cs.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = false, want true in mixed mode")
	}
}

func TestCompositeStore_AllowUnauthenticated_OIDCMode(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	cs := auth.NewCompositeStore(cfgStore, nil, "oidc")
	if cs.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = true, want false in oidc mode")
	}
}

func TestCompositeStore_AllowUnauthenticated_LocalNoKeys(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	cs := auth.NewCompositeStore(cfgStore, nil, "local")
	if !cs.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = false, want true in local mode with no keys")
	}
}

func TestCompositeStore_AllowUnauthenticated_LocalWithKeys(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	cs := auth.NewCompositeStore(cfgStore, nil, "local")
	if cs.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = true, want false in local mode with keys")
	}
}

func TestCompositeStore_HasAuth_WithOIDCOnly(t *testing.T) {
	srv, _ := mockOIDCServer(t)
	defer srv.Close()

	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	oidcStore, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	cs := auth.NewCompositeStore(cfgStore, []*auth.OIDCStore{oidcStore}, "oidc")
	if !cs.HasAuth() {
		t.Error("HasAuth() = false, want true with OIDC providers")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run "TestCompositeStore" -v -count=1`
Expected: FAIL — `NewCompositeStore` doesn't exist

- [ ] **Step 3: Implement `CompositeStore`**

Create `internal/auth/composite_store.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// CompositeStore routes authentication to ConfigStore (API keys) or OIDCStore (JWTs).
// It implements IdentityStore and encapsulates all routing logic.
type CompositeStore struct {
	config     *ConfigStore
	oidc       []*OIDCStore
	issuerMap  map[string]*OIDCStore
	mode       string
}

// NewCompositeStore creates a CompositeStore wrapping the given stores.
// mode must be "local", "oidc", or "mixed".
func NewCompositeStore(config *ConfigStore, oidc []*OIDCStore, mode string) *CompositeStore {
	issuerMap := make(map[string]*OIDCStore, len(oidc))
	for _, s := range oidc {
		issuerMap[s.Issuer()] = s
	}
	return &CompositeStore{
		config:    config,
		oidc:      oidc,
		issuerMap: issuerMap,
		mode:      mode,
	}
}

// ResolveAPIKey tries the ConfigStore first. On ErrUnknownKey, if the token
// is JWT-shaped and mode allows OIDC, delegates to ResolveJWT.
func (s *CompositeStore) ResolveAPIKey(ctx context.Context, token string) (*Identity, error) {
	id, err := s.config.ResolveAPIKey(ctx, token)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, ErrUnknownKey) {
		return nil, err
	}

	// In local mode, no OIDC routing.
	if s.mode == "local" {
		return nil, ErrUnknownKey
	}

	// Check if token looks like a JWT (3 dot-separated segments).
	if strings.Count(token, ".") != 2 {
		return nil, ErrUnknownKey
	}

	id, jwtErr := s.ResolveJWT(ctx, token)
	if jwtErr != nil {
		// Map OIDC-specific errors to ErrUnknownKey for the interceptor.
		if errors.Is(jwtErr, ErrUnknownIssuer) {
			return nil, ErrUnknownKey
		}
		return nil, jwtErr
	}
	return id, nil
}

// ResolveJWT peeks at the unverified issuer claim and routes to the matching OIDCStore.
func (s *CompositeStore) ResolveJWT(ctx context.Context, token string) (*Identity, error) {
	issuer, err := peekIssuer(token)
	if err != nil {
		slog.Warn("auth: malformed JWT, cannot extract issuer", "error", err.Error())
		return nil, ErrUnknownKey
	}

	store, ok := s.issuerMap[issuer]
	if !ok {
		slog.Warn("auth: unknown JWT issuer", "issuer", issuer)
		return nil, ErrUnknownIssuer
	}

	return store.ResolveJWT(ctx, token)
}

// HasAuth reports whether any authentication mechanism is configured.
func (s *CompositeStore) HasAuth() bool {
	return s.config.HasAuth() || len(s.oidc) > 0
}

// AllowUnauthenticated reports whether unauthenticated requests get local identity.
func (s *CompositeStore) AllowUnauthenticated() bool {
	switch s.mode {
	case "mixed":
		return true
	case "local":
		return !s.config.HasAuth()
	default: // "oidc"
		return false
	}
}

// peekIssuer extracts the "iss" claim from the JWT payload without verification.
// This is safe because it's only used for routing — the actual verification
// happens in the OIDCStore after routing.
func peekIssuer(token string) (string, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return "", errors.New("not a JWT")
	}

	// JWT payload is base64url-encoded (no padding).
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}

	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("unmarshal payload: %w", err)
	}
	if claims.Issuer == "" {
		return "", errors.New("missing iss claim")
	}
	return claims.Issuer, nil
}
```

- [ ] **Step 4: Run composite tests**

Run: `go test ./internal/auth/ -run "TestCompositeStore" -v -count=1`
Expected: All PASS

- [ ] **Step 5: Run full auth test suite**

Run: `go test ./internal/auth/ -v -count=1`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(auth): add CompositeStore for API key + OIDC routing with mode enforcement (spgr-0az)"
jj --no-pager new -m ""
```

---

## Chunk 3: Bootstrap + Server Integration

### Task 6: Implement credential bootstrap

**Files:**

- Create: `internal/auth/bootstrap.go`
- Create: `internal/auth/bootstrap_test.go`

- [ ] **Step 1: Write failing tests for bootstrap**

Create `internal/auth/bootstrap_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
	"gopkg.in/yaml.v3"
)

func TestBootstrap_GeneratesKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	key, err := auth.Bootstrap(path)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if key == "" {
		t.Fatal("Bootstrap returned empty key")
	}
	if len(key) != 40 {
		t.Errorf("key too short: %d chars", len(key))
	}
	if key[:8] != "spgr_sk_" {
		t.Errorf("key prefix = %q, want spgr_sk_", key[:8])
	}

	// Verify file exists with correct permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %o, want 600", info.Mode().Perm())
	}

	// Verify file content is valid YAML with the key.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	var creds auth.CredentialsFile
	if err := yaml.Unmarshal(data, &creds); err != nil {
		t.Fatalf("unmarshal credentials: %v", err)
	}
	if len(creds.APIKeys) != 1 {
		t.Fatalf("api_keys count = %d, want 1", len(creds.APIKeys))
	}
	if creds.APIKeys[0].Key != key {
		t.Errorf("stored key = %q, want %q", creds.APIKeys[0].Key, key)
	}
	if creds.APIKeys[0].Role != "admin" {
		t.Errorf("role = %q, want admin", creds.APIKeys[0].Role)
	}
}

func TestBootstrap_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	key1, err := auth.Bootstrap(path)
	if err != nil {
		t.Fatalf("Bootstrap first: %v", err)
	}

	key2, err := auth.Bootstrap(path)
	if err != nil {
		t.Fatalf("Bootstrap second: %v", err)
	}

	if key1 != key2 {
		t.Errorf("second call returned different key: %q vs %q", key1, key2)
	}
}

func TestBootstrap_PermissionWarning(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	// First create normally.
	_, err := auth.Bootstrap(path)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Widen permissions.
	if err := os.Chmod(path, 0o644); err != nil { //nolint:gosec // intentional for test
		t.Fatalf("chmod: %v", err)
	}

	// CheckCredentialPermissions should return a warning.
	warning := auth.CheckCredentialPermissions(path)
	if warning == "" {
		t.Error("expected warning for open permissions")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run "TestBootstrap" -v -count=1`
Expected: FAIL — `Bootstrap`, `CredentialsFile`, `CheckCredentialPermissions` don't exist

- [ ] **Step 3: Implement bootstrap**

Create `internal/auth/bootstrap.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/specgraph/specgraph/internal/config"
)

// CredentialsFile is the structure of the credentials.yaml file.
type CredentialsFile struct {
	APIKeys []config.APIKeyConfig `yaml:"api_keys"`
}

// Bootstrap generates a default admin API key and writes it to credentialsPath.
// If the file already exists, reads and returns the existing key.
// Returns the generated/existing API key value.
func Bootstrap(credentialsPath string) (string, error) {
	// Check if file already exists.
	data, err := os.ReadFile(credentialsPath)
	if err == nil {
		var creds CredentialsFile
		if unmarshalErr := yaml.Unmarshal(data, &creds); unmarshalErr != nil {
			return "", fmt.Errorf("parse existing credentials: %w", unmarshalErr)
		}
		if len(creds.APIKeys) > 0 {
			return creds.APIKeys[0].Key, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read credentials: %w", err)
	}

	// Generate new key.
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("generate random key: %w", err)
	}
	key := "spgr_sk_" + hex.EncodeToString(keyBytes)

	creds := CredentialsFile{
		APIKeys: []config.APIKeyConfig{
			{
				ID:   "default-admin",
				Key:  key,
				Name: "Default Admin (auto-generated)",
				Role: "admin",
			},
		},
	}

	yamlData, err := yaml.Marshal(creds)
	if err != nil {
		return "", fmt.Errorf("marshal credentials: %w", err)
	}

	header := []byte("# Auto-generated by specgraph server bootstrap. Do not commit to version control.\n")
	content := append(header, yamlData...)

	if err := os.MkdirAll(filepath.Dir(credentialsPath), 0o750); err != nil {
		return "", fmt.Errorf("create credentials directory: %w", err)
	}

	if err := os.WriteFile(credentialsPath, content, 0o600); err != nil {
		return "", fmt.Errorf("write credentials: %w", err)
	}

	return key, nil
}

// CheckCredentialPermissions returns a warning message if the credentials file
// has permissions more open than 0600. Returns "" if permissions are OK or
// the file doesn't exist.
func CheckCredentialPermissions(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		return fmt.Sprintf("credentials file %s has permissions %04o (expected 0600)", path, perm)
	}
	return ""
}
```

- [ ] **Step 4: Run bootstrap tests**

Run: `go test ./internal/auth/ -run "TestBootstrap" -v -count=1`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
jj --no-pager describe -m "feat(auth): add credential bootstrap for local mode API key generation (spgr-0az)"
jj --no-pager new -m ""
```

---

### Task 7: Wire everything together in `serve.go`

**Files:**

- Modify: `cmd/specgraph/serve.go:93-102`
- Modify: `internal/xdg/xdg.go` (add `CredentialsFile` helper)

- [ ] **Step 1: Add `CredentialsFile` path helper to xdg package**

In `internal/xdg/xdg.go`, add after `ConfigFile()`:

```go
// CredentialsFile returns the path to the credentials file.
func CredentialsFile() string {
	return filepath.Join(ConfigHome(), "credentials.yaml")
}
```

- [ ] **Step 2: Update `serve.go` to wire CompositeStore with OIDC and bootstrap**

In `cmd/specgraph/serve.go`, replace the auth setup block (lines 93-102):

```go
		credPath := xdg.CredentialsFile()
		authStore, err := auth.NewConfigStore(cfg.Auth, credPath)
		if err != nil {
			return fmt.Errorf("auth config: %w", err)
		}

		mode := cfg.Auth.Mode
		if mode == "" {
			mode = "local"
		}

		// Validate auth mode.
		switch mode {
		case "local", "oidc", "mixed":
		default:
			return fmt.Errorf("invalid auth.mode %q (must be local, oidc, or mixed)", mode)
		}
		if mode == "oidc" && len(cfg.Auth.OIDCProviders) == 0 {
			return fmt.Errorf("auth.mode=oidc requires at least one oidc_providers entry")
		}

		// Bootstrap: generate default admin key in local mode if none configured.
		if mode == "local" && !authStore.HasKeys() {
			key, bootstrapErr := auth.Bootstrap(credPath)
			if bootstrapErr != nil {
				return fmt.Errorf("auth bootstrap: %w", bootstrapErr)
			}
			fmt.Fprintf(os.Stderr, "\n  SpecGraph generated a default admin API key:\n\n    %s\n\n  Save this key — it won't be shown again.\n  Stored in: %s\n\n", key, credPath)

			// Reload store with the new key.
			authStore, err = auth.NewConfigStore(cfg.Auth, credPath)
			if err != nil {
				return fmt.Errorf("reload auth after bootstrap: %w", err)
			}
		}

		if warning := auth.CheckCredentialPermissions(credPath); warning != "" {
			slog.Warn(warning)
		}

		// Set up OIDC providers (only for oidc/mixed modes).
		var oidcStores []*auth.OIDCStore
		if mode != "local" {
			defaultRole := cfg.Auth.DefaultRole
			if defaultRole == "" {
				defaultRole = "reader"
			}

			rolePerms := make(map[auth.Role][]string)
			for role, perms := range auth.DefaultRolePermissions {
				rolePerms[role] = perms
			}
			for name, rc := range cfg.Auth.Roles {
				rolePerms[auth.Role(name)] = rc.Permissions
			}

			oidcCtx, oidcCancel := context.WithTimeout(ctx, 10*time.Second)
			defer oidcCancel()

			for _, pc := range cfg.Auth.OIDCProviders {
				oidcStore, oidcErr := auth.NewOIDCStore(oidcCtx, pc, defaultRole, rolePerms)
				if oidcErr != nil {
					return fmt.Errorf("OIDC provider %s: %w", pc.ID, oidcErr)
				}
				oidcStores = append(oidcStores, oidcStore)
				slog.Info("auth: OIDC provider configured", "id", pc.ID, "issuer", pc.Issuer)
			}
		}

		compositeStore := auth.NewCompositeStore(authStore, oidcStores, mode)

		if !compositeStore.HasAuth() && !isLoopbackAddr(cfg.Server.Listen) {
			slog.Warn("server listening without authentication on non-loopback interface",
				"addr", cfg.Server.Listen,
				"risk", "all requests will have full admin access")
		}
		interceptor := auth.NewAuthInterceptor(compositeStore)
```

Also update the `RequireAuth` call later (line ~131):

```go
		server.RegisterAPIHandlers(mux, store, auth.RequireAuth(compositeStore))
```

Ensure `time` is in the import block.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Clean

- [ ] **Step 4: Run `task check`**

Run: `task check`
Expected: All checks pass (fmt, lint, build, unit tests)

- [ ] **Step 5: Commit**

```bash
jj --no-pager describe -m "feat(auth): wire CompositeStore, OIDC providers, and bootstrap into serve (spgr-0az)"
jj --no-pager new -m ""
```

---

### Task 8: Run full test suite and fix any issues

- [ ] **Step 1: Run unit tests**

Run: `task test`
Expected: All PASS

- [ ] **Step 2: Run integration tests (requires Docker)**

Run: `task test:integration`
Expected: All PASS (existing memgraph tests should be unaffected)

- [ ] **Step 3: Run `task pr-prep`**

Run: `task pr-prep`
Expected: All checks pass including e2e

- [ ] **Step 4: Review change stack**

```bash
jj --no-pager log --limit 10
```

Leave as atomic commits per task (one commit per task is the intended granularity). Do not squash unless specifically requested.

- [ ] **Step 5: Close the bead**

```bash
bd update spgr-0az --claim
bd close spgr-0az --reason="OIDC authentication implemented: multi-provider JWKS validation, claims mapping, auth modes, credential bootstrap"
```
