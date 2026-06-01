# Identity Bootstrap & UX — Plan 4a: IdentityService RPC Surface + CLI

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the existing `storage.UsersBackend` as an `IdentityService` ConnectRPC API (users, service accounts, API keys, OIDC bindings, whoami), gate it in Cedar as admin-only (plus self-readable whoami), and surface it as the `specgraph auth` CLI subtree — so a hosted operator with RPC-only access can manage identity.

**Architecture:** This is the FIRST of two plans split from the "Identity Bootstrap & Operator UX" design (`docs/plans/2026-05-22-identity-bootstrap-ux-design.md`). 4a builds the operator RPC surface + CLI; **Plan 4b** (bootstrap flows, credentials-file, force-flag protections) follows immediately and depends on these RPCs. The `IdentityService` is **global** (not project-scoped) — its handler holds a `storage.UsersBackend` directly, unlike the project-scoped handlers that take a `storage.Scoper`. Authorization is admin-only via a new Cedar `manage` verb (additive to the Cedar plan); `whoami` is self-scoped via the existing `read` verb. Self-service ownership (rotate *my own* key) is explicitly deferred to story #4 per the Cedar design's sequencing. Build-alongside discipline: every task ends with `go build ./... && go test ./...` green at package and whole-project level.

**Tech Stack:** Go 1.26, ConnectRPC, Protobuf (buf via `task proto`), pgx v5 (via `UsersBackend`), `golang.org/x/crypto/argon2` (API-key minting), Cobra (CLI), `cedar-go` (authz, via the Cedar plan's engine).

**Implements bead:** Part of approved design `spgr-g90r` (Identity Bootstrap & UX) under epic `spgr-rjrt`, sub-plan 4a. Depends on the Storage (`spgr-e82m`), Authn (`spgr-n2rw`), and Cedar (`spgr-rjrt.1`) plans being merged first.

---

## Dependency contract (what the prior plans provide)

This plan builds on the **merged output** of the three prior plans. The symbols it consumes:

- **Storage** — `storage.UsersBackend` (interface with `ListUsers`, `GetUserByID`, `CreateServiceAccount`, `UpdateUserRole`, `SoftDeleteUser`, `PurgeUser`, `CreateAPIKey`, `RevokeAPIKey`, `RotateAPIKey`, `ListAPIKeys`, `ListOIDCBindings`, `UnbindOIDC`, `GetBootstrap`); domain types `storage.User`, `storage.APIKey`, `storage.OIDCBinding`, `storage.Kind` (`KindHuman`/`KindServiceAccount`); filters `storage.ListUsersFilter`, `storage.ListAPIKeysFilter`; sentinels `storage.ErrUserNotFound`, `storage.ErrAPIKeyNotFound`, `storage.ErrOIDCBindingNotFound`, `storage.ErrBootstrapExists`, `storage.ErrAPIKeyPrefixExists`. The postgres `*AuthStore` (constructed via `postgres.NewAuth(ctx, pool)`) implements `UsersBackend`.
  - **Key-generation contract (load-bearing for Tasks 7–8):** `CreateAPIKey(ctx, k)` and `RotateAPIKey(ctx, oldID, newKey)` **own prefix generation** — they call `s.genPrefix()` and IGNORE any `Prefix` the caller set, returning the key with the storage-assigned prefix. `RotateAPIKey` does NOT copy metadata from the old key; it inserts `newKey.UserID`, `newKey.PHCHash`, `newKey.RoleDowngrade`, `newKey.Label`, `newKey.ExpiresAt`, so the caller MUST supply `UserID` + metadata on `newKey` (there is no get-key-by-id backend method to look the old one up). This is why minting is split (`GenerateAPIKeySecret` + `FormatAPIKeyToken`) and `RotateAPIKeyRequest` carries `user_id`.
- **Authn** — `auth.Identity` (fields `Subject`, `UserID`, `EffectiveRole`, `Email`, `DisplayName`, `Role`, `Source`); `auth.IdentityFromContext(ctx) (*Identity, bool)` and `auth.WithIdentity(ctx, *Identity) context.Context` (both in `internal/auth/context.go`); `auth.RoleReader`/`RoleWriter`/`RoleAdmin` (typed `auth.Role`; `RoleAdmin == "admin"`, so `string(id.Role)` yields `"admin"`); the post-cleanup interceptor constructor `auth.NewAuthInterceptor(resolver, authorizer)` (Authn Task 30b renames `NewAuthInterceptorV2`→`NewAuthInterceptor`); the API-key format constants in `internal/auth/identitystore.go`: `apiKeyPrefix = "spgr_sk_"`, `apiKeyPrefixLen = 8`, `apiKeySecretLen = 32` (unexported, same package), and the argon2id PHC scheme (`argon2.IDKey(secret, salt, 2, 19456, 1, 32)`, salt 16 bytes, format `$argon2id$v=19$m=19456,t=2,p=1$<b64salt>$<b64hash>`). `GenerateAPIKeySecret` reuses `apiKeyPrefix`/`apiKeySecretLen` but NOT `apiKeyPrefixLen` (storage owns the prefix).
- **Cedar** — `auth.procedureActions` map + `auth.ActionForProcedure`/`auth.ActionNames`; `auth.knownVerbs`; the engine's `buildActionEntities`; `internal/auth/policies/base.cedar`; `CedarAuthorizer` (drives interceptor authz; an unconfigured procedure → `CodeInternal`).

If any of these names differ in the merged code, adapt to the actual name — they are pinned here so the plan's code is concrete, not fictional.

---

## Testing approach

Four categories (same framework as the prior plans). Each behavior task tags `**Covers:**`.

- **Happy** — normal success per RPC / command.
- **Invariants** — admin-only gating holds for every management RPC; minted keys verify against the Authn resolver; plaintext is returned exactly once and never persisted; whoami reflects the caller.
- **Boundaries** — NotFound mapping, invalid arguments, unconfigured-procedure guard, empty lists.
- **E2E** — interceptor → Cedar → handler → backend, and CLI → RPC.

Handler unit tests use a hand-rolled `UsersBackend` stub (Task 3 introduces a server-package stub; it mirrors the Authn plan's `usersBackendStub` shape but lives in `internal/server`). Integration tests (`//go:build integration`) use the testcontainer `AuthStore` and the real interceptor.

---

## File Structure

**Create:**

- `proto/specgraph/v1/identity.proto` — `IdentityService` + messages. (`task proto` regenerates `gen/specgraph/v1/identity*.go` + `specgraphv1connect/identity.connect.go`.)
- `internal/server/convert_identity.go` (+ `_test.go`) — `userToProto`, `apiKeyToProto`, `oidcBindingToProto`, `userKindToProto`, filter converters.
- `internal/server/identity_handler.go` (+ `_test.go`) — `IdentityHandler`, all 13 RPCs, `identityError` mapper, `RegisterIdentityService`.
- `internal/server/usersbackend_stub_test.go` — server-package `UsersBackend` stub for handler unit tests.
- `internal/server/identity_integration_test.go` (`//go:build integration`) — interceptor→Cedar→handler.
- `internal/auth/mint.go` (+ `_test.go`) — `GenerateAPIKeySecret` + `FormatAPIKeyToken` + `APIKeyTokenPrefix` (package `auth`, reuse the unexported key-format constants; storage owns the prefix).
- `cmd/specgraph/identity_client.go` — `identityClient()` helper.
- `cmd/specgraph/auth.go` — `specgraph auth` root + `whoami`.
- `cmd/specgraph/auth_user.go` — `auth user list|show|set-role|delete|purge`.
- `cmd/specgraph/auth_apikey.go` — `auth api-key list|create|revoke|rotate`.
- `cmd/specgraph/auth_oidc.go` — `auth oidc list|unbind`.
- `internal/render/identity.go` (+ `_test.go`) — table renderers for User / APIKey / OIDCBinding.

**Modify:**

- `internal/auth/engine.go` — add `"manage"` to `knownVerbs` (one line, additive).
- `internal/auth/policies/base.cedar` — add the `manage → admin` policy.
- `internal/auth/actions.go` — add the 13 identity procedure→action entries to `procedureActions`.
- `cmd/specgraph/serve.go` — `server.RegisterIdentityService(mux, authStore, connectOpts...)`.

**Do NOT touch:** the CedarAuthorizer/engine evaluation logic (only `knownVerbs` + the policy file change), the resolver, the interceptor (its zero-diff property holds — identity RPCs flow through the same interceptor).

---

## Symbol-lifetime sweep

New package-level identifiers, checked against their target packages:

| Identifier | Package | Collision? |
|---|---|---|
| `IdentityHandler`, `RegisterIdentityService`, `identityError`, `userToProto`, `apiKeyToProto`, `oidcBindingToProto`, `userKindToProto`, `usersBackendStub` (test), `defaultIdentityListLimit` | `server` | none — `server` has no identity symbols today; `usersBackendStub` is test-only and distinct from the Authn plan's same-named stub (different package). `defaultIdentityListLimit` is distinct from the existing `defaultListLimit`. The timestamp helper is named `tsOrNil` (the existing server helper is `timeToProto` — no clash). |
| `GenerateAPIKeySecret`, `FormatAPIKeyToken`, `APIKeyTokenPrefix`, `mintArgon*` consts | `auth` | none — Authn provides parse/verify, not mint. Reuses `apiKeyPrefix`/`apiKeySecretLen` (same package, additive). Does NOT generate a prefix — storage owns that. |
| `manage` (knownVerbs entry) | `auth` | additive map key; `actionVerb`'s reject-unknown test still passes (`frobnicate` still unknown). |
| identity entries in `procedureActions` | `auth` | additive map keys (new procedure constants). |
| `identityClient`, `authCmd`, `authUserCmd`, `authJSON` + flag vars, … | `main` (cmd) | none — no existing `auth` command subtree. There is no global `--json`; `authJSON` is a new shared var (Task 15) bound per-command. |
| `UserList`/`APIKeyList`/`OIDCBindingList`/`userKindLabel` renderers | `render` | none. **BUT `truncate` already exists** (`render/slice.go:92`) — Task 14 REUSES it, does not redeclare it. |

No deletions in this plan; no survivor analysis needed.

---

## Task 1: Define `identity.proto` and generate code

**Files:**

- Create: `proto/specgraph/v1/identity.proto`
- Generated (committed): `gen/specgraph/v1/identity.pb.go`, `gen/specgraph/v1/specgraphv1connect/identity.connect.go`

**Covers:** N/A (schema + codegen).

- [ ] **Step 1: Write the proto**

```proto
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/specgraph/specgraph/gen/specgraph/v1;specgraphv1";

import "google/protobuf/timestamp.proto";

// IdentityService manages users, service accounts, API keys, and OIDC
// bindings. It is a GLOBAL service (not project-scoped). All RPCs except
// Whoami require the admin role (enforced by Cedar via the "manage" verb);
// Whoami is self-scoped and available to any authenticated principal.
service IdentityService {
  rpc Whoami(WhoamiRequest) returns (WhoamiResponse);

  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc CreateServiceAccount(CreateServiceAccountRequest) returns (CreateServiceAccountResponse);
  rpc UpdateUserRole(UpdateUserRoleRequest) returns (UpdateUserRoleResponse);
  rpc SoftDeleteUser(SoftDeleteUserRequest) returns (SoftDeleteUserResponse);
  rpc PurgeUser(PurgeUserRequest) returns (PurgeUserResponse);

  rpc CreateAPIKey(CreateAPIKeyRequest) returns (CreateAPIKeyResponse);
  rpc RevokeAPIKey(RevokeAPIKeyRequest) returns (RevokeAPIKeyResponse);
  rpc RotateAPIKey(RotateAPIKeyRequest) returns (RotateAPIKeyResponse);
  rpc ListAPIKeys(ListAPIKeysRequest) returns (ListAPIKeysResponse);

  rpc ListOIDCBindings(ListOIDCBindingsRequest) returns (ListOIDCBindingsResponse);
  rpc UnbindOIDC(UnbindOIDCRequest) returns (UnbindOIDCResponse);
}

enum UserKind {
  USER_KIND_UNSPECIFIED = 0;
  USER_KIND_HUMAN = 1;
  USER_KIND_SERVICE_ACCOUNT = 2;
}

// User mirrors storage.User. deleted_at is unset for active users.
message User {
  string id = 1;
  UserKind kind = 2;
  string display_name = 3;
  string email = 4;
  string role = 5;
  string owner_user_id = 6;
  bool bootstrap = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp deleted_at = 9;
}

// APIKey mirrors storage.APIKey MINUS the secret material. The PHC hash is
// never serialized; the plaintext token appears only in Create/Rotate
// responses, never here.
message APIKey {
  string id = 1;
  string user_id = 2;
  string prefix = 3;
  string role_downgrade = 4;
  string label = 5;
  google.protobuf.Timestamp expires_at = 6;
  google.protobuf.Timestamp last_used_at = 7;
  google.protobuf.Timestamp revoked_at = 8;
  google.protobuf.Timestamp created_at = 9;
}

message OIDCBinding {
  string id = 1;
  string user_id = 2;
  string issuer = 3;
  string subject = 4;
  string email_at_bind = 5;
  google.protobuf.Timestamp created_at = 6;
}

message WhoamiRequest {}
message WhoamiResponse {
  string subject = 1;
  string user_id = 2;
  string display_name = 3;
  string role = 4;
  string effective_role = 5;
  string email = 6;
  string source = 7;
}

message ListUsersRequest {
  UserKind kind = 1;          // UNSPECIFIED = all kinds
  string role = 2;            // empty = all roles
  bool include_deleted = 3;
  int32 limit = 4;            // 0 = default
  int32 offset = 5;
}
message ListUsersResponse { repeated User users = 1; }

message GetUserRequest { string id = 1; }
message GetUserResponse { User user = 1; }

message CreateServiceAccountRequest {
  string display_name = 1;
  string role = 2;
  string owner_user_id = 3;  // must reference an active Human
}
message CreateServiceAccountResponse { User user = 1; }

message UpdateUserRoleRequest {
  string id = 1;
  string role = 2;
}
message UpdateUserRoleResponse { User user = 1; }

// NOTE: Plan 4b adds a `bool force = 2;` field here plus the bootstrap-
// protection guard. 4a implements the unguarded delete.
message SoftDeleteUserRequest { string id = 1; }
message SoftDeleteUserResponse {}

// NOTE: Plan 4b adds `bool force = 2;` plus the bootstrap-purge guard.
message PurgeUserRequest { string id = 1; }
message PurgeUserResponse {}

message CreateAPIKeyRequest {
  string user_id = 1;
  string label = 2;
  string role_downgrade = 3;            // empty = no downgrade
  google.protobuf.Timestamp expires_at = 4; // unset = no expiry
}
// plaintext is the full token (spgr_sk_<prefix>_<secret>); shown ONCE.
message CreateAPIKeyResponse {
  APIKey key = 1;
  string plaintext = 2;
}

message RevokeAPIKeyRequest { string key_id = 1; }
message RevokeAPIKeyResponse {}

// RotateAPIKey revokes the old key and mints a replacement. storage's
// RotateAPIKey requires the new key's UserID + metadata (it does NOT copy
// from the old key, and there is no get-key-by-id backend method), so the
// caller supplies user_id and the new key's metadata explicitly.
message RotateAPIKeyRequest {
  string key_id = 1;
  string user_id = 2;        // owner of the key being rotated (required)
  string label = 3;          // new key's label (caller-specified)
  string role_downgrade = 4; // new key's role downgrade (empty = none)
}
message RotateAPIKeyResponse {
  APIKey key = 1;
  string plaintext = 2;
}

message ListAPIKeysRequest {
  string user_id = 1;        // empty = all users (admin)
  bool include_revoked = 2;
  int32 limit = 3;
  int32 offset = 4;
}
message ListAPIKeysResponse { repeated APIKey keys = 1; }

message ListOIDCBindingsRequest { string user_id = 1; }
message ListOIDCBindingsResponse { repeated OIDCBinding bindings = 1; }

// NOTE: Plan 4b adds `bool force = 2;` plus the last-credential guard.
message UnbindOIDCRequest { string binding_id = 1; }
message UnbindOIDCResponse {}
```

- [ ] **Step 2: Generate**

Run: `task proto`

Expected: `gen/specgraph/v1/identity.pb.go` and `gen/specgraph/v1/specgraphv1connect/identity.connect.go` appear. The connect file declares `IdentityServiceClient`, `IdentityServiceHandler`, `NewIdentityServiceClient`, `NewIdentityServiceHandler`, and procedure constants (`IdentityServiceWhoamiProcedure`, `IdentityServiceListUsersProcedure`, … `IdentityServiceUnbindOIDCProcedure`).

- [ ] **Step 3: Verify build**

Run: `go build ./...`

Expected: PASS (generated code compiles; nothing references it yet).

- [ ] **Step 4: Commit**

```bash
git add proto/specgraph/v1/identity.proto gen/specgraph/v1/identity.pb.go gen/specgraph/v1/specgraphv1connect/identity.connect.go
git commit -s -m "feat(proto): add IdentityService (users, keys, oidc, whoami)"
```

---

## Task 2: Domain→proto converters

**Files:**

- Create: `internal/server/convert_identity.go`
- Create: `internal/server/convert_identity_test.go`

**Covers:** Happy (full round of fields) + Invariant (APIKey proto never carries the PHC hash) + Boundary (nil `*time.Time` → unset; unknown kind → error).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

func TestUserToProto_HumanActive(t *testing.T) {
	created := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	u := &storage.User{
		ID: "u1", Kind: storage.KindHuman, DisplayName: "Alice",
		Email: "a@x.com", Role: "admin", CreatedAt: created,
	}
	pb, err := userToProto(u)
	require.NoError(t, err)
	require.Equal(t, "u1", pb.GetId())
	require.Equal(t, specv1.UserKind_USER_KIND_HUMAN, pb.GetKind())
	require.Equal(t, "Alice", pb.GetDisplayName())
	require.Equal(t, "admin", pb.GetRole())
	require.Equal(t, created.Unix(), pb.GetCreatedAt().GetSeconds())
	require.Nil(t, pb.GetDeletedAt(), "active user has no deleted_at")
}

func TestUserToProto_SoftDeleted(t *testing.T) {
	del := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	u := &storage.User{ID: "u2", Kind: storage.KindServiceAccount, Role: "reader", DeletedAt: &del}
	pb, err := userToProto(u)
	require.NoError(t, err)
	require.Equal(t, specv1.UserKind_USER_KIND_SERVICE_ACCOUNT, pb.GetKind())
	require.Equal(t, del.Unix(), pb.GetDeletedAt().GetSeconds())
}

func TestUserToProto_UnknownKindErrors(t *testing.T) {
	_, err := userToProto(&storage.User{ID: "u3", Kind: storage.Kind("alien")})
	require.Error(t, err)
}

func TestAPIKeyToProto_NoSecretMaterial(t *testing.T) {
	k := &storage.APIKey{
		ID: "k1", UserID: "u1", Prefix: "abc12345",
		PHCHash: "$argon2id$v=19$m=19456,t=2,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaA",
		Label:   "ci", CreatedAt: time.Now(),
	}
	pb := apiKeyToProto(k)
	require.Equal(t, "k1", pb.GetId())
	require.Equal(t, "abc12345", pb.GetPrefix())
	require.Equal(t, "ci", pb.GetLabel())
	// The proto type has no field that could carry the hash; assert the
	// prefix is the only key-identifying string exposed.
	require.NotContains(t, pb.String(), "argon2", "PHC hash must never be serialized")
}

func TestOIDCBindingToProto(t *testing.T) {
	b := &storage.OIDCBinding{ID: "b1", UserID: "u1", Issuer: "https://idp", Subject: "sub", EmailAtBind: "a@x.com", CreatedAt: time.Now()}
	pb := oidcBindingToProto(b)
	require.Equal(t, "b1", pb.GetId())
	require.Equal(t, "https://idp", pb.GetIssuer())
	require.Equal(t, "sub", pb.GetSubject())
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/server && go test -run 'TestUserToProto|TestAPIKeyToProto|TestOIDCBindingToProto' -v`

Expected: FAIL ("undefined: userToProto").

- [ ] **Step 3: Write the converters**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// tsOrNil converts a *time.Time to a proto Timestamp, returning nil for nil.
func tsOrNil(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// userKindToProto maps the storage Kind discriminator to the proto enum.
func userKindToProto(k storage.Kind) (specv1.UserKind, error) {
	switch k {
	case storage.KindHuman:
		return specv1.UserKind_USER_KIND_HUMAN, nil
	case storage.KindServiceAccount:
		return specv1.UserKind_USER_KIND_SERVICE_ACCOUNT, nil
	default:
		return specv1.UserKind_USER_KIND_UNSPECIFIED, fmt.Errorf("unknown user kind %q", k)
	}
}

// userToProto converts a storage.User to its proto representation.
func userToProto(u *storage.User) (*specv1.User, error) {
	kind, err := userKindToProto(u.Kind)
	if err != nil {
		return nil, err
	}
	return &specv1.User{
		Id:          u.ID,
		Kind:        kind,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Role:        u.Role,
		OwnerUserId: u.OwnerUserID,
		Bootstrap:   u.Bootstrap,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		DeletedAt:   tsOrNil(u.DeletedAt),
	}, nil
}

// usersToProto converts a slice, propagating the first conversion error.
func usersToProto(us []*storage.User) ([]*specv1.User, error) {
	out := make([]*specv1.User, 0, len(us))
	for _, u := range us {
		pb, err := userToProto(u)
		if err != nil {
			return nil, err
		}
		out = append(out, pb)
	}
	return out, nil
}

// apiKeyToProto converts a storage.APIKey, deliberately omitting PHCHash.
// The proto APIKey message has no field for secret material.
func apiKeyToProto(k *storage.APIKey) *specv1.APIKey {
	return &specv1.APIKey{
		Id:            k.ID,
		UserId:        k.UserID,
		Prefix:        k.Prefix,
		RoleDowngrade: k.RoleDowngrade,
		Label:         k.Label,
		ExpiresAt:     tsOrNil(k.ExpiresAt),
		LastUsedAt:    tsOrNil(k.LastUsedAt),
		RevokedAt:     tsOrNil(k.RevokedAt),
		CreatedAt:     timestamppb.New(k.CreatedAt),
	}
}

func apiKeysToProto(ks []*storage.APIKey) []*specv1.APIKey {
	out := make([]*specv1.APIKey, 0, len(ks))
	for _, k := range ks {
		out = append(out, apiKeyToProto(k))
	}
	return out
}

func oidcBindingToProto(b *storage.OIDCBinding) *specv1.OIDCBinding {
	return &specv1.OIDCBinding{
		Id:          b.ID,
		UserId:      b.UserID,
		Issuer:      b.Issuer,
		Subject:     b.Subject,
		EmailAtBind: b.EmailAtBind,
		CreatedAt:   timestamppb.New(b.CreatedAt),
	}
}

func oidcBindingsToProto(bs []*storage.OIDCBinding) []*specv1.OIDCBinding {
	out := make([]*specv1.OIDCBinding, 0, len(bs))
	for _, b := range bs {
		out = append(out, oidcBindingToProto(b))
	}
	return out
}

// userKindFromProto maps the proto enum to the storage Kind (for filters).
// UNSPECIFIED maps to empty Kind (= "all kinds" in ListUsersFilter).
func userKindFromProto(k specv1.UserKind) storage.Kind {
	switch k {
	case specv1.UserKind_USER_KIND_HUMAN:
		return storage.KindHuman
	case specv1.UserKind_USER_KIND_SERVICE_ACCOUNT:
		return storage.KindServiceAccount
	default:
		return ""
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/server && go build ./... && go test -run 'TestUserToProto|TestAPIKeyToProto|TestOIDCBindingToProto' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/convert_identity.go internal/server/convert_identity_test.go
git commit -s -m "feat(server): identity domain↔proto converters (no secret material)"
```

---

## Task 3: `UsersBackend` stub + `IdentityHandler` scaffold + `Whoami`

**Files:**

- Create: `internal/server/usersbackend_stub_test.go`
- Create: `internal/server/identity_handler.go`
- Create: `internal/server/identity_handler_test.go`

**Covers:** Happy (Whoami returns the caller's identity) + Boundary (no identity in context → Unauthenticated).

- [ ] **Step 1: Write the server-package `UsersBackend` stub**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"github.com/specgraph/specgraph/internal/storage"
)

// usersBackendStub is a hand-rolled storage.UsersBackend for IdentityHandler
// unit tests. Each field is a function the test sets; unset methods return a
// loud "unexpected call" error so tests fail fast on unplanned access.
type usersBackendStub struct {
	listUsers            func(context.Context, storage.ListUsersFilter) ([]*storage.User, error)
	getUserByID          func(context.Context, string) (*storage.User, error)
	createServiceAccount func(context.Context, *storage.User) (*storage.User, error)
	updateUserRole       func(context.Context, string, string) error
	softDeleteUser       func(context.Context, string) error
	purgeUser            func(context.Context, string) error
	createAPIKey         func(context.Context, *storage.APIKey) (*storage.APIKey, error)
	revokeAPIKey         func(context.Context, string) error
	rotateAPIKey         func(context.Context, string, *storage.APIKey) (*storage.APIKey, error)
	listAPIKeys          func(context.Context, storage.ListAPIKeysFilter) ([]*storage.APIKey, error)
	listOIDCBindings     func(context.Context, string) ([]*storage.OIDCBinding, error)
	unbindOIDC           func(context.Context, string) error
}

func (s *usersBackendStub) ListUsers(ctx context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
	if s.listUsers == nil {
		return nil, nil
	}
	return s.listUsers(ctx, f)
}
func (s *usersBackendStub) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	if s.getUserByID == nil {
		return nil, storage.ErrUserNotFound
	}
	return s.getUserByID(ctx, id)
}
func (s *usersBackendStub) CreateServiceAccount(ctx context.Context, u *storage.User) (*storage.User, error) {
	if s.createServiceAccount == nil {
		return nil, errUnexpected("CreateServiceAccount")
	}
	return s.createServiceAccount(ctx, u)
}
func (s *usersBackendStub) UpdateUserRole(ctx context.Context, id, role string) error {
	if s.updateUserRole == nil {
		return errUnexpected("UpdateUserRole")
	}
	return s.updateUserRole(ctx, id, role)
}
func (s *usersBackendStub) SoftDeleteUser(ctx context.Context, id string) error {
	if s.softDeleteUser == nil {
		return errUnexpected("SoftDeleteUser")
	}
	return s.softDeleteUser(ctx, id)
}
func (s *usersBackendStub) PurgeUser(ctx context.Context, id string) error {
	if s.purgeUser == nil {
		return errUnexpected("PurgeUser")
	}
	return s.purgeUser(ctx, id)
}
func (s *usersBackendStub) CreateAPIKey(ctx context.Context, k *storage.APIKey) (*storage.APIKey, error) {
	if s.createAPIKey == nil {
		return nil, errUnexpected("CreateAPIKey")
	}
	return s.createAPIKey(ctx, k)
}
func (s *usersBackendStub) RevokeAPIKey(ctx context.Context, id string) error {
	if s.revokeAPIKey == nil {
		return errUnexpected("RevokeAPIKey")
	}
	return s.revokeAPIKey(ctx, id)
}
func (s *usersBackendStub) RotateAPIKey(ctx context.Context, oldID string, k *storage.APIKey) (*storage.APIKey, error) {
	if s.rotateAPIKey == nil {
		return nil, errUnexpected("RotateAPIKey")
	}
	return s.rotateAPIKey(ctx, oldID, k)
}
func (s *usersBackendStub) ListAPIKeys(ctx context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	if s.listAPIKeys == nil {
		return nil, nil
	}
	return s.listAPIKeys(ctx, f)
}
func (s *usersBackendStub) ListOIDCBindings(ctx context.Context, userID string) ([]*storage.OIDCBinding, error) {
	if s.listOIDCBindings == nil {
		return nil, nil
	}
	return s.listOIDCBindings(ctx, userID)
}
func (s *usersBackendStub) UnbindOIDC(ctx context.Context, id string) error {
	if s.unbindOIDC == nil {
		return errUnexpected("UnbindOIDC")
	}
	return s.unbindOIDC(ctx, id)
}

// Methods on UsersBackend not used by IdentityHandler — resolve-path and
// bootstrap/JIT methods. They guard against unexpected handler access.
func (s *usersBackendStub) LookupAPIKeyByPrefix(context.Context, string) (*storage.APIKey, error) {
	return nil, errUnexpected("LookupAPIKeyByPrefix")
}
func (s *usersBackendStub) LookupOIDCBinding(context.Context, string, string) (*storage.OIDCBinding, error) {
	return nil, errUnexpected("LookupOIDCBinding")
}
func (s *usersBackendStub) GetBootstrap(context.Context) (*storage.User, error) {
	return nil, errUnexpected("GetBootstrap")
}
func (s *usersBackendStub) CreateHuman(context.Context, *storage.User, *storage.OIDCBinding) (*storage.User, error) {
	return nil, errUnexpected("CreateHuman")
}
func (s *usersBackendStub) JITCreateHuman(context.Context, *storage.User, *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
	return nil, nil, errUnexpected("JITCreateHuman")
}
func (s *usersBackendStub) TouchLastUsed(context.Context, string) error {
	return errUnexpected("TouchLastUsed")
}

func errUnexpected(method string) error {
	return &unexpectedCallError{method: method}
}

type unexpectedCallError struct{ method string }

func (e *unexpectedCallError) Error() string {
	return "usersBackendStub: unexpected call to " + e.method
}
```

> If `storage.UsersBackend` has methods beyond these in the merged code, add matching guard stubs (the compile-time assertion in Step 3 forces completeness). Compare against `internal/storage/users.go`.

- [ ] **Step 2: Write the handler scaffold + Whoami + failing test**

`identity_handler.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

// IdentityHandler implements the GLOBAL IdentityService. Unlike project-
// scoped handlers, it holds a storage.UsersBackend directly (identity is not
// project-scoped). Authorization is enforced upstream by the Cedar
// interceptor; the handler trusts that an admin reached a management RPC.
type IdentityHandler struct {
	users  storage.UsersBackend
	logger *slog.Logger
}

var _ specgraphv1connect.IdentityServiceHandler = (*IdentityHandler)(nil)

// Whoami returns the caller's resolved identity from the request context.
func (h *IdentityHandler) Whoami(ctx context.Context, _ *connect.Request[specv1.WhoamiRequest]) (*connect.Response[specv1.WhoamiResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	return connect.NewResponse(&specv1.WhoamiResponse{
		Subject:       id.Subject,
		UserId:        id.UserID,
		DisplayName:   id.DisplayName,
		Role:          string(id.Role),
		EffectiveRole: string(id.EffectiveRole),
		Email:         id.Email,
		Source:        id.Source,
	}), nil
}

// identityError maps storage sentinels to sanitized connect codes.
func (h *IdentityHandler) identityError(ctx context.Context, err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	switch {
	case errors.Is(err, storage.ErrUserNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	case errors.Is(err, storage.ErrAPIKeyNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("api key not found"))
	case errors.Is(err, storage.ErrOIDCBindingNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("oidc binding not found"))
	case errors.Is(err, storage.ErrBootstrapExists):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("bootstrap user already exists"))
	case errors.Is(err, storage.ErrAPIKeyPrefixExists):
		return connect.NewError(connect.CodeAborted, errors.New("api key prefix collision — retry"))
	default:
		h.logger.ErrorContext(ctx, "identityError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}

// RegisterIdentityService registers the IdentityService on the mux.
func RegisterIdentityService(mux *http.ServeMux, users storage.UsersBackend, opts ...connect.HandlerOption) {
	handler := &IdentityHandler{users: users, logger: slog.Default()}
	path, h := specgraphv1connect.NewIdentityServiceHandler(handler, opts...)
	mux.Handle(path, h)
}
```

> The compile-time assertion `var _ specgraphv1connect.IdentityServiceHandler = (*IdentityHandler)(nil)` will FAIL to build until all 13 RPC methods exist. To keep this task green, add the remaining 12 methods as stubs returning `connect.NewError(connect.CodeUnimplemented, ...)`; they are replaced with real impls in Tasks 4–10. Add this block to `identity_handler.go`:

```go
// --- stubs replaced in Tasks 4–10 ---
func (h *IdentityHandler) ListUsers(context.Context, *connect.Request[specv1.ListUsersRequest]) (*connect.Response[specv1.ListUsersResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) GetUser(context.Context, *connect.Request[specv1.GetUserRequest]) (*connect.Response[specv1.GetUserResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) CreateServiceAccount(context.Context, *connect.Request[specv1.CreateServiceAccountRequest]) (*connect.Response[specv1.CreateServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) UpdateUserRole(context.Context, *connect.Request[specv1.UpdateUserRoleRequest]) (*connect.Response[specv1.UpdateUserRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) SoftDeleteUser(context.Context, *connect.Request[specv1.SoftDeleteUserRequest]) (*connect.Response[specv1.SoftDeleteUserResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) PurgeUser(context.Context, *connect.Request[specv1.PurgeUserRequest]) (*connect.Response[specv1.PurgeUserResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) CreateAPIKey(context.Context, *connect.Request[specv1.CreateAPIKeyRequest]) (*connect.Response[specv1.CreateAPIKeyResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) RevokeAPIKey(context.Context, *connect.Request[specv1.RevokeAPIKeyRequest]) (*connect.Response[specv1.RevokeAPIKeyResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) RotateAPIKey(context.Context, *connect.Request[specv1.RotateAPIKeyRequest]) (*connect.Response[specv1.RotateAPIKeyResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) ListAPIKeys(context.Context, *connect.Request[specv1.ListAPIKeysRequest]) (*connect.Response[specv1.ListAPIKeysResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) ListOIDCBindings(context.Context, *connect.Request[specv1.ListOIDCBindingsRequest]) (*connect.Response[specv1.ListOIDCBindingsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
func (h *IdentityHandler) UnbindOIDC(context.Context, *connect.Request[specv1.UnbindOIDCRequest]) (*connect.Response[specv1.UnbindOIDCResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
```

`identity_handler_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/auth"
)

func newTestIdentityHandler(stub *usersBackendStub) *IdentityHandler {
	return &IdentityHandler{users: stub, logger: slog.Default()}
}

func TestWhoami_ReturnsContextIdentity(t *testing.T) {
	h := newTestIdentityHandler(&usersBackendStub{})
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{
		Subject: "apikey:k1", UserID: "u1", DisplayName: "Alice",
		Role: auth.RoleAdmin, EffectiveRole: auth.RoleAdmin, Email: "a@x.com", Source: "apikey",
	})
	resp, err := h.Whoami(ctx, connect.NewRequest(&specv1.WhoamiRequest{}))
	require.NoError(t, err)
	require.Equal(t, "u1", resp.Msg.GetUserId())
	require.Equal(t, "admin", resp.Msg.GetRole())
	require.Equal(t, "apikey", resp.Msg.GetSource())
}

func TestWhoami_NoIdentityUnauthenticated(t *testing.T) {
	h := newTestIdentityHandler(&usersBackendStub{})
	_, err := h.Whoami(context.Background(), connect.NewRequest(&specv1.WhoamiRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
```

> `auth.WithIdentity(ctx, *Identity) context.Context` is the existing context setter (see `internal/auth/context.go`, used by the interceptor). If it is named differently in the merged code, use that name.

- [ ] **Step 3: Run the tests**

Run: `cd internal/server && go build ./... && go test -run 'TestWhoami' -v`

Expected: PASS (the `var _` assertion compiles because all 13 methods exist as stubs).

- [ ] **Step 4: Whole-project build**

Run: `go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_handler_test.go internal/server/usersbackend_stub_test.go
git commit -s -m "feat(server): IdentityHandler scaffold + Whoami + UsersBackend test stub"
```

---

## Task 4: `ListUsers` + `GetUser`

**Files:**

- Modify: `internal/server/identity_handler.go` (replace the two stubs)
- Modify: `internal/server/identity_handler_test.go`

**Covers:** Happy (list maps rows; get returns one) + Boundary (GetUser NotFound; empty list).

- [ ] **Step 1: Write the failing test**

Append to `identity_handler_test.go`:

```go
import "github.com/specgraph/specgraph/internal/storage" // add to imports
import "time"

func TestListUsers_MapsRows(t *testing.T) {
	stub := &usersBackendStub{
		listUsers: func(_ context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
			require.Equal(t, storage.KindHuman, f.Kind)
			require.True(t, f.IncludeDeleted)
			return []*storage.User{
				{ID: "u1", Kind: storage.KindHuman, Role: "admin", CreatedAt: time.Now()},
			}, nil
		},
	}
	h := newTestIdentityHandler(stub)
	resp, err := h.ListUsers(context.Background(), connect.NewRequest(&specv1.ListUsersRequest{
		Kind: specv1.UserKind_USER_KIND_HUMAN, IncludeDeleted: true,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetUsers(), 1)
	require.Equal(t, "u1", resp.Msg.GetUsers()[0].GetId())
}

func TestGetUser_NotFound(t *testing.T) {
	stub := &usersBackendStub{
		getUserByID: func(context.Context, string) (*storage.User, error) {
			return nil, storage.ErrUserNotFound
		},
	}
	h := newTestIdentityHandler(stub)
	_, err := h.GetUser(context.Background(), connect.NewRequest(&specv1.GetUserRequest{Id: "missing"}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetUser_Found(t *testing.T) {
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			require.Equal(t, "u1", id)
			return &storage.User{ID: "u1", Kind: storage.KindHuman, Role: "reader", CreatedAt: time.Now()}, nil
		},
	}
	h := newTestIdentityHandler(stub)
	resp, err := h.GetUser(context.Background(), connect.NewRequest(&specv1.GetUserRequest{Id: "u1"}))
	require.NoError(t, err)
	require.Equal(t, "reader", resp.Msg.GetUser().GetRole())
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/server && go test -run 'TestListUsers|TestGetUser' -v`

Expected: FAIL — stubs return Unimplemented.

- [ ] **Step 3: Replace the stubs**

In `identity_handler.go`, delete the `ListUsers` and `GetUser` stub methods and add:

```go
const defaultIdentityListLimit = 100

// ListUsers lists users matching the filter (admin-gated upstream).
func (h *IdentityHandler) ListUsers(ctx context.Context, req *connect.Request[specv1.ListUsersRequest]) (*connect.Response[specv1.ListUsersResponse], error) {
	msg := req.Msg
	limit := int(msg.GetLimit())
	if limit == 0 {
		limit = defaultIdentityListLimit
	}
	users, err := h.users.ListUsers(ctx, storage.ListUsersFilter{
		Kind:           userKindFromProto(msg.GetKind()),
		Role:           msg.GetRole(),
		IncludeDeleted: msg.GetIncludeDeleted(),
		Limit:          limit,
		Offset:         int(msg.GetOffset()),
	})
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	pbs, err := usersToProto(users)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.ListUsersResponse{Users: pbs}), nil
}

// GetUser returns a single user by ID.
func (h *IdentityHandler) GetUser(ctx context.Context, req *connect.Request[specv1.GetUserRequest]) (*connect.Response[specv1.GetUserResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	u, err := h.users.GetUserByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	pb, err := userToProto(u)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.GetUserResponse{User: pb}), nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/server && go build ./... && go test -run 'TestListUsers|TestGetUser|TestWhoami' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_handler_test.go
git commit -s -m "feat(server): implement ListUsers and GetUser RPCs"
```

---

## Task 5: `CreateServiceAccount` + `UpdateUserRole`

**Files:**

- Modify: `internal/server/identity_handler.go`
- Modify: `internal/server/identity_handler_test.go`

**Covers:** Happy (create SA returns row; update role returns updated) + Boundary (missing display_name / id → InvalidArgument; UpdateUserRole NotFound).

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestCreateServiceAccount_Happy(t *testing.T) {
	stub := &usersBackendStub{
		createServiceAccount: func(_ context.Context, u *storage.User) (*storage.User, error) {
			require.Equal(t, storage.KindServiceAccount, u.Kind)
			require.Equal(t, "ci-bot", u.DisplayName)
			require.Equal(t, "owner-1", u.OwnerUserID)
			u.ID = "sa1"
			u.CreatedAt = time.Now()
			return u, nil
		},
	}
	h := newTestIdentityHandler(stub)
	resp, err := h.CreateServiceAccount(context.Background(), connect.NewRequest(&specv1.CreateServiceAccountRequest{
		DisplayName: "ci-bot", Role: "writer", OwnerUserId: "owner-1",
	}))
	require.NoError(t, err)
	require.Equal(t, "sa1", resp.Msg.GetUser().GetId())
	require.Equal(t, specv1.UserKind_USER_KIND_SERVICE_ACCOUNT, resp.Msg.GetUser().GetKind())
}

func TestCreateServiceAccount_RequiresDisplayName(t *testing.T) {
	h := newTestIdentityHandler(&usersBackendStub{})
	_, err := h.CreateServiceAccount(context.Background(), connect.NewRequest(&specv1.CreateServiceAccountRequest{Role: "writer"}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateUserRole_Happy(t *testing.T) {
	stub := &usersBackendStub{
		updateUserRole: func(_ context.Context, id, role string) error {
			require.Equal(t, "u1", id)
			require.Equal(t, "admin", role)
			return nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Kind: storage.KindHuman, Role: "admin", CreatedAt: time.Now()}, nil
		},
	}
	h := newTestIdentityHandler(stub)
	resp, err := h.UpdateUserRole(context.Background(), connect.NewRequest(&specv1.UpdateUserRoleRequest{Id: "u1", Role: "admin"}))
	require.NoError(t, err)
	require.Equal(t, "admin", resp.Msg.GetUser().GetRole())
}

func TestUpdateUserRole_NotFound(t *testing.T) {
	stub := &usersBackendStub{
		updateUserRole: func(context.Context, string, string) error { return storage.ErrUserNotFound },
	}
	h := newTestIdentityHandler(stub)
	_, err := h.UpdateUserRole(context.Background(), connect.NewRequest(&specv1.UpdateUserRoleRequest{Id: "x", Role: "admin"}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/server && go test -run 'TestCreateServiceAccount|TestUpdateUserRole' -v`

Expected: FAIL (Unimplemented).

- [ ] **Step 3: Replace the stubs**

Delete the `CreateServiceAccount` and `UpdateUserRole` stubs; add:

```go
// CreateServiceAccount creates a machine identity owned by a Human.
func (h *IdentityHandler) CreateServiceAccount(ctx context.Context, req *connect.Request[specv1.CreateServiceAccountRequest]) (*connect.Response[specv1.CreateServiceAccountResponse], error) {
	msg := req.Msg
	if msg.GetDisplayName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("display_name is required"))
	}
	if msg.GetRole() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}
	if msg.GetOwnerUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("owner_user_id is required"))
	}
	u, err := h.users.CreateServiceAccount(ctx, &storage.User{
		Kind:        storage.KindServiceAccount,
		DisplayName: msg.GetDisplayName(),
		Role:        msg.GetRole(),
		OwnerUserID: msg.GetOwnerUserId(),
	})
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	pb, err := userToProto(u)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.CreateServiceAccountResponse{User: pb}), nil
}

// UpdateUserRole sets the role then returns the refreshed user.
func (h *IdentityHandler) UpdateUserRole(ctx context.Context, req *connect.Request[specv1.UpdateUserRoleRequest]) (*connect.Response[specv1.UpdateUserRoleResponse], error) {
	msg := req.Msg
	if msg.GetId() == "" || msg.GetRole() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id and role are required"))
	}
	if err := h.users.UpdateUserRole(ctx, msg.GetId(), msg.GetRole()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	u, err := h.users.GetUserByID(ctx, msg.GetId())
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	pb, err := userToProto(u)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.UpdateUserRoleResponse{User: pb}), nil
}
```

> Role validity (is `msg.Role` a known role?) is intentionally NOT checked here — the storage layer documents role validation as the caller's responsibility, and the canonical known-role set lives in `auth.KnownRolesFrom(cfg.Auth.Roles)` (Cedar plan), which the handler does not have. Validating against config is a follow-up; an unknown role simply authorizes nothing under Cedar (default-deny), which is safe.

- [ ] **Step 4: Run the tests**

Run: `cd internal/server && go build ./... && go test -run 'TestCreateServiceAccount|TestUpdateUserRole' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_handler_test.go
git commit -s -m "feat(server): implement CreateServiceAccount and UpdateUserRole RPCs"
```

---

## Task 6: `SoftDeleteUser` + `PurgeUser` (unguarded; 4b adds protections)

**Files:**

- Modify: `internal/server/identity_handler.go`
- Modify: `internal/server/identity_handler_test.go`

**Covers:** Happy (both call through and return empty response) + Boundary (missing id → InvalidArgument; NotFound mapping).

> Scope note: these are functional but UNGUARDED in 4a. The bootstrap-protection guards (refuse to delete/purge the bootstrap user without a `force` flag) and the `force` proto fields are added by **Plan 4b**. 4a and 4b land together; do not ship 4a to production without 4b.

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestSoftDeleteUser_Happy(t *testing.T) {
	called := false
	stub := &usersBackendStub{softDeleteUser: func(_ context.Context, id string) error {
		require.Equal(t, "u1", id)
		called = true
		return nil
	}}
	h := newTestIdentityHandler(stub)
	_, err := h.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: "u1"}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestSoftDeleteUser_RequiresID(t *testing.T) {
	h := newTestIdentityHandler(&usersBackendStub{})
	_, err := h.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPurgeUser_NotFound(t *testing.T) {
	stub := &usersBackendStub{purgeUser: func(context.Context, string) error { return storage.ErrUserNotFound }}
	h := newTestIdentityHandler(stub)
	_, err := h.PurgeUser(context.Background(), connect.NewRequest(&specv1.PurgeUserRequest{Id: "x"}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/server && go test -run 'TestSoftDeleteUser|TestPurgeUser' -v`

Expected: FAIL (Unimplemented).

- [ ] **Step 3: Replace the stubs**

```go
// SoftDeleteUser soft-deletes a user and revokes their keys. UNGUARDED in
// 4a; 4b adds the bootstrap-protection force flag.
func (h *IdentityHandler) SoftDeleteUser(ctx context.Context, req *connect.Request[specv1.SoftDeleteUserRequest]) (*connect.Response[specv1.SoftDeleteUserResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if err := h.users.SoftDeleteUser(ctx, req.Msg.GetId()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.SoftDeleteUserResponse{}), nil
}

// PurgeUser hard-deletes a user. UNGUARDED in 4a; 4b adds the bootstrap-
// protection force flag.
func (h *IdentityHandler) PurgeUser(ctx context.Context, req *connect.Request[specv1.PurgeUserRequest]) (*connect.Response[specv1.PurgeUserResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if err := h.users.PurgeUser(ctx, req.Msg.GetId()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.PurgeUserResponse{}), nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/server && go build ./... && go test -run 'TestSoftDeleteUser|TestPurgeUser' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_handler_test.go
git commit -s -m "feat(server): implement SoftDeleteUser and PurgeUser RPCs (unguarded; 4b hardens)"
```

---

## Task 7: API-key minting helpers (package `auth`)

> **Design reconciliation (was a review Critical):** the Storage plan's `AuthStore.CreateAPIKey`/`RotateAPIKey` **own prefix generation** — they call `s.genPrefix()` (8 random base32 chars) and IGNORE any `Prefix` the caller set, returning the key with the storage-generated prefix. So the minting helper must NOT generate a prefix or assemble the token before persisting. Instead: generate only the secret + PHC hash, persist (storage assigns the prefix), then assemble the token from the **storage-returned** prefix. Two helpers express this: `GenerateAPIKeySecret()` (secret + hash) and `FormatAPIKeyToken(prefix, secret)`. They live in package `auth` so they reuse the unexported format constants — zero drift from the verifier.

**Files:**

- Create: `internal/auth/mint.go`
- Create: `internal/auth/mint_test.go`
- Create: `internal/auth/mint_integration_test.go` (`//go:build integration`)

**Covers:** Happy (secret length 32; token shape) + Invariant (PHC parses; storage-prefix round-trips through the real resolver) + Boundary (distinct secrets per call).

- [ ] **Step 1: Write the failing unit test**

`mint_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKeySecret_Shape(t *testing.T) {
	secret, phc, err := GenerateAPIKeySecret()
	require.NoError(t, err)
	require.Len(t, secret, apiKeySecretLen, "secret length matches the resolver's parser expectation")
	require.True(t, strings.HasPrefix(phc, "$argon2id$"), "PHC format")
}

func TestGenerateAPIKeySecret_DistinctEachCall(t *testing.T) {
	s1, h1, err := GenerateAPIKeySecret()
	require.NoError(t, err)
	s2, h2, err := GenerateAPIKeySecret()
	require.NoError(t, err)
	require.NotEqual(t, s1, s2)
	require.NotEqual(t, h1, h2)
}

func TestFormatAPIKeyToken(t *testing.T) {
	// The token is assembled from the STORAGE-assigned prefix + the secret.
	token := FormatAPIKeyToken("abc12345", "thirtytwocharsecretthirtytwocha0")
	require.True(t, strings.HasPrefix(token, apiKeyPrefix))
	require.Equal(t, apiKeyPrefix+"abc12345_thirtytwocharsecretthirtytwocha0", token)
	require.Equal(t, apiKeyPrefix, APIKeyTokenPrefix())
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestGenerateAPIKeySecret|TestFormatAPIKeyToken' -v`

Expected: FAIL ("undefined: GenerateAPIKeySecret").

- [ ] **Step 3: Write the impl**

`mint.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters — MUST match the resolver's verifier (identitystore.go
// / the Authn plan). Kept here as named constants; the round-trip integration
// test (mint_integration_test.go) is the guarantee that mint and verify agree.
const (
	mintArgonTime    = 2
	mintArgonMemory  = 19456
	mintArgonThreads = 1
	mintArgonKeyLen  = 32
	mintArgonSaltLen = 16
)

// GenerateAPIKeySecret generates a fresh API-key secret and its argon2id PHC
// hash. It deliberately does NOT generate a prefix or assemble a token — the
// storage layer (AuthStore.CreateAPIKey/RotateAPIKey) owns prefix generation
// and ignores any caller-supplied prefix. The caller persists the hash, reads
// back the storage-assigned prefix, then calls FormatAPIKeyToken to assemble
// the credential.
//
//   - secret: the 32-char secret (hex of 16 bytes; matches apiKeySecretLen).
//   - phcHash: the argon2id PHC string (store on APIKey.PHCHash; the secret is
//     never persisted).
func GenerateAPIKeySecret() (secret, phcHash string, err error) {
	secretBytes := make([]byte, apiKeySecretLen/2) // hex doubles length
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", fmt.Errorf("generate secret: %w", err)
	}
	secret = hex.EncodeToString(secretBytes)

	salt := make([]byte, mintArgonSaltLen)
	if _, err = rand.Read(salt); err != nil {
		return "", "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(secret), salt, mintArgonTime, mintArgonMemory, mintArgonThreads, mintArgonKeyLen)
	phcHash = fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		mintArgonMemory, mintArgonTime, mintArgonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash))
	return secret, phcHash, nil
}

// FormatAPIKeyToken assembles the full credential token from the prefix the
// storage layer assigned and the secret from GenerateAPIKeySecret. The result
// is what the resolver parses: spgr_sk_<prefix>_<secret>.
func FormatAPIKeyToken(prefix, secret string) string {
	return apiKeyPrefix + prefix + "_" + secret
}

// APIKeyTokenPrefix returns the vendor prefix all API-key tokens carry
// (spgr_sk_). Exported so other packages can recognize a token without
// depending on the unexported constant.
func APIKeyTokenPrefix() string { return apiKeyPrefix }
```

> `apiKeyPrefix`, `apiKeySecretLen` are the Authn plan's unexported constants in `identitystore.go`. If `apiKeySecretLen` is not even (so `/2` for hex doesn't divide cleanly), generate `ceil(n/2)` bytes and slice the hex string to length `n`. Confirm the value (32) is even — it is. Note we do NOT reference `apiKeyPrefixLen` here — storage owns the prefix.

- [ ] **Step 4: Write the round-trip integration test**

This is the contract that proves the storage-owned prefix + caller secret + the resolver's verifier all agree.

`mint_integration_test.go`:

```go
//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

// TestAPIKeyMint_RoundTrip proves the full issuance path: generate a secret,
// persist WITHOUT a prefix (storage assigns one), assemble the token from the
// STORAGE-RETURNED prefix, and verify it resolves. This is the contract that
// catches any drift between minting and the resolver's verifier.
func TestAPIKeyMint_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx) // from the postgres test harness
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })

	_, err = pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)

	user, err := authStore.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "minter", Role: "writer",
	}, nil)
	require.NoError(t, err)

	secret, phc, err := auth.GenerateAPIKeySecret()
	require.NoError(t, err)
	// Persist WITHOUT a prefix — storage assigns it.
	created, err := authStore.CreateAPIKey(ctx, &storage.APIKey{UserID: user.ID, PHCHash: phc})
	require.NoError(t, err)
	require.NotEmpty(t, created.Prefix, "storage assigns the prefix")

	token := auth.FormatAPIKeyToken(created.Prefix, secret)

	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{Users: authStore, Tracker: tracker})
	require.NoError(t, err)

	id, err := resolver.Resolve(ctx, token)
	require.NoError(t, err, "a token built from the storage prefix + minted secret must resolve")
	require.Equal(t, user.ID, id.UserID)
}
```

> Reuses the integration harness: the **exported** `postgrestest.SharedPool(t, ctx)` (Storage plan Task 32; the in-package `sharedTestPool` is not importable from `auth_test`), plus `usagetracker`, `postgres.NewAuth`, `NewIdentityStore`. Match the actual helper names from the merged tests.

- [ ] **Step 5: Run the tests**

```bash
cd internal/auth && go build ./... && go test -run 'TestGenerateAPIKeySecret|TestFormatAPIKeyToken' -v
go test -tags integration -run TestAPIKeyMint_RoundTrip -v
```

Expected: PASS (unit + integration round-trip).

- [ ] **Step 6: Commit**

```bash
git add internal/auth/mint.go internal/auth/mint_test.go internal/auth/mint_integration_test.go
git commit -s -m "feat(auth): add GenerateAPIKeySecret + FormatAPIKeyToken (storage owns prefix)"
```

---

## Task 8: `CreateAPIKey` + `RotateAPIKey` (return plaintext once)

**Files:**

- Modify: `internal/server/identity_handler.go`
- Modify: `internal/server/identity_handler_test.go`

**Covers:** Happy (create/rotate return key + plaintext) + Invariant (plaintext present in response, absent from the persisted key; token is assembled from the storage-assigned prefix) + Boundary (missing user_id / key_id → InvalidArgument).

- [ ] **Step 1: Write the failing test**

Append (the test imports `auth` to assert the vendor prefix via `auth.APIKeyTokenPrefix()`, defined in Task 7). Note the create/rotate flow: the handler persists WITHOUT a prefix, storage assigns one, and the token is built from the storage-returned prefix — so the stub must populate `k.Prefix` to mimic storage:

```go
import "github.com/specgraph/specgraph/internal/auth" // add to imports

func TestCreateAPIKey_ReturnsPlaintextOnce(t *testing.T) {
	var stored *storage.APIKey
	stub := &usersBackendStub{
		createAPIKey: func(_ context.Context, k *storage.APIKey) (*storage.APIKey, error) {
			// storage owns the prefix: the caller passes none, storage assigns it.
			require.Empty(t, k.Prefix, "handler must not pre-set the prefix")
			require.True(t, strings.HasPrefix(k.PHCHash, "$argon2id$"))
			require.Equal(t, "u1", k.UserID)
			k.ID = "k1"
			k.Prefix = "stor1234" // simulate storage-assigned prefix
			k.CreatedAt = time.Now()
			stored = k
			return k, nil
		},
	}
	h := newTestIdentityHandler(stub)
	resp, err := h.CreateAPIKey(context.Background(), connect.NewRequest(&specv1.CreateAPIKeyRequest{
		UserId: "u1", Label: "ci",
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetPlaintext(), "plaintext returned once")
	require.True(t, strings.HasPrefix(resp.Msg.GetPlaintext(), auth.APIKeyTokenPrefix()))
	require.Equal(t, "k1", resp.Msg.GetKey().GetId())
	require.Equal(t, "stor1234", resp.Msg.GetKey().GetPrefix())
	// The plaintext token embeds the STORAGE-assigned prefix.
	require.Contains(t, resp.Msg.GetPlaintext(), stored.Prefix)
}

func TestCreateAPIKey_RequiresUserID(t *testing.T) {
	h := newTestIdentityHandler(&usersBackendStub{})
	_, err := h.CreateAPIKey(context.Background(), connect.NewRequest(&specv1.CreateAPIKeyRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRotateAPIKey_ReturnsNewPlaintext(t *testing.T) {
	stub := &usersBackendStub{
		rotateAPIKey: func(_ context.Context, oldID string, k *storage.APIKey) (*storage.APIKey, error) {
			require.Equal(t, "old1", oldID)
			// storage needs UserID + metadata on newKey; the handler supplies them.
			require.Equal(t, "u1", k.UserID)
			require.Empty(t, k.Prefix, "handler must not pre-set the prefix")
			k.ID = "new1"
			k.Prefix = "stor5678" // simulate storage-assigned prefix
			k.CreatedAt = time.Now()
			return k, nil
		},
	}
	h := newTestIdentityHandler(stub)
	resp, err := h.RotateAPIKey(context.Background(), connect.NewRequest(&specv1.RotateAPIKeyRequest{
		KeyId: "old1", UserId: "u1", Label: "ci",
	}))
	require.NoError(t, err)
	require.Equal(t, "new1", resp.Msg.GetKey().GetId())
	require.NotEmpty(t, resp.Msg.GetPlaintext())
	require.Contains(t, resp.Msg.GetPlaintext(), "stor5678")
}

func TestRotateAPIKey_RequiresKeyAndUser(t *testing.T) {
	h := newTestIdentityHandler(&usersBackendStub{})
	_, err := h.RotateAPIKey(context.Background(), connect.NewRequest(&specv1.RotateAPIKeyRequest{KeyId: "old1"}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "user_id required for rotate")
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/server && go test -run 'TestCreateAPIKey|TestRotateAPIKey' -v`

Expected: FAIL (Unimplemented).

- [ ] **Step 3: Replace the handler stubs**

In `identity_handler.go` delete the `CreateAPIKey`/`RotateAPIKey` stubs and add. The flow honors storage's prefix-ownership: generate the secret + hash, persist WITHOUT a prefix, then build the token from the storage-returned prefix.

```go
// CreateAPIKey generates a new key for a user and returns the plaintext token
// exactly once. The secret is never persisted (only the PHC hash); storage
// assigns the prefix, and the token is assembled from it.
func (h *IdentityHandler) CreateAPIKey(ctx context.Context, req *connect.Request[specv1.CreateAPIKeyRequest]) (*connect.Response[specv1.CreateAPIKeyResponse], error) {
	msg := req.Msg
	if msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	k := &storage.APIKey{
		UserID:        msg.GetUserId(),
		PHCHash:       phc, // no Prefix — storage assigns it
		Label:         msg.GetLabel(),
		RoleDowngrade: msg.GetRoleDowngrade(),
	}
	if msg.GetExpiresAt() != nil {
		exp := msg.GetExpiresAt().AsTime()
		k.ExpiresAt = &exp
	}
	created, err := h.users.CreateAPIKey(ctx, k)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.CreateAPIKeyResponse{
		Key:       apiKeyToProto(created),
		Plaintext: auth.FormatAPIKeyToken(created.Prefix, secret),
	}), nil
}

// RotateAPIKey revokes the old key and generates a replacement. storage's
// RotateAPIKey requires the new key's UserID + metadata (it does not copy from
// the old key, and there is no get-key-by-id method), so the request carries
// user_id and the new key's label/role_downgrade explicitly.
func (h *IdentityHandler) RotateAPIKey(ctx context.Context, req *connect.Request[specv1.RotateAPIKeyRequest]) (*connect.Response[specv1.RotateAPIKeyResponse], error) {
	msg := req.Msg
	if msg.GetKeyId() == "" || msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key_id and user_id are required"))
	}
	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	rotated, err := h.users.RotateAPIKey(ctx, msg.GetKeyId(), &storage.APIKey{
		UserID:        msg.GetUserId(),
		PHCHash:       phc, // no Prefix — storage assigns it
		Label:         msg.GetLabel(),
		RoleDowngrade: msg.GetRoleDowngrade(),
	})
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.RotateAPIKeyResponse{
		Key:       apiKeyToProto(rotated),
		Plaintext: auth.FormatAPIKeyToken(rotated.Prefix, secret),
	}), nil
}
```

`identity_handler.go` already imports `auth` (for `IdentityFromContext`, Task 3), so no import change is needed.

- [ ] **Step 4: Run the tests**

Run: `cd internal/server && go build ./... && go test -run 'TestCreateAPIKey|TestRotateAPIKey' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_handler_test.go
git commit -s -m "feat(server): implement CreateAPIKey and RotateAPIKey (storage-owned prefix)"
```

---

## Task 9: `RevokeAPIKey` + `ListAPIKeys`

**Files:**

- Modify: `internal/server/identity_handler.go`
- Modify: `internal/server/identity_handler_test.go`

**Covers:** Happy (revoke calls through; list maps + passes filter) + Boundary (missing key_id → InvalidArgument; list excludes revoked unless requested).

- [ ] **Step 1: Write the failing test**

```go
func TestRevokeAPIKey_Happy(t *testing.T) {
	called := false
	stub := &usersBackendStub{revokeAPIKey: func(_ context.Context, id string) error {
		require.Equal(t, "k1", id)
		called = true
		return nil
	}}
	h := newTestIdentityHandler(stub)
	_, err := h.RevokeAPIKey(context.Background(), connect.NewRequest(&specv1.RevokeAPIKeyRequest{KeyId: "k1"}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestRevokeAPIKey_RequiresID(t *testing.T) {
	h := newTestIdentityHandler(&usersBackendStub{})
	_, err := h.RevokeAPIKey(context.Background(), connect.NewRequest(&specv1.RevokeAPIKeyRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestListAPIKeys_PassesFilter(t *testing.T) {
	stub := &usersBackendStub{
		listAPIKeys: func(_ context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
			require.Equal(t, "u1", f.UserID)
			require.True(t, f.IncludeRevoked)
			return []*storage.APIKey{{ID: "k1", UserID: "u1", Prefix: "abc12345", CreatedAt: time.Now()}}, nil
		},
	}
	h := newTestIdentityHandler(stub)
	resp, err := h.ListAPIKeys(context.Background(), connect.NewRequest(&specv1.ListAPIKeysRequest{
		UserId: "u1", IncludeRevoked: true,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetKeys(), 1)
	require.Equal(t, "abc12345", resp.Msg.GetKeys()[0].GetPrefix())
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/server && go test -run 'TestRevokeAPIKey|TestListAPIKeys' -v`

Expected: FAIL (Unimplemented).

- [ ] **Step 3: Replace the stubs**

```go
// RevokeAPIKey marks a key revoked (idempotent).
func (h *IdentityHandler) RevokeAPIKey(ctx context.Context, req *connect.Request[specv1.RevokeAPIKeyRequest]) (*connect.Response[specv1.RevokeAPIKeyResponse], error) {
	if req.Msg.GetKeyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key_id is required"))
	}
	if err := h.users.RevokeAPIKey(ctx, req.Msg.GetKeyId()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.RevokeAPIKeyResponse{}), nil
}

// ListAPIKeys lists keys, optionally scoped to a user and including revoked.
func (h *IdentityHandler) ListAPIKeys(ctx context.Context, req *connect.Request[specv1.ListAPIKeysRequest]) (*connect.Response[specv1.ListAPIKeysResponse], error) {
	msg := req.Msg
	limit := int(msg.GetLimit())
	if limit == 0 {
		limit = defaultIdentityListLimit
	}
	keys, err := h.users.ListAPIKeys(ctx, storage.ListAPIKeysFilter{
		UserID:         msg.GetUserId(),
		IncludeRevoked: msg.GetIncludeRevoked(),
		Limit:          limit,
		Offset:         int(msg.GetOffset()),
	})
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.ListAPIKeysResponse{Keys: apiKeysToProto(keys)}), nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/server && go build ./... && go test -run 'TestRevokeAPIKey|TestListAPIKeys' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_handler_test.go
git commit -s -m "feat(server): implement RevokeAPIKey and ListAPIKeys RPCs"
```

---

## Task 10: `ListOIDCBindings` + `UnbindOIDC` (unguarded; 4b adds last-credential guard)

**Files:**

- Modify: `internal/server/identity_handler.go`
- Modify: `internal/server/identity_handler_test.go`

**Covers:** Happy (list maps; unbind calls through) + Boundary (missing binding_id → InvalidArgument; NotFound mapping).

> Scope note: `UnbindOIDC` is unguarded in 4a. The last-credential protection (refuse to unbind a user's only credential without `force`) and the `force` proto field are **Plan 4b**.

- [ ] **Step 1: Write the failing test**

```go
func TestListOIDCBindings_Happy(t *testing.T) {
	stub := &usersBackendStub{
		listOIDCBindings: func(_ context.Context, userID string) ([]*storage.OIDCBinding, error) {
			require.Equal(t, "u1", userID)
			return []*storage.OIDCBinding{{ID: "b1", UserID: "u1", Issuer: "https://idp", Subject: "s", CreatedAt: time.Now()}}, nil
		},
	}
	h := newTestIdentityHandler(stub)
	resp, err := h.ListOIDCBindings(context.Background(), connect.NewRequest(&specv1.ListOIDCBindingsRequest{UserId: "u1"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetBindings(), 1)
	require.Equal(t, "b1", resp.Msg.GetBindings()[0].GetId())
}

func TestUnbindOIDC_Happy(t *testing.T) {
	called := false
	stub := &usersBackendStub{unbindOIDC: func(_ context.Context, id string) error {
		require.Equal(t, "b1", id)
		called = true
		return nil
	}}
	h := newTestIdentityHandler(stub)
	_, err := h.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1"}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestUnbindOIDC_NotFound(t *testing.T) {
	stub := &usersBackendStub{unbindOIDC: func(context.Context, string) error { return storage.ErrOIDCBindingNotFound }}
	h := newTestIdentityHandler(stub)
	_, err := h.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "x"}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/server && go test -run 'TestListOIDCBindings|TestUnbindOIDC' -v`

Expected: FAIL (Unimplemented).

- [ ] **Step 3: Replace the stubs**

```go
// ListOIDCBindings lists a user's OIDC bindings.
func (h *IdentityHandler) ListOIDCBindings(ctx context.Context, req *connect.Request[specv1.ListOIDCBindingsRequest]) (*connect.Response[specv1.ListOIDCBindingsResponse], error) {
	if req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	bindings, err := h.users.ListOIDCBindings(ctx, req.Msg.GetUserId())
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.ListOIDCBindingsResponse{Bindings: oidcBindingsToProto(bindings)}), nil
}

// UnbindOIDC removes an OIDC binding. UNGUARDED in 4a; 4b adds the last-
// credential force flag.
func (h *IdentityHandler) UnbindOIDC(ctx context.Context, req *connect.Request[specv1.UnbindOIDCRequest]) (*connect.Response[specv1.UnbindOIDCResponse], error) {
	if req.Msg.GetBindingId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("binding_id is required"))
	}
	if err := h.users.UnbindOIDC(ctx, req.Msg.GetBindingId()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.UnbindOIDCResponse{}), nil
}
```

- [ ] **Step 4: Run the tests + whole-project**

```bash
cd internal/server && go build ./... && go test ./...
go build ./... && go test ./...
```

Expected: PASS. All 13 RPCs are now real; no Unimplemented stubs remain.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_handler_test.go
git commit -s -m "feat(server): implement ListOIDCBindings and UnbindOIDC RPCs"
```

---

## Task 11: Cedar — add the `manage` verb and admin policy

**Files:**

- Modify: `internal/auth/engine.go` (add `"manage"` to `knownVerbs`)
- Modify: `internal/auth/policies/base.cedar` (add the manage→admin policy)
- Modify: `internal/auth/engine_evaluate_test.go` (add manage matrix cases)

**Covers:** Happy (admin allowed manage) + Invariant (writer/reader denied manage; manage doesn't leak through read/write/delete).

- [ ] **Step 1: Write the failing test**

Append to `internal/auth/engine_evaluate_test.go`. First extend the shared `basePolicies` const used by the engine tests to include the manage policy — but that const lives in `engine_test.go` (Task 6 of the Cedar plan). Rather than edit the shared const, add a dedicated test with its own policy text:

```go
const basePoliciesWithManage = basePolicies + `
permit (principal, action in SpecGraph::Action::"manage", resource)
when { principal has role && principal.role == "admin" };
`

func TestEvaluate_ManageVerbAdminOnly(t *testing.T) {
	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{stubSource{name: "test", docs: []auth.PolicyDocument{{Source: "test:base.cedar", Text: basePoliciesWithManage}}}},
		[]string{"spec.read", "spec.write", "graph.delete", "user.manage"})
	require.NoError(t, err)

	check := func(role auth.Role) bool {
		dec, evalErr := eng.Evaluate(context.Background(), auth.EvalRequest{
			Identity: &auth.Identity{UserID: "u1", EffectiveRole: role, Role: role},
			Action:   "user.manage",
			Resource: auth.ResourceRef{Type: "user"},
		})
		require.NoError(t, evalErr)
		return dec.Allowed
	}
	require.True(t, check(auth.RoleAdmin), "admin allowed manage")
	require.False(t, check(auth.RoleWriter), "writer denied manage")
	require.False(t, check(auth.RoleReader), "reader denied manage")
}
```

> This test depends on `buildActionEntities` accepting `"user.manage"` — i.e. `manage` being a known verb. It fails until Step 3.

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run TestEvaluate_ManageVerbAdminOnly -v`

Expected: FAIL — `NewCedarEngine` errors (`action "user.manage" has unknown verb "manage"`) because `manage` isn't in `knownVerbs` yet.

- [ ] **Step 3: Add `manage` to `knownVerbs`**

In `internal/auth/engine.go`, change:

```go
var knownVerbs = map[string]bool{"read": true, "write": true, "delete": true}
```

to:

```go
var knownVerbs = map[string]bool{"read": true, "write": true, "delete": true, "manage": true}
```

- [ ] **Step 4: Add the manage policy to the embedded base policies**

Append to `internal/auth/policies/base.cedar`:

```text
// Identity-management operations (IdentityService): admin-only. The "manage"
// verb is reserved for sensitive operations that must NOT inherit the
// read/write/delete role gates (e.g. ListUsers is a read but must be admin).
permit (
	principal,
	action in SpecGraph::Action::"manage",
	resource
) when {
	principal has role && principal.role == "admin"
};
```

- [ ] **Step 5: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run 'TestEvaluate|TestBuildActionEntities' -v`

Expected: PASS — `manage` is a known verb; the admin policy gates it; the Cedar plan's existing `TestBuildActionEntities_RejectsUnknownVerb` (frobnicate) still passes.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/engine.go internal/auth/policies/base.cedar internal/auth/engine_evaluate_test.go
git commit -s -m "feat(auth): add Cedar 'manage' verb + admin-only base policy for identity ops"
```

---

## Task 12: Map identity procedures to Cedar actions

**Files:**

- Modify: `internal/auth/actions.go` (add identity entries to `procedureActions`)
- Modify: `internal/auth/actions_test.go`

**Covers:** Happy (each identity procedure maps) + Invariant (whoami→identity.read; all mgmt→a `.manage` action; no method-name leakage).

- [ ] **Step 1: Write the failing test**

Append to `internal/auth/actions_test.go`:

```go
func TestActionForProcedure_Identity(t *testing.T) {
	cases := map[string]string{
		specgraphv1connect.IdentityServiceWhoamiProcedure:               "identity.read",
		specgraphv1connect.IdentityServiceListUsersProcedure:            "user.manage",
		specgraphv1connect.IdentityServiceGetUserProcedure:              "user.manage",
		specgraphv1connect.IdentityServiceUpdateUserRoleProcedure:       "user.manage",
		specgraphv1connect.IdentityServiceSoftDeleteUserProcedure:       "user.manage",
		specgraphv1connect.IdentityServicePurgeUserProcedure:            "user.manage",
		specgraphv1connect.IdentityServiceCreateServiceAccountProcedure: "serviceaccount.manage",
		specgraphv1connect.IdentityServiceCreateAPIKeyProcedure:         "apikey.manage",
		specgraphv1connect.IdentityServiceRevokeAPIKeyProcedure:         "apikey.manage",
		specgraphv1connect.IdentityServiceRotateAPIKeyProcedure:         "apikey.manage",
		specgraphv1connect.IdentityServiceListAPIKeysProcedure:          "apikey.manage",
		specgraphv1connect.IdentityServiceListOIDCBindingsProcedure:     "oidc.manage",
		specgraphv1connect.IdentityServiceUnbindOIDCProcedure:           "oidc.manage",
	}
	for proc, want := range cases {
		got, ok := auth.ActionForProcedure(proc)
		require.Truef(t, ok, "procedure %s must map", proc)
		require.Equalf(t, want, got, "procedure %s", proc)
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run TestActionForProcedure_Identity -v`

Expected: FAIL — the identity procedures aren't in `procedureActions` yet.

- [ ] **Step 3: Add the entries**

In `internal/auth/actions.go`, add to the `procedureActions` map literal (before the closing brace):

```go
	// IdentityService — whoami is self-scoped (read); all management ops are
	// admin-gated via the "manage" verb.
	specgraphv1connect.IdentityServiceWhoamiProcedure:               "identity.read",
	specgraphv1connect.IdentityServiceListUsersProcedure:            "user.manage",
	specgraphv1connect.IdentityServiceGetUserProcedure:              "user.manage",
	specgraphv1connect.IdentityServiceUpdateUserRoleProcedure:       "user.manage",
	specgraphv1connect.IdentityServiceSoftDeleteUserProcedure:       "user.manage",
	specgraphv1connect.IdentityServicePurgeUserProcedure:            "user.manage",
	specgraphv1connect.IdentityServiceCreateServiceAccountProcedure: "serviceaccount.manage",
	specgraphv1connect.IdentityServiceCreateAPIKeyProcedure:         "apikey.manage",
	specgraphv1connect.IdentityServiceRevokeAPIKeyProcedure:         "apikey.manage",
	specgraphv1connect.IdentityServiceRotateAPIKeyProcedure:         "apikey.manage",
	specgraphv1connect.IdentityServiceListAPIKeysProcedure:          "apikey.manage",
	specgraphv1connect.IdentityServiceListOIDCBindingsProcedure:     "oidc.manage",
	specgraphv1connect.IdentityServiceUnbindOIDCProcedure:           "oidc.manage",
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run 'TestActionForProcedure|TestActionNames' -v`

Expected: PASS. `ActionNames()` now includes `identity.read`, `user.manage`, `serviceaccount.manage`, `apikey.manage`, `oidc.manage` (the `.manage` ones group under the new `manage` verb; `identity.read` under `read`). The Cedar plan's `TestActionNames_AllParseToKnownVerb` still passes (all suffixes are known, including `manage`).

- [ ] **Step 5: Commit**

```bash
git add internal/auth/actions.go internal/auth/actions_test.go
git commit -s -m "feat(auth): map IdentityService procedures to Cedar actions"
```

---

## Task 13: Register `IdentityService` in `serve.go`

**Files:**

- Modify: `cmd/specgraph/serve.go`

**Covers:** E2E (the service is wired with the auth interceptor and the real `AuthStore` backend).

- [ ] **Step 1: Add the registration**

In `cmd/specgraph/serve.go`, alongside the other `server.Register*Service(...)` calls, add the identity registration. The siblings pass two trailing `connect.HandlerOption` values — `opts` (`connect.WithInterceptors(interceptor)`) and `maxBytes` (`connect.WithReadMaxBytes(4<<20)`) — into the variadic; mirror that exactly so identity RPCs get both the auth interceptor AND the body-size limit:

```go
	server.RegisterIdentityService(mux, authStore, opts, maxBytes)
```

> `RegisterIdentityService(mux, users storage.UsersBackend, opts ...connect.HandlerOption)` (Task 3) takes the variadic, matching the sibling `Register*Service` signatures (e.g. `RegisterGraphService(mux, scoper, opts ...connect.HandlerOption)`, called as `RegisterGraphService(mux, store, opts, maxBytes)`). Match the merged serve.go's variable names: `authStore` (the `*postgres.AuthStore`, which implements `storage.UsersBackend`), `opts`, `maxBytes` (all present in the post-Authn/Cedar serve.go). The IdentityService MUST receive `opts` — otherwise its RPCs would be unauthenticated.

- [ ] **Step 2: Verify whole-project build + test**

Run: `go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/specgraph/serve.go
git commit -s -m "feat(serve): register IdentityService with the auth interceptor"
```

---

## Task 14: Render helpers for identity entities

**Files:**

- Create: `internal/render/identity.go`
- Create: `internal/render/identity_test.go`

**Covers:** Happy (renders a user table, key table, binding table) + Boundary (empty slice → "no X" message).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestUserList_RendersRows(t *testing.T) {
	out := render.UserList([]*specv1.User{
		{Id: "u1", Kind: specv1.UserKind_USER_KIND_HUMAN, DisplayName: "Alice", Role: "admin", CreatedAt: timestamppb.Now()},
	})
	require.Contains(t, out, "u1")
	require.Contains(t, out, "Alice")
	require.Contains(t, out, "admin")
}

func TestUserList_Empty(t *testing.T) {
	require.Contains(t, strings.ToLower(render.UserList(nil)), "no users")
}

func TestAPIKeyList_RedactsSecret(t *testing.T) {
	out := render.APIKeyList([]*specv1.APIKey{{Id: "k1", Prefix: "abc12345", Label: "ci"}})
	require.Contains(t, out, "abc12345")
	require.Contains(t, out, "ci")
}

func TestOIDCBindingList_RendersRows(t *testing.T) {
	out := render.OIDCBindingList([]*specv1.OIDCBinding{{Id: "b1", Issuer: "https://idp", Subject: "sub"}})
	require.Contains(t, out, "https://idp")
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/render && go test -run 'TestUserList|TestAPIKeyList|TestOIDCBindingList' -v`

Expected: FAIL ("undefined: render.UserList").

- [ ] **Step 3: Write the renderers**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// UserList renders a table of users.
func UserList(users []*specv1.User) string {
	if len(users) == 0 {
		return "No users found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-38s  %-16s  %-20s  %-8s  %s\n", "ID", "KIND", "DISPLAY NAME", "ROLE", "STATUS")
	for _, u := range users {
		status := "active"
		if u.GetDeletedAt() != nil {
			status = "deleted"
		}
		fmt.Fprintf(&b, "%-38s  %-16s  %-20s  %-8s  %s\n",
			u.GetId(), userKindLabel(u.GetKind()), truncate(u.GetDisplayName(), 20), u.GetRole(), status)
	}
	return b.String()
}

func userKindLabel(k specv1.UserKind) string {
	switch k {
	case specv1.UserKind_USER_KIND_HUMAN:
		return "human"
	case specv1.UserKind_USER_KIND_SERVICE_ACCOUNT:
		return "service_account"
	default:
		return "unspecified"
	}
}

// APIKeyList renders a table of API keys. Only the prefix is shown; the
// secret is never available here.
func APIKeyList(keys []*specv1.APIKey) string {
	if len(keys) == 0 {
		return "No API keys found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-38s  %-10s  %-20s  %s\n", "ID", "PREFIX", "LABEL", "STATUS")
	for _, k := range keys {
		status := "active"
		if k.GetRevokedAt() != nil {
			status = "revoked"
		}
		fmt.Fprintf(&b, "%-38s  %-10s  %-20s  %s\n",
			k.GetId(), k.GetPrefix(), truncate(k.GetLabel(), 20), status)
	}
	return b.String()
}

// OIDCBindingList renders a table of OIDC bindings.
func OIDCBindingList(bindings []*specv1.OIDCBinding) string {
	if len(bindings) == 0 {
		return "No OIDC bindings found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-38s  %-30s  %s\n", "ID", "ISSUER", "SUBJECT")
	for _, bd := range bindings {
		fmt.Fprintf(&b, "%-38s  %-30s  %s\n", bd.GetId(), truncate(bd.GetIssuer(), 30), bd.GetSubject())
	}
	return b.String()
}
```

> **Reuse the existing `truncate`.** `internal/render/slice.go:92` already defines `func truncate(s string, maxLen int) string` in package `render` — do NOT redeclare it (same-package redeclaration is a build error). The calls above resolve to that existing helper (signature `(s, maxLen)` is call-compatible). These renderers follow the existing `render.*` "return a string" convention used by other commands.

- [ ] **Step 4: Run the tests**

Run: `cd internal/render && go build ./... && go test -run 'TestUserList|TestAPIKeyList|TestOIDCBindingList' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/identity.go internal/render/identity_test.go
git commit -s -m "feat(render): identity table renderers (user, api-key, oidc binding)"
```

---

## Task 15: CLI — `identityClient` + `specgraph auth` root + `whoami`

**Files:**

- Create: `cmd/specgraph/identity_client.go`
- Create: `cmd/specgraph/auth.go`

**Covers:** Happy (whoami prints identity) — exercised manually; the command wiring is verified by `go build` + a `--help` smoke.

- [ ] **Step 1: Write the client helper**

`identity_client.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// identityClient builds an authenticated IdentityService client using the
// shared credential/base-URL resolution (newClient).
func identityClient() (specgraphv1connect.IdentityServiceClient, error) {
	return newClient(specgraphv1connect.NewIdentityServiceClient)
}
```

> `newClient[C any](ctor)` is the existing generic client builder in `cmd/specgraph/client.go` (it resolves base URL + project + API key and wraps the transport). Confirm its signature; the spec/decision commands use the same pattern.

- [ ] **Step 2: Write the auth root + whoami**

`auth.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage identity: users, service accounts, API keys, and OIDC bindings",
}

// authJSON backs the --json flag across the auth subtree. There is no global
// --json flag in this CLI (each command declares its own — see spec.go's
// per-command showJSON/listJSON pattern). Cobra binds a distinct --json flag
// on each leaf command to this shared var; only one command runs per
// invocation, so sharing is safe and keeps the registrations terse.
var authJSON bool

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the identity of the current credential",
	RunE:  runAuthWhoami,
}

func runAuthWhoami(cmd *cobra.Command, _ []string) error {
	client, err := identityClient()
	if err != nil {
		return err
	}
	resp, err := client.Whoami(cmd.Context(), connect.NewRequest(&specv1.WhoamiRequest{}))
	if err != nil {
		return fmt.Errorf("whoami: %w", err)
	}
	if authJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Subject:        %s\n", resp.Msg.GetSubject())
	fmt.Fprintf(cmd.OutOrStdout(), "User ID:        %s\n", resp.Msg.GetUserId())
	fmt.Fprintf(cmd.OutOrStdout(), "Display name:   %s\n", resp.Msg.GetDisplayName())
	fmt.Fprintf(cmd.OutOrStdout(), "Role:           %s\n", resp.Msg.GetRole())
	fmt.Fprintf(cmd.OutOrStdout(), "Effective role: %s\n", resp.Msg.GetEffectiveRole())
	fmt.Fprintf(cmd.OutOrStdout(), "Source:         %s\n", resp.Msg.GetSource())
	return nil
}

func init() {
	authWhoamiCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authCmd.AddCommand(authWhoamiCmd)
	rootCmd.AddCommand(authCmd)
}
```

> `printJSON(w io.Writer, msg proto.Message)` is the existing helper (`cmd/specgraph/output.go`). There is NO global `--json` var — each command registers its own (`spec.go` uses `showJSON`/`listJSON` bound via `cmd.Flags().BoolVar(&x, "json", false, ...)`). Tasks 15–18 share the package-level `authJSON` var declared here and register a `--json` flag on every JSON-emitting leaf command.

- [ ] **Step 3: Verify build + help smoke**

Run: `go build ./... && go run ./cmd/specgraph auth --help`

Expected: build PASS; `--help` lists `whoami`.

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/identity_client.go cmd/specgraph/auth.go
git commit -s -m "feat(cli): add 'specgraph auth' root + whoami"
```

---

## Task 16: CLI — `specgraph auth user`

**Files:**

- Create: `cmd/specgraph/auth_user.go`

**Covers:** Command wiring (build + help smoke). Behavior is covered by the handler tests (Tasks 4–6) and the integration test (Task 19).

- [ ] **Step 1: Write the user subcommands**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

var (
	userListKind        string
	userListRole        string
	userListIncludeDel  bool
)

var authUserCmd = &cobra.Command{Use: "user", Short: "Manage users"}

var authUserListCmd = &cobra.Command{
	Use:   "list",
	Short: "List users",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		req := &specv1.ListUsersRequest{
			Role:           userListRole,
			IncludeDeleted: userListIncludeDel,
		}
		switch userListKind {
		case "human":
			req.Kind = specv1.UserKind_USER_KIND_HUMAN
		case "service_account":
			req.Kind = specv1.UserKind_USER_KIND_SERVICE_ACCOUNT
		case "":
			// all kinds
		default:
			return fmt.Errorf("invalid --kind %q (want human|service_account)", userListKind)
		}
		resp, err := client.ListUsers(cmd.Context(), connect.NewRequest(req))
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		fmt.Fprint(cmd.OutOrStdout(), render.UserList(resp.Msg.GetUsers()))
		return nil
	},
}

var authUserShowCmd = &cobra.Command{
	Use:   "show <user-id>",
	Short: "Show a single user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.GetUser(cmd.Context(), connect.NewRequest(&specv1.GetUserRequest{Id: args[0]}))
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		fmt.Fprint(cmd.OutOrStdout(), render.UserList([]*specv1.User{resp.Msg.GetUser()}))
		return nil
	},
}

var authUserSetRoleCmd = &cobra.Command{
	Use:   "set-role <user-id> <role>",
	Short: "Change a user's role",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.UpdateUserRole(cmd.Context(), connect.NewRequest(&specv1.UpdateUserRoleRequest{Id: args[0], Role: args[1]}))
		if err != nil {
			return fmt.Errorf("set role: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated %s → role %s\n", args[0], resp.Msg.GetUser().GetRole())
		return nil
	},
}

var authUserDeleteCmd = &cobra.Command{
	Use:   "delete <user-id>",
	Short: "Soft-delete a user (revokes their keys)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		if _, err := client.SoftDeleteUser(cmd.Context(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: args[0]})); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Soft-deleted %s\n", args[0])
		return nil
	},
}

var authUserPurgeCmd = &cobra.Command{
	Use:   "purge <user-id>",
	Short: "Permanently delete a user (irreversible)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		if _, err := client.PurgeUser(cmd.Context(), connect.NewRequest(&specv1.PurgeUserRequest{Id: args[0]})); err != nil {
			return fmt.Errorf("purge user: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Purged %s\n", args[0])
		return nil
	},
}

func init() {
	authUserListCmd.Flags().StringVar(&userListKind, "kind", "", "filter by kind: human|service_account")
	authUserListCmd.Flags().StringVar(&userListRole, "role", "", "filter by role")
	authUserListCmd.Flags().BoolVar(&userListIncludeDel, "include-deleted", false, "include soft-deleted users")
	// --json on the JSON-emitting commands (shared authJSON var from auth.go).
	authUserListCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authUserShowCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authUserSetRoleCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authUserCmd.AddCommand(authUserListCmd, authUserShowCmd, authUserSetRoleCmd, authUserDeleteCmd, authUserPurgeCmd)
	authCmd.AddCommand(authUserCmd)
}
```

> Plan 4b adds `--force` to `delete` and `purge` (wired to the proto `force` field it introduces).

- [ ] **Step 2: Verify build + help**

Run: `go build ./... && go run ./cmd/specgraph auth user --help`

Expected: build PASS; lists `list`, `show`, `set-role`, `delete`, `purge`.

- [ ] **Step 3: Commit**

```bash
git add cmd/specgraph/auth_user.go
git commit -s -m "feat(cli): add 'specgraph auth user' subcommands"
```

---

## Task 17: CLI — `specgraph auth api-key`

**Files:**

- Create: `cmd/specgraph/auth_apikey.go`

**Covers:** Command wiring (build + help smoke); the create/rotate plaintext-once behavior is asserted by the handler tests (Task 8).

- [ ] **Step 1: Write the api-key subcommands**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

var (
	apiKeyListUser    string
	apiKeyListRevoked bool
	apiKeyCreateUser  string
	apiKeyCreateLabel string
	apiKeyCreateDown  string
	apiKeyRotateUser  string
	apiKeyRotateLabel string
	apiKeyRotateDown  string
)

var authAPIKeyCmd = &cobra.Command{Use: "api-key", Short: "Manage API keys"}

var authAPIKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.ListAPIKeys(cmd.Context(), connect.NewRequest(&specv1.ListAPIKeysRequest{
			UserId: apiKeyListUser, IncludeRevoked: apiKeyListRevoked,
		}))
		if err != nil {
			return fmt.Errorf("list api keys: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		fmt.Fprint(cmd.OutOrStdout(), render.APIKeyList(resp.Msg.GetKeys()))
		return nil
	},
}

var authAPIKeyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API key (prints the secret once)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if apiKeyCreateUser == "" {
			return fmt.Errorf("--user is required")
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateAPIKey(cmd.Context(), connect.NewRequest(&specv1.CreateAPIKeyRequest{
			UserId: apiKeyCreateUser, Label: apiKeyCreateLabel, RoleDowngrade: apiKeyCreateDown,
		}))
		if err != nil {
			return fmt.Errorf("create api key: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created API key %s (prefix %s).\n", resp.Msg.GetKey().GetId(), resp.Msg.GetKey().GetPrefix())
		fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n\n", resp.Msg.GetPlaintext())
		fmt.Fprintln(cmd.OutOrStdout(), "Store this token now — it will not be shown again.")
		return nil
	},
}

var authAPIKeyRevokeCmd = &cobra.Command{
	Use:   "revoke <key-id>",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		if _, err := client.RevokeAPIKey(cmd.Context(), connect.NewRequest(&specv1.RevokeAPIKeyRequest{KeyId: args[0]})); err != nil {
			return fmt.Errorf("revoke api key: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Revoked %s\n", args[0])
		return nil
	},
}

var authAPIKeyRotateCmd = &cobra.Command{
	Use:   "rotate <key-id>",
	Short: "Rotate an API key (revokes the old, prints the new secret once)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// storage's RotateAPIKey requires the new key's owner + metadata (it
		// does not copy from the old key; there is no get-key-by-id method).
		if apiKeyRotateUser == "" {
			return fmt.Errorf("--user is required (owner of the key being rotated)")
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.RotateAPIKey(cmd.Context(), connect.NewRequest(&specv1.RotateAPIKeyRequest{
			KeyId: args[0], UserId: apiKeyRotateUser, Label: apiKeyRotateLabel, RoleDowngrade: apiKeyRotateDown,
		}))
		if err != nil {
			return fmt.Errorf("rotate api key: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Rotated → new key %s (prefix %s).\n", resp.Msg.GetKey().GetId(), resp.Msg.GetKey().GetPrefix())
		fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n\n", resp.Msg.GetPlaintext())
		fmt.Fprintln(cmd.OutOrStdout(), "Store this token now — it will not be shown again.")
		return nil
	},
}

func init() {
	authAPIKeyListCmd.Flags().StringVar(&apiKeyListUser, "user", "", "filter by user ID")
	authAPIKeyListCmd.Flags().BoolVar(&apiKeyListRevoked, "include-revoked", false, "include revoked keys")
	authAPIKeyListCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateUser, "user", "", "user ID to own the key (required)")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateLabel, "label", "", "human-friendly label")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateDown, "role-downgrade", "", "cap the key's effective role")
	authAPIKeyCreateCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyRotateCmd.Flags().StringVar(&apiKeyRotateUser, "user", "", "owner of the key being rotated (required)")
	authAPIKeyRotateCmd.Flags().StringVar(&apiKeyRotateLabel, "label", "", "label for the new key")
	authAPIKeyRotateCmd.Flags().StringVar(&apiKeyRotateDown, "role-downgrade", "", "role downgrade for the new key")
	authAPIKeyRotateCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyCmd.AddCommand(authAPIKeyListCmd, authAPIKeyCreateCmd, authAPIKeyRevokeCmd, authAPIKeyRotateCmd)
	authCmd.AddCommand(authAPIKeyCmd)
}
```

- [ ] **Step 2: Verify build + help**

Run: `go build ./... && go run ./cmd/specgraph auth api-key --help`

Expected: build PASS; lists `list`, `create`, `revoke`, `rotate`.

- [ ] **Step 3: Commit**

```bash
git add cmd/specgraph/auth_apikey.go
git commit -s -m "feat(cli): add 'specgraph auth api-key' subcommands"
```

---

## Task 18: CLI — `specgraph auth oidc`

**Files:**

- Create: `cmd/specgraph/auth_oidc.go`

**Covers:** Command wiring (build + help smoke).

- [ ] **Step 1: Write the oidc subcommands**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

var oidcListUser string

var authOIDCCmd = &cobra.Command{Use: "oidc", Short: "Manage OIDC bindings"}

var authOIDCListCmd = &cobra.Command{
	Use:   "list",
	Short: "List a user's OIDC bindings",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if oidcListUser == "" {
			return fmt.Errorf("--user is required")
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.ListOIDCBindings(cmd.Context(), connect.NewRequest(&specv1.ListOIDCBindingsRequest{UserId: oidcListUser}))
		if err != nil {
			return fmt.Errorf("list oidc bindings: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		fmt.Fprint(cmd.OutOrStdout(), render.OIDCBindingList(resp.Msg.GetBindings()))
		return nil
	},
}

var authOIDCUnbindCmd = &cobra.Command{
	Use:   "unbind <binding-id>",
	Short: "Remove an OIDC binding",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		if _, err := client.UnbindOIDC(cmd.Context(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: args[0]})); err != nil {
			return fmt.Errorf("unbind oidc: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Unbound %s\n", args[0])
		return nil
	},
}

func init() {
	authOIDCListCmd.Flags().StringVar(&oidcListUser, "user", "", "user ID (required)")
	authOIDCListCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authOIDCCmd.AddCommand(authOIDCListCmd, authOIDCUnbindCmd)
	authCmd.AddCommand(authOIDCCmd)
}
```

> Plan 4b adds `--force` to `unbind` (last-credential protection).

- [ ] **Step 2: Verify build + help + whole-project test**

```bash
go build ./... && go run ./cmd/specgraph auth oidc --help
go test ./...
```

Expected: build PASS; lists `list`, `unbind`; all tests pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/specgraph/auth_oidc.go
git commit -s -m "feat(cli): add 'specgraph auth oidc' subcommands"
```

---

## Task 19: Integration test — interceptor → Cedar → IdentityHandler

End-to-end through the real auth interceptor and Cedar engine: admin can manage, reader is denied management but can whoami.

**Files:**

- Create: `internal/server/identity_integration_test.go`

**Covers:** E2E (auth + authz + handler) + Invariant (manage gated to admin; whoami open to reader).

- [ ] **Step 1: Write the test**

```go
//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

// buildIdentityTestServer wires the real interceptor (resolver + Cedar
// authorizer) and the IdentityService over an httptest server. Returns the
// base URL and a helper to mint a token for a user of a given role.
func buildIdentityTestServer(t *testing.T, ctx context.Context) (string, *postgres.AuthStore) {
	t.Helper()
	pool := postgrestest.SharedPool(t, ctx) // postgres harness
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })
	_, err = pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)

	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{Users: authStore, Tracker: tracker})
	require.NoError(t, err)

	engine, err := auth.NewCedarEngine(ctx, []auth.PolicySource{auth.NewEmbeddedPolicySource()}, auth.ActionNames())
	require.NoError(t, err)
	authorizer := auth.NewCedarAuthorizer(engine)
	interceptor := auth.NewAuthInterceptor(resolver, authorizer)

	mux := http.NewServeMux()
	server.RegisterIdentityService(mux, authStore, connect.WithInterceptors(interceptor))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL, authStore
}

// mintFor creates a user of the given role + an API key, returning the token.
func mintFor(t *testing.T, ctx context.Context, store *postgres.AuthStore, role string, bootstrap bool) string {
	t.Helper()
	u, err := store.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: role + "-user", Role: role, Bootstrap: bootstrap}, nil)
	require.NoError(t, err)
	secret, phc, err := auth.GenerateAPIKeySecret()
	require.NoError(t, err)
	created, err := store.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: phc}) // storage assigns the prefix
	require.NoError(t, err)
	return auth.FormatAPIKeyToken(created.Prefix, secret)
}

func tokenClient(baseURL, token string) specgraphv1connect.IdentityServiceClient {
	httpc := &http.Client{Transport: &bearerTransport{token: token, base: http.DefaultTransport}}
	return specgraphv1connect.NewIdentityServiceClient(httpc, baseURL)
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b *bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r2)
}

func TestIntegration_IdentityAdminCanManage(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	adminToken := mintFor(t, ctx, store, "admin", true)

	client := tokenClient(baseURL, adminToken)
	resp, err := client.ListUsers(ctx, connect.NewRequest(&specv1.ListUsersRequest{}))
	require.NoError(t, err, "admin may ListUsers (user.manage)")
	require.GreaterOrEqual(t, len(resp.Msg.GetUsers()), 1)
}

func TestIntegration_IdentityReaderDeniedManage(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	_ = mintFor(t, ctx, store, "admin", true) // ensure a bootstrap exists
	readerToken := mintFor(t, ctx, store, "reader", false)

	client := tokenClient(baseURL, readerToken)
	_, err := client.ListUsers(ctx, connect.NewRequest(&specv1.ListUsersRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "reader denied user.manage")
}

func TestIntegration_IdentityWhoamiOpenToReader(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	_ = mintFor(t, ctx, store, "admin", true)
	readerToken := mintFor(t, ctx, store, "reader", false)

	client := tokenClient(baseURL, readerToken)
	resp, err := client.Whoami(ctx, connect.NewRequest(&specv1.WhoamiRequest{}))
	require.NoError(t, err, "any authenticated principal may whoami (identity.read)")
	require.Equal(t, "reader", resp.Msg.GetEffectiveRole())
}
```

> Match the merged harness: `postgrestest.SharedPool` (exported; Storage plan Task 32), `postgres.NewAuth`, `auth.NewAuthInterceptor` (post-Authn-cleanup name), `auth.NewIdentityStore`, `usagetracker`. If `CreateHuman` rejects a second `Bootstrap:true`, the reader is created with `bootstrap=false` (as written). The bootstrap admin is created first so `mintFor(...,"admin",true)` is the sole bootstrap.

- [ ] **Step 2: Run**

Run: `cd internal/server && go test -tags integration -run 'TestIntegration_Identity' -v`

Expected: PASS — admin manages, reader denied (PermissionDenied), reader whoami works.

- [ ] **Step 3: Full suites**

```bash
# from the repository root:
go build ./... && go test ./...
go test -tags integration ./internal/server/... ./internal/auth/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/server/identity_integration_test.go
git commit -s -m "test(server): integration test for identity authz (admin/reader/whoami)"
```

---

## Self-Review

**1. Spec coverage** (against the design's "CLI shape" + the RPC surface 4a owns):

- [x] User management (list, show, change role, delete, purge): Tasks 4, 5, 6 (RPCs) + 16 (CLI).
- [x] ServiceAccount management: Task 5 (RPC) + 16 (`user list --kind service_account`; create via api-key/user flows — SA creation RPC in Task 5, surfaced as a follow-up CLI command if desired; the design says SA creation is "a normal admin RPC", delivered).
- [x] API-key management (list, create, revoke, rotate): Tasks 8, 9 (RPCs) + 17 (CLI), with mint helper Task 7.
- [x] OIDC binding management (list, unbind): Task 10 (RPC) + 18 (CLI).
- [x] Identity introspection (`whoami`): Task 3 (RPC) + 15 (CLI).
- [x] All identity commands under one root, discoverable via `specgraph auth --help`: Tasks 15–18.
- [x] Admin-role gating expressed as Cedar policies (not handler flags): Tasks 11–12.
- [x] Plaintext key shown once, never persisted: Tasks 7, 8 (assertions) + 17 (UX).
- [x] (Deferred to 4b, explicitly): force-flag protections + `force` proto fields + credential-file/bootstrap. Tasks 6, 10 note the boundary.

> ServiceAccount *creation* has an RPC (Task 5) but Task 16's CLI doesn't add a dedicated `service-account create` command. If the design wants that surfaced, add an `auth service-account create` command mirroring Task 16's pattern — flagged here as the one CLI affordance not yet wired (RPC exists; CLI command is a trivial follow-up). Listing SAs is covered by `auth user list --kind service_account`.

**2. Placeholder scan:** No TODO/TBD/"similar to above". The Unimplemented stubs in Task 3 are explicitly temporary and each is replaced in Tasks 4–10 with a "delete the stub" instruction. Every handler method shows complete code; every test shows complete code.

**3. Type consistency:** `IdentityHandler`, `userToProto`/`apiKeyToProto`/`oidcBindingToProto`, `usersBackendStub`, `GenerateAPIKeySecret`/`FormatAPIKeyToken`/`APIKeyTokenPrefix`, `RegisterIdentityService`, `identityClient`, the shared `authJSON` flag var, the `auth*Cmd` cobra vars, and the `render.UserList`/`APIKeyList`/`OIDCBindingList` renderers are referenced identically across tasks. The `var _ specgraphv1connect.IdentityServiceHandler = (*IdentityHandler)(nil)` assertion (Task 3) guarantees the method set matches the generated interface. The API-key minting split (`GenerateAPIKeySecret` returns secret+hash; storage assigns the prefix; `FormatAPIKeyToken` assembles the token from the storage-returned prefix) is applied consistently in Tasks 7, 8, and the Task 19 `mintFor` helper.

**4. Build discipline:** Every task ends with `go build ./... && go test ./...` green at package level; Tasks 3, 6, 10, 13, 18, 19 add whole-project build+test. No task leaves the project non-building: Task 3's `var _` assertion is satisfied by Unimplemented stubs from the start, so the handler compiles before any RPC is real. The Cedar changes (Tasks 11–12) are additive (`manage` verb, new policy, new map entries) and don't disturb the migration matrix. serve.go registration (Task 13) only runs after the handler (3–10) and Cedar wiring (11–12) exist, so the interceptor never hits an unconfigured identity procedure.

**5. Symbol-lifetime sweep:** Completed in the dedicated section. No collisions; the only cross-plan touch is additive (`manage` ∈ `knownVerbs`; identity entries in `procedureActions`; one base.cedar policy). No deletions. The Cedar plan's `TestBuildActionEntities_RejectsUnknownVerb` and `TestActionNames_AllParseToKnownVerb` both still pass (manage is now a known verb; frobnicate still isn't).

**6. Dependency-name risk:** Every consumed prior-plan symbol is listed in the "Dependency contract" with a note to adapt if the merged name differs. The two highest-risk assumptions — the API-key format constants (`apiKeyPrefixLen`/`apiKeySecretLen`/argon2 params) and `auth.NewAuthInterceptor`'s post-cleanup name — are pinned and exercised by the Task 7 round-trip integration test and the Task 19 interceptor test respectively.

---

## Execution

1. **Subagent-Driven (recommended)** — Tasks 1–10 are short TDD cycles (proto, converters, one-or-two RPCs each). Tasks 11–12 (Cedar) and 13 (serve) are a small wiring batch. Tasks 14–18 (render + CLI) are mechanical. Task 19 (integration) is the capstone. Fresh subagent per task; the "delete the Unimplemented stub" steps in 4–10 are explicit to avoid redeclaration.
2. **Inline Execution** — practical for 11–13 (Cedar + serve wiring) and 14–18 (CLI) as batches.

**Plan 4b follows immediately** and depends on this plan: it adds the bootstrap DB helper (reusing `GenerateAPIKeySecret` + `FormatAPIKeyToken`), the `specgraph init` local path and `serve.go` hosted path, the credentials-file multi-server schema, and the force-flag / last-credential protections (adding `force` to `SoftDeleteUserRequest`/`PurgeUserRequest`/`UnbindOIDCRequest` + the guard logic + `--force` CLI flags). 4a and 4b should land together; do not deploy 4a's unguarded delete/purge/unbind without 4b's protections.
