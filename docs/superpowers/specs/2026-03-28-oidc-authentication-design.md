# OIDC Authentication Design

**Date:** 2026-03-28
**Bead:** spgr-0az (OIDC authentication: Azure Entra ID, GitHub, and local accounts)
**Epic:** spgr-5wb (Authentication & Authorization)
**Status:** Draft

## Problem

SpecGraph's auth system (PR #38) supports API keys and local identity fallback, but has no OIDC support. The JWT detection hook in `resolveIdentity` returns `CodeUnimplemented` for JWT-shaped tokens. Multi-user teams using Azure Entra ID or GitHub as their identity provider cannot authenticate against SpecGraph without manually provisioning API keys for each user.

## Decision Summary

| Question | Decision |
|----------|----------|
| Provider model | Multi-provider: multiple OIDC providers simultaneously + local fallback |
| Claims to role mapping | Static claim mapping in config + configurable default role |
| Token validation | Local JWKS validation via `go-oidc/v3` (no per-request introspection) |
| Local mode behavior | Explicit `auth.mode` config field, defaults to `local` |
| Integration approach | Composite `IdentityStore` — extend interface with `ResolveJWT` |
| Credential storage | Separate `credentials.yaml` file (`0600`), not in main config |
| Bootstrap | Automatic admin API key generation on first server start |
| Library | `github.com/coreos/go-oidc/v3` for OIDC discovery + JWKS + ID token verification |

## Config Schema

```yaml
auth:
  mode: local            # local | oidc | mixed (default: local)
  default_role: reader   # role for OIDC users with no matching claim mapping

  api_keys:              # existing, unchanged
    - id: ci-bot
      key: spgr_sk_...
      name: CI Bot
      role: writer

  oidc_providers:
    - id: entra
      issuer: https://login.microsoftonline.com/{tenant}/v2.0
      client_id: <app-id>
      audience: <api-audience>   # optional, defaults to client_id
      claims_mapping:
        - claim: groups
          value: "specgraph-admins"
          role: admin
        - claim: groups
          value: "specgraph-writers"
          role: writer
    - id: github
      issuer: https://token.actions.githubusercontent.com
      client_id: <client-id>
      claims_mapping:
        - claim: repository_owner
          value: "specgraph"
          role: writer

  roles:                 # existing custom roles, unchanged
    deployer:
      permissions: ["spec:read", "execution:*"]
```

### Auth Mode Semantics

**When auth is configured (keys or OIDC providers), all requests require a valid token.** When no auth is configured (`!HasAuth()`), requests fall back to a local admin identity. In `local` mode, bootstrap auto-generates an admin API key on first writable start — after which auth is required.

| Mode | Bearer token present | No Authorization header |
|------|---------------------|----------------------|
| `local` | Try API key only. JWT-shaped tokens return `CodeUnauthenticated` (OIDC disabled, `CompositeStore` skips JWT routing). | If keys configured: `CodeUnauthenticated`. If no keys (pre-bootstrap or read-only env): local admin identity. |
| `oidc` | Try API key, then OIDC for JWT-shaped tokens. | `CodeUnauthenticated`. |
| `mixed` | Try API key, then OIDC for JWT-shaped tokens. | If keys/OIDC configured: `CodeUnauthenticated`. If none: local admin identity. |

The `CompositeStore` checks `Mode` before attempting OIDC resolution. In `local` mode, `ResolveAPIKey` never delegates to `ResolveJWT` even for JWT-shaped tokens — it returns `ErrUnknownKey` directly.

**Note on `mixed` mode:** `mixed` enables both API key and OIDC authentication simultaneously. Useful during migration from API keys to OIDC — existing API key clients continue working alongside new OIDC clients.

### Config Validation Rules

- `mode` must be one of `local`, `oidc`, `mixed`. Go type: `string` with const validation.
- `mode=oidc` requires at least one entry in `oidc_providers`.
- `default_role` must reference a built-in role (`admin`, `writer`, `reader`) or a custom role defined in `roles`. Defaults to `reader` if omitted.
- `oidc_providers[].id` values must be unique.
- `oidc_providers[].issuer` must be a valid HTTPS URL (enforced by `go-oidc` discovery).
- `claims_mapping[].role` must reference a known role.

### Bootstrap Behavior (local mode)

On first server start with no API keys configured:

1. Generate a default admin API key (`spgr_sk_<32-char-random>`).
2. Write to `~/.config/specgraph/credentials.yaml` with `0600` permissions.
3. Print the key to stdout with a "save this key" warning.
4. Subsequent starts load keys from credentials file — bootstrap is idempotent.
5. Bootstrap only runs in `local` mode. In `oidc` or `mixed` mode, no auto-generation occurs — operators must configure keys explicitly if needed.

### Credentials File

**Path resolution:** Uses XDG convention — `$XDG_CONFIG_HOME/specgraph/credentials.yaml`, falling back to `~/.config/specgraph/credentials.yaml`. Follows the same logic as the existing `GlobalConfig` loading in `internal/config/`.

**Format:**

```yaml
# Auto-generated by specgraph server bootstrap. Do not commit to version control.
api_keys:
  - id: default-admin
    key: spgr_sk_...
    name: "Default Admin (auto-generated)"
    role: admin
```

**Merge semantics:** Keys from `credentials.yaml` are merged with keys from the main `config.yaml` at load time. If both files define a key with the same `id`, the main config wins (explicit config overrides auto-generated). This merge is implemented in `NewConfigStore` (`internal/auth/config_store.go`) via `loadCredentialKeys` — the function accepts both `AuthConfig` (from main config) and a `credentialsPath` string, reads the credential file if it exists, and appends any keys whose IDs don't conflict with config keys.

**Startup check:** If `credentials.yaml` exists but has permissions more open than `0600`, log a warning.

## Architecture

### New Types

| Type | File | Purpose |
|------|------|---------|
| `OIDCStore` | `internal/auth/oidc_store.go` | Per-provider JWKS verifier using `go-oidc/v3`. Validates JWT, extracts claims, applies claim mappings, returns `*Identity`. |
| `CompositeStore` | `internal/auth/composite_store.go` | Wraps `ConfigStore` + `[]OIDCStore` + auth `Mode`. Implements `IdentityStore`. `ResolveAPIKey` tries `ConfigStore` first, then internally delegates JWT-shaped tokens to `ResolveJWT`. `ResolveJWT` routes to the matching `OIDCStore` by `iss` claim. `HasAuth` returns true if any keys or providers are configured. |
| `OIDCProviderConfig` | `internal/config/global.go` | Config struct for `oidc_providers` YAML section. |
| `ClaimMapping` | `internal/config/global.go` | Config struct for claim-to-role mapping rules. |

### Modified Types

| Type | Change |
|------|--------|
| `IdentityStore` | Add `ResolveJWT(ctx context.Context, token string) (*Identity, error)` and `HasAuth() bool` (replaces `HasKeys`, returns true if any auth is configured — keys or OIDC). All callers of `HasKeys` must update: `interceptor.go:68`, `middleware.go:33`, `serve.go:97`. No-header branch uses `!store.HasAuth()` → `localIdentity()` fallback (only when no keys/OIDC configured). |
| `AuthConfig` | Add `Mode string`, `DefaultRole string`, `OIDCProviders []OIDCProviderConfig` fields |
| `resolveIdentity()` in `interceptor.go` | Replace `CodeUnimplemented` JWT stub — `store.ResolveAPIKey()` handles JWT fallback internally via `CompositeStore`. No-header branch: `!store.HasAuth()` → `localIdentity()`, else `CodeUnauthenticated`. Remove explicit JWT-shape detection (dead code after composite routing). |
| `authenticate()` in `middleware.go` | No-header branch: `!store.HasAuth()` → `localIdentity()`, else `nil, false`. No other changes — `store.ResolveAPIKey()` handles JWT routing internally. |
| `serve.go` | Update `authStore.HasKeys()` → `authStore.HasAuth()` in advisory loopback warning. Change `NewConfigStore` call to pass credentials path. |
| `ConfigStore` | Implement `ResolveJWT` as no-op returning `ErrNoOIDC`. Implement `HasAuth` delegating to existing `HasKeys`. |
| `NewConfigStore` | Signature changes to `NewConfigStore(cfg config.AuthConfig, credentialsPath string) (*ConfigStore, error)`. Reads and merges keys from credentials file before validating. Main config keys with duplicate IDs take precedence over credentials file keys. |

### New Files

| File | Purpose |
|------|---------|
| `internal/auth/oidc_store.go` | OIDC provider setup, JWKS caching, token verification, claims extraction |
| `internal/auth/oidc_store_test.go` | Tests with mock OIDC provider (httptest server serving fake JWKS) |
| `internal/auth/composite_store.go` | Composite routing logic |
| `internal/auth/composite_store_test.go` | API key + OIDC resolution integration |
| `internal/auth/bootstrap.go` | Credential file generation, API key bootstrap |
| `internal/auth/bootstrap_test.go` | Bootstrap idempotency, file permissions |

### Token Resolution Flow

The interceptor (`resolveIdentity`) and HTTP middleware (`authenticate`) both delegate to `IdentityStore` methods. The `CompositeStore` encapsulates all routing logic internally — callers don't choose which method to call based on token shape. Instead:

1. `CompositeStore.ResolveAPIKey(ctx, token)` tries the `ConfigStore` first.
2. If `ConfigStore` returns `ErrUnknownKey` and the token is JWT-shaped (3 dot-segments), `CompositeStore` internally delegates to `ResolveJWT`.
3. `CompositeStore.ResolveJWT(ctx, token)` peeks at the unverified `iss` claim, routes to the matching `OIDCStore`, and verifies.

This means the interceptor's `resolveIdentity` structure barely changes — it still calls `store.ResolveAPIKey()` as the primary path. The `CompositeStore` handles the API-key-then-JWT fallback internally, keeping the interceptor and middleware simple.

```text
Bearer token arrives -> store.ResolveAPIKey(ctx, token)
  -> CompositeStore internally:
    -> try ConfigStore (API key lookup)
      -> found: return Identity
      -> ErrUnknownKey:
        -> mode=local? -> ErrUnknownKey (no OIDC routing in local mode)
        -> mode=oidc|mixed + token is JWT-shaped:
          -> peek unverified `iss` claim
          -> match against OIDCStore issuers
            -> found: verify via JWKS, extract claims, apply mappings -> Identity
            -> not found: ErrUnknownKey (bubbles up)
        -> not JWT: ErrUnknownKey (bubbles up)
  -> no Authorization header?
    -> store.HasAuth() is false: localIdentity() (pre-bootstrap or read-only)
    -> store.HasAuth() is true: CodeUnauthenticated
```

Both `resolveIdentity` (interceptor) and `authenticate` (middleware) call `store.ResolveAPIKey` for the token path and `store.HasAuth()` for the no-header path. Token routing logic is internal to `CompositeStore`.

### Claims Mapping Logic

For a verified JWT, claims are evaluated against the provider's `claims_mapping` list in order:

1. Extract the claim value from the JWT (supports string and string-array claims).
2. For array claims (e.g., `groups`), check if any element matches.
3. First matching rule wins — assign that rule's role.
4. No match — assign `default_role` from config.
5. Build `Identity` with `Source: "oidc"`, `Subject: "oidc:<sub>"`, resolved permissions from role.

### Unchanged Files

- **`permissions.go`** — RPC permission map is role-based, not auth-method-based. No changes needed for OIDC.
- **`context.go`** — `WithIdentity`/`IdentityFromContext` are already generic over `Identity.Source`. No changes needed.

## Error Handling

| Scenario | Error Code | Log Level |
|----------|-----------|-----------|
| Expired JWT | `CodeUnauthenticated` | Warn (with `sub`, `exp`, provider ID) |
| Invalid signature | `CodeUnauthenticated` | Warn (with provider ID) |
| Unknown issuer | `CodeUnauthenticated` | Warn (with issuer value) |
| JWKS fetch failure on startup | Server fails to start | Error |
| JWKS refresh failure at runtime | Stale keys continue, retry with backoff | Error |
| Claims mapping yields no role match | Uses `default_role` | Info |
| Malformed JWT | `CodeUnauthenticated` | Warn |

**New sentinel errors:**

- `ErrNoOIDC` — returned by `ConfigStore.ResolveJWT` (no OIDC support in standalone config store). Internal only, never surfaces to clients.
- `ErrUnknownIssuer` — returned by `CompositeStore` when JWT `iss` doesn't match any configured provider. Mapped to `ErrUnknownKey` by `CompositeStore.ResolveAPIKey`, then to `CodeUnauthenticated` by the interceptor.
- `ErrInvalidToken` — returned by `OIDCStore.ResolveJWT` when a JWT fails verification (expired, bad signature, wrong audience). Mapped to `ErrUnknownKey` by `CompositeStore.ResolveAPIKey`, then to `CodeUnauthenticated` by the interceptor. Internal only.

**Error-to-code mapping in interceptor:** `ErrUnknownKey` from `ResolveAPIKey` (including composite fallthrough) → `CodeUnauthenticated`. This preserves the existing behavior where unrecognized tokens are treated as unauthenticated, not as internal errors.

### Security Invariants

- **No error message leakage** — clients see error codes only, details in server logs.
- **JWKS over HTTPS only** — `go-oidc` enforces this. Test-only `InsecureIssuerURLContext` for unit tests.
- **Credentials file `0600`** — bootstrap enforces. Startup warns if permissions too open.
- **Unverified `iss` peek is routing only** — cryptographic verification happens after routing to the correct provider's verifier. Forged `iss` picks the wrong verifier, which rejects the signature.
- **API key constant-time comparison** — existing `fixedLenCompare` unchanged.

### Graceful Degradation

- Provider unreachable at startup: fail fast with a 10-second context deadline on OIDC provider discovery. Refuse to start with broken auth.
- Provider unreachable at runtime: `go-oidc/v3`'s `RemoteKeySet` fetches JWKS on first use. It does NOT automatically re-fetch on unknown `kid` — tokens signed with keys not in the cached JWKS fail verification immediately. There is no TTL-based cache expiry. Tokens signed with already-cached keys continue to verify. If the provider rotates keys, new tokens will fail until the server is restarted (triggering a fresh JWKS fetch via `oidc.NewProvider`).
- Single provider down in multi-provider setup: only that provider's tokens fail. Others unaffected.

## Testing Strategy

### Unit Tests (no network, no Docker)

| Test | File | Coverage |
|------|------|----------|
| OIDC token verification | `oidc_store_test.go` | Mock OIDC provider via `httptest.Server` with local RSA key. Valid token, expired, wrong audience, wrong issuer, bad signature, unknown key ID. |
| Claims mapping | `oidc_store_test.go` | First-match wins, groups claim (string array), no match -> default role, empty claims. |
| Composite routing | `composite_store_test.go` | API key -> ConfigStore, JWT -> OIDCStore by issuer, unknown issuer -> error, mode-based local fallback. |
| Bootstrap | `bootstrap_test.go` | First-run generation, idempotency, file permissions `0600`, key format (`spgr_sk_` prefix). |
| Config validation | `config_store_test.go` (extend) | Mode validation, provider config required for `mode=oidc`, default_role must be known role. |

### Integration Tests (`//go:build integration`)

| Test | Coverage |
|------|----------|
| Full auth flow | `httptest.TLSServer` as mock IdP. `CompositeStore` configured with it. Requests through `NewAuthInterceptor`. Token -> identity -> context -> permission check. |
| Multi-provider routing | Two mock IdP servers. JWT from provider A -> verifier A, provider B -> verifier B. |
| Mode enforcement | `local` rejects OIDC tokens, `oidc` rejects unauthenticated, `mixed` allows both. |

### Not Testing

- No tests against real Entra ID / GitHub — environment-specific, flaky in CI. Mock IdP covers the same protocol flows.
- No E2E tests for bootstrap — unit tests for file I/O sufficient.

## Dependencies

- `github.com/coreos/go-oidc/v3` — OIDC provider discovery, JWKS, ID token verification.
- `golang.org/x/oauth2` — transitive dependency of `go-oidc` (used for HTTP client context).
