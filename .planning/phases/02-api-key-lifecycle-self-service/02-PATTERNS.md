# Phase 2: API Key Lifecycle & Self-Service - Pattern Map

**Mapped:** 2026-07-09
**Files analyzed:** 24 (create + modify)
**Analogs found:** 22 / 24 (2 partial — CSRF middleware, one-time reveal modal)

> This is a **brownfield hardening phase**. Almost every new symbol has a sibling already in-tree; the
> failure mode is *re-inventing* a reviewed helper, not a missing pattern. RESEARCH.md's
> §"Verified Touch-Surface" is the authoritative file list — this map grounds each entry against the
> actual current source with concrete copy-from excerpts. All line numbers below were read this session.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `proto/specgraph/v1/identity.proto` (mod) | proto/schema | request-response | Existing `CreateAPIKey`/`RotateAPIKey`/`RevokeAPIKey`/`ListAPIKeys` + `UpdateUserRole` RPCs (`identity.proto:26-32`) | exact |
| `internal/auth/engine.go` (mod) | config/registry | transform | `knownVerbs` map (`engine.go:155`) | exact (1-line add) |
| `internal/auth/actions.go` (mod) | config/registry | transform | `procedureActions` Identity block (`actions.go:92-106`) | exact |
| `internal/auth/actions_test.go` (mod) | test | — | `TestActionNames_AllParseToKnownVerb` (`:33`) + `TestActionForProcedure_Identity` (`:50`) | exact |
| `internal/auth/policies/base.cedar` (mod) | policy | — | `"manage"` verb permit (`base.cedar:42-48`) | exact |
| `internal/auth/rolemin.go` (new) | utility | transform | `clampedRole`/`roleLessThan`/`roleRank` (`identitystore.go:256-302`) | role-match |
| `internal/auth/rolemin_test.go` (new) | test | — | (table test; no direct analog) — model on `clampedRole` test conventions | partial |
| `internal/server/identity_handler.go` (mod) | controller/handler | request-response | `CreateAPIKey` (`:223`), `RotateAPIKey` (`:280`), `ListAPIKeys` (`:313`), `RevokeAPIKey` (`:266`), `UpdateUserRole` (`:155`) | exact |
| `internal/server/csrf.go` (new) | middleware | request-response | `cookieToAuthHeader` middleware (`auth_handler.go:136`) | role-match |
| `internal/server/identity_selfkeys_test.go` (new) | test | — | (server handler tests) | role-match |
| `internal/server/identity_resync_test.go` (new) | test | — | (server handler tests) | role-match |
| `internal/storage/users.go` (mod) | interface/model | CRUD | `UsersBackend` API-key CRUD block (`users.go:77-102`) | exact |
| `internal/storage/postgres/users.go` (mod) | storage/repository | CRUD | `RevokeAPIKey` (`:408`), `RotateAPIKey` (`:425`), `CreateAPIKey` (`:361`), `UpdateUserRole` `RowsAffected` guard (`:233`) | exact |
| `internal/storage/postgres/users_selfkeys_test.go` (new) | test (integration) | CRUD | (postgres testcontainer tests) | role-match |
| `internal/auth/usersbackend_stub_test.go` (mod) | test (compile-gate) | — | existing `var _ storage.UsersBackend` stub | exact |
| `internal/server/usersbackend_stub_test.go` (mod) | test (compile-gate) | — | existing `var _ storage.UsersBackend` stub | exact |
| `cmd/specgraph/auth_apikey.go` (mod) | CLI/command | request-response | `authAPIKeyCreateCmd`/`Rotate`/`Revoke`/`List` (`auth_apikey.go:44-171`) | exact |
| `cmd/specgraph/auth_user.go` (mod) | CLI/command | request-response | `authUserSetRoleCmd` (`auth_user.go:80`) | exact |
| `cmd/specgraph/client.go` (mod) | CLI/client wiring | request-response | `resolveAPIKey` precedence (`client.go:102`) | exact |
| `internal/config/global.go` (mod) | config | — | `JITCreateConfig` struct + `globalDefaults()` (`global.go:226`, `:433`) | exact (role-match; NOT `APIKeyConfig`) |
| `web/src/routes/keys/+page.svelte` (new) | component/route | request-response | read-only routes `graph/+page.svelte`, `constitution/+page.svelte` | role-match |
| `web/src/lib/components/RevealKeyModal.svelte` (new) | component | — | `LoginModal.svelte` (modal overlay + form) | partial |
| `web/src/lib/keys.svelte.ts` (new) | store/state | request-response | `auth.svelte.ts` ($state + fetch + POST) | role-match |
| `web/src/lib/api/client.ts` (mod) | client wiring | request-response | `createClient(...)` block (`client.ts:35-41`) | exact |

---

## Shared Patterns

### Cedar verb registration (server-boot invariant)
**Sources:** `internal/auth/engine.go:155`, `internal/auth/policies/base.cedar:42`, `internal/auth/actions.go:92`
**Apply to:** the new `apikey.self` verb — ALL THREE must land in the **same commit** or the engine fails at
construction (`actionVerb` errors → `NewCedarEngine` fails → server won't boot). See RESEARCH Pitfall 2.

`engine.go:155` — add `"self": true`:
```go
var knownVerbs = map[string]bool{"read": true, "write": true, "delete": true, "manage": true}
```
`base.cedar:42-48` — model the new permit on the `"manage"` permit, but gate on ANY authenticated role
(add a comment that the handler further restricts source/role-floor, since Cedar only sees `role`/`id`/`email`
via `principalEntity` at `engine.go:212-222`):
```
permit (
	principal,
	action in SpecGraph::Action::"manage",
	resource
) when {
	principal has role && principal.role == "admin"
};
```

### Handler error discipline (return codes, never message strings)
**Source:** `internal/server/identity_handler.go:55-76` (`identityError`)
**Apply to:** all 4 self handlers + `ResyncUserRole`. Reuse `h.identityError(ctx, err)` for storage-sentinel
mapping (`ErrAPIKeyNotFound → CodeNotFound`, `ErrUserNotFound → CodeNotFound`, `ErrAPIKeyPrefixExists →
CodeAborted`). Tests assert on **codes**, not strings (AGENTS.md). New codes this phase:
`CodePermissionDenied` (source gate), `CodeResourceExhausted` (rate-limit + quota), `CodeInvalidArgument`
(expiry over cap).
```go
func (h *IdentityHandler) identityError(ctx context.Context, err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) { return connErr }
	switch {
	case errors.Is(err, storage.ErrUserNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	case errors.Is(err, storage.ErrAPIKeyNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("api key not found"))
	// ...
	default:
		h.logger.ErrorContext(ctx, "identityError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
```

### Owner from context, never from request field
**Source:** `internal/server/identity_handler.go:39-43` (`Whoami`) + `auth.IdentityFromContext` (used at
`auth_handler.go:124`)
**Apply to:** all 4 self handlers. Owner MUST come from `auth.IdentityFromContext(ctx)`; there is NO target
field on the self RPCs (the anti-pattern in RESEARCH §"Anti-Patterns"). `ListMyAPIKeys` MUST hard-set the
storage filter `UserID` from context — passing `msg.GetUserId()` empty into `ListAPIKeys` returns ALL users'
keys (`identity_handler.go:319-324`).
```go
id, ok := auth.IdentityFromContext(ctx)
if !ok {
	return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity in context"))
}
```

### Emit-once secret delivery
**Source:** `internal/server/identity_handler.go:258-262` (`CreateAPIKey`) + CLI `auth_apikey.go:91-99`
**Apply to:** self create/rotate handlers + CLI + web reveal modal. Plaintext returned exactly once via
`auth.FormatAPIKeyToken(prefix, secret)`; never re-serialized. CLI prints once with a "will not be shown
again" line.
```go
plaintext := auth.FormatAPIKeyToken(created.Prefix, secret)
return connect.NewResponse(&specv1.CreateAPIKeyResponse{Key: apiKeyToProto(created), Plaintext: plaintext}), nil
```

### Owner-scoped mutation → uniform NotFound (enumeration-hardening)
**Source:** `internal/storage/postgres/users.go:408-416` (`RevokeAPIKey`) + `RowsAffected()==0` guard used at
`UpdateUserRole` (`:233`)
**Apply to:** all new owner-scoped storage methods (`RevokeAPIKeyForUser`, `RotateAPIKeyForUser`,
`GetAPIKeyForUser`). Add `AND user_id = $caller` to the WHERE clause; `RowsAffected()==0 → storage.ErrAPIKeyNotFound`
so "not yours" / "missing" / "already-revoked" are indistinguishable. See Pattern-1 excerpt below.

### RoleDowngrade must be a built-in role
**Source:** `internal/server/identity_handler.go:231-234` + `auth.IsBuiltinRole` (`identitystore.go:262`)
**Apply to:** self create handler input validation. A non-empty custom `role_downgrade` → `CodeInvalidArgument`.
```go
if d := msg.GetRoleDowngrade(); d != "" && !auth.IsBuiltinRole(auth.Role(d)) {
	return nil, connect.NewError(connect.CodeInvalidArgument,
		errors.New("role_downgrade must be one of: reader, writer, admin"))
}
```

---

## Pattern Assignments

### `internal/auth/rolemin.go` (utility, transform) — NEW

**Analog:** `internal/auth/identitystore.go:254-302` (`roleRank`, `roleLessThan`, `clampedRole`)

Add an **exported** `RoleMin(a, b Role) Role` next to the unexported ordering family. It must be
**fail-closed for unranked** (mirrors `clampedRole`'s spgr-rjrt.9 guarantee: an unranked role on either side
→ `RoleReader`, never fall through to the fuller role). Reuse the existing `roleRank` map — do NOT introduce a
second comparator (RESEARCH §"Don't Hand-Roll").

**Ordering source to reuse** (`identitystore.go:256-302`):
```go
var roleRank = map[Role]int{RoleReader: 1, RoleWriter: 2, RoleAdmin: 3}

func roleLessThan(a, b Role) bool {
	ra, oka := roleRank[a]
	rb, okb := roleRank[b]
	if !oka || !okb { return false }
	return ra < rb
}

func clampedRole(userRole, downgrade Role) Role {
	if downgrade == "" { return userRole }
	_, ownerRanked := roleRank[userRole]
	_, downgradeRanked := roleRank[downgrade]
	if !ownerRanked || !downgradeRanked { return RoleReader } // fail-closed
	if roleLessThan(downgrade, userRole) { return downgrade }
	return userRole
}
```

**New symbol to write** (RESEARCH Pattern 3):
```go
// RoleMin returns the less-privileged of a and b, fail-closed to RoleReader
// when either is unranked (mirrors clampedRole's spgr-rjrt.9 semantics).
func RoleMin(a, b Role) Role {
	ra, oka := roleRank[a]
	rb, okb := roleRank[b]
	if !oka || !okb { return RoleReader }
	if ra <= rb { return a }
	return b
}
```
Package doc comment + SPDX header required (revive/addlicense).

---

### `internal/server/identity_handler.go` (controller, request-response) — MODIFY (4 self handlers + ResyncUserRole)

**Analog:** `CreateAPIKey` (`:223`), `RotateAPIKey` (`:280`), `RevokeAPIKey` (`:266`), `ListAPIKeys` (`:313`),
`UpdateUserRole` (`:155`) — all in this same file.

**Create-key skeleton to copy from** (`identity_handler.go:223-263`) — note the secret generation, the
`storage.APIKey` build, and the emit-once return:
```go
func (h *IdentityHandler) CreateAPIKey(ctx context.Context, req *connect.Request[specv1.CreateAPIKeyRequest]) (*connect.Response[specv1.CreateAPIKeyResponse], error) {
	msg := req.Msg
	if msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	if d := msg.GetRoleDowngrade(); d != "" && !auth.IsBuiltinRole(auth.Role(d)) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role_downgrade must be one of: reader, writer, admin"))
	}
	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		h.logger.ErrorContext(ctx, "CreateAPIKey: generate secret", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	newKey := &storage.APIKey{UserID: msg.GetUserId(), PHCHash: phc, Label: msg.GetLabel(), RoleDowngrade: msg.GetRoleDowngrade()}
	if ts := msg.GetExpiresAt(); ts != nil { t := ts.AsTime(); newKey.ExpiresAt = &t }
	created, err := h.users.CreateAPIKey(ctx, newKey)
	if err != nil { return nil, h.identityError(ctx, err) }
	plaintext := auth.FormatAPIKeyToken(created.Prefix, secret)
	return connect.NewResponse(&specv1.CreateAPIKeyResponse{Key: apiKeyToProto(created), Plaintext: plaintext}), nil
}
```

**What the `CreateMyAPIKey` variant adds** (deltas from the admin version — RESEARCH §"The four gates" + Code Example):
```go
id, ok := auth.IdentityFromContext(ctx)                       // owner from context, NOT msg.UserId
if !ok { return nil, connect.NewError(connect.CodeUnauthenticated, errUnauth) }
if id.Source == "apikey" {                                    // gate F5: anti key-chaining
	return nil, connect.NewError(connect.CodePermissionDenied, errNoKeyChaining)
}
if !h.selfMintLimiter(id.UserID).Allow() {                    // per-identity rate limit (see rateLimiterFor)
	return nil, connect.NewError(connect.CodeResourceExhausted, errRateLimited)
}
downgrade := req.Msg.GetRoleDowngrade()
if downgrade == "" { downgrade = string(id.EffectiveRole) }   // inherit → caller effective, BEFORE RoleMin
floored := auth.RoleMin(auth.Role(downgrade), id.EffectiveRole) // the laundering floor (SC#2)
expiresAt := clampExpiry(req.Msg.GetExpiresAt(), cfg.DefaultTTL, cfg.MaxTTL) // 90d/180d (D-08); over cap → CodeInvalidArgument
// ... call the quota-safe AuthStore mint with owner=id.UserID, role_downgrade=floored ...
```

**RotateMyAPIKey** — model on `RotateAPIKey` (`:280-310`) BUT do NOT inherit old `role_downgrade`/`expires_at`
via the storage nil-fallback (that re-pins a stale ceiling — RESEARCH Pitfall 4). Read the old downgrade via
the new `GetAPIKeyForUser`, then pass **explicit** `RoleMin(old, caller.EffectiveRole)` + a defaulted ≤-cap
`expires_at` into `RotateAPIKeyForUser`. Apply the same `Source=="apikey"` gate + rate limit as create.

**RevokeMyAPIKey / ListMyAPIKeys** — model on `RevokeAPIKey` (`:266`) / `ListAPIKeys` (`:313`). `ListMyAPIKeys`
MUST hard-set `UserID: id.UserID` (never `msg.GetUserId()`) into `storage.ListAPIKeysFilter`.

**ResyncUserRole (AUTH-02)** — reuses `UpdateUserRole` (`:155-175`) + `RevokeAPIKey` + `ListAPIKeys`
(RESEARCH Code Example "AUTH-02 resync handler skeleton"):
```go
func (h *IdentityHandler) ResyncUserRole(ctx context.Context, req ...) (...) {
	if err := h.users.UpdateUserRole(ctx, req.Msg.GetId(), req.Msg.GetRole()); err != nil { // writes users.role → live floor
		return nil, h.identityError(ctx, err)
	}
	if req.Msg.GetRevokeKeys() { // D-02 hard off-board
		keys, err := h.users.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: req.Msg.GetId()})
		if err != nil { return nil, h.identityError(ctx, err) }
		for _, k := range keys { _ = h.users.RevokeAPIKey(ctx, k.ID) }
	}
	// ... GetUserByID + userToProto, mirror UpdateUserRole's return shape ...
}
```
Maps to `user.manage` in `actions.go` (admin-only). The role-derivation *input* stays explicit (`--role`) so a
future automation driver reuses the same entrypoint (D-01 seam).

---

### `internal/storage/postgres/users.go` (storage, CRUD) — MODIFY (owner-scoped methods + quota mint)

**Analog:** `RevokeAPIKey` (`:408`), `RotateAPIKey` (`:425`), `CreateAPIKey` (`:361`) — all on `*AuthStore`,
all using `s.pool` directly (NOT `*Store.RunInTransaction` — ADR-004, RESEARCH-confirmed).

**Owner-scoped revoke** — copy `RevokeAPIKey` (`:408-416`) and add `AND user_id`, plus the `RowsAffected`
guard (Pattern 1):
```go
// existing RevokeAPIKey to copy from:
func (s *AuthStore) RevokeAPIKey(ctx context.Context, keyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE id = $2::uuid AND revoked_at IS NULL`, s.now(), keyID)
	if err != nil { return fmt.Errorf("revoke key: %w", err) }
	return nil
}
// owner-scoped variant adds AND user_id + RowsAffected()==0 → NotFound (revoke: no revoked_at guard, idempotent)
```

**Quota-safe mint** — model the tx on `RotateAPIKey`'s `s.pool.BeginTx` + savepoint pattern (`:425-499`), but
add the parent-row lock (RESEARCH Pattern 2 / Pitfall 5; NOT `count(*) FOR UPDATE`):
```go
tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
defer tx.Rollback(ctx)
if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1::uuid FOR UPDATE`, caller); err != nil { ... }
var n int
tx.QueryRow(ctx, `SELECT count(*) FROM api_keys
	WHERE user_id=$1::uuid AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`, caller).Scan(&n)
if n >= quota { return storage.ErrQuotaExceeded } // new sentinel → CodeResourceExhausted
// INSERT with generated prefix (reuse s.genPrefix + collision retry from CreateAPIKey:368-403), tx.Commit(ctx)
```
The `CreateAPIKey` INSERT…SELECT…WHERE EXISTS (`:377-383`) + prefix-collision retry loop is the insert body to
reuse. **No goose migration** — `api_keys` already has every column (RESEARCH confirms `role_downgrade`,
`expires_at`, `revoked_at`, `user_id`).

**RotateAPIKeyForUser** — copy `RotateAPIKey` (`:425`) but take **explicit** `roleDowngrade`+`expiresAt` args
(never the inherit-on-nil fallback at `:437,:460`) and scope the old-key read + revoke with `AND user_id =
$caller AND revoked_at IS NULL`.

---

### `internal/storage/users.go` (interface, CRUD) — MODIFY

**Analog:** `UsersBackend` API-key CRUD block (`users.go:77-102`)

Add to the interface next to the existing `CreateAPIKey`/`RevokeAPIKey`/`RotateAPIKey`/`ListAPIKeys`:
`GetAPIKeyForUser(ctx, userID, keyID)`, `RevokeAPIKeyForUser(ctx, userID, keyID)`,
`RotateAPIKeyForUser(ctx, userID, keyID, roleDowngrade, expiresAt)` (explicit args), and the quota-safe
`CreateAPIKeyForUser`/count method. Mirror the existing doc-comment style (`:83-92`). Both compile-gate stubs
(`internal/auth/usersbackend_stub_test.go`, `internal/server/usersbackend_stub_test.go`) MUST gain these or the
build breaks.

---

### `internal/auth/actions.go` + `actions_test.go` (registry + test) — MODIFY

**Analog:** Identity block `actions.go:92-106` + `actions_test.go:33-71`

`actions.go` — add 5 entries mirroring the existing style (`:101-104`):
```go
specgraphv1connect.IdentityServiceCreateAPIKeyProcedure: "apikey.manage", // existing
// add:
specgraphv1connect.IdentityServiceCreateMyAPIKeyProcedure: "apikey.self",
specgraphv1connect.IdentityServiceListMyAPIKeysProcedure:  "apikey.self",
specgraphv1connect.IdentityServiceRotateMyAPIKeyProcedure: "apikey.self",
specgraphv1connect.IdentityServiceRevokeMyAPIKeyProcedure: "apikey.self",
specgraphv1connect.IdentityServiceResyncUserRoleProcedure: "user.manage", // AUTH-02, admin-only
```

`actions_test.go` — **TWO hard-coded lists** must be updated (RESEARCH Pitfall 1, the fresh finding):
- `:40` `require.Contains(t, []string{"read", "write", "delete", "manage"}, verb, ...)` → add `"self"`.
- `:50-65` `TestActionForProcedure_Identity` map → add the 5 new procedures.
Also add the mandated **`self`-verb-only-on-`apikey.*` drift test** here (assert no non-`apikey` action ends in
`.self`).

---

### `internal/server/csrf.go` (middleware, request-response) — NEW

**Analog:** `internal/server/auth_handler.go:136-148` (`cookieToAuthHeader`) — same middleware shape

Model a POST-only CSRF-token middleware on `cookieToAuthHeader`'s `http.Handler` wrapper. RESEARCH
recommendation (D-09): double-submit — a `crypto/rand` token set as a non-HttpOnly cookie + echoed header,
validated in a small wrapper around the mutating routes. Cookie config to mirror is `sessionCookie`
(`auth_handler.go:173-182`, `SameSite=Lax`, dynamic `Secure`). Only web mutations need it; the cookie→bearer
promotion already flows through `cookieToAuthHeader`.
```go
func cookieToAuthHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
				r2 := r.Clone(r.Context())
				r2.Header.Set("Authorization", "Bearer "+c.Value)
				r = r2
			}
		}
		next.ServeHTTP(w, r)
	})
}
```

---

### `internal/config/global.go` (config) — MODIFY

**Analog:** `JITCreateConfig` struct (`global.go:226-233`) + defaults in `globalDefaults()` (`:433`)

Add a NEW self-service key-policy struct (expiry default 90d / max 180d, quota 10, rate-limit refill+burst) —
do **NOT** extend the deprecated `APIKeyConfig` (`:254`, "ignored after Authn plan, storage owns"). Model the
struct + yaml/koanf tags on `JITCreateConfig`:
```go
type JITCreateConfig struct {
	Enabled              bool     `yaml:"enabled" koanf:"enabled"`
	DefaultRole          string   `yaml:"default_role" koanf:"default_role"`
	RateLimitPerHour     int      `yaml:"rate_limit_per_hour" koanf:"rate_limit_per_hour"`
	EmailDomainAllowlist []string `yaml:"email_domain_allowlist" koanf:"email_domain_allowlist"`
}
```
Default it in `globalDefaults()` alongside the existing OIDC/JIT defaults (`:433`).

---

### `cmd/specgraph/auth_apikey.go` (CLI, request-response) — MODIFY (self variants)

**Analog:** `authAPIKeyCreateCmd` (`:66-101`), `Rotate` (`:120`), `Revoke` (`:103`), `List` (`:44`)

Self path = no `--user` → call the new `CreateMyAPIKey`/etc RPCs; `--user <other>` keeps the existing admin
path. Reuse `parseExpiresAt` (`:31`), the `printJSON`/`render` output split (`:88-99`), and the emit-once
"Store this token now" line (`:98`). Existing flag wiring in `init()` (`:157-171`) is the template.
```go
resp, err := client.CreateAPIKey(cmd.Context(), connect.NewRequest(&specv1.CreateAPIKeyRequest{
	UserId: apiKeyCreateUser, Label: apiKeyCreateLabel, RoleDowngrade: apiKeyCreateDown, ExpiresAt: expiresAt,
}))
// ... self variant: when apiKeyCreateUser == "" call client.CreateMyAPIKey(...) instead ...
```

**CLI session-precedence fix (Finding D — mandatory)** — see `client.go` assignment below. The self-mint
command MUST authenticate with the stored `spgr_ws_` session, not `SPECGRAPH_API_KEY`, or the `Source=="apikey"`
gate hard-fails `PermissionDenied` on a normal dev box.

---

### `cmd/specgraph/client.go` (CLI client wiring) — MODIFY (Finding D)

**Analog:** `resolveAPIKey` (`client.go:99-111`)

Today precedence is env-key-first:
```go
func resolveAPIKey(serverURL string) string {
	if key := os.Getenv("SPECGRAPH_API_KEY"); key != "" { return key }
	creds, err := credentials.Load(xdg.CredentialsFile())
	if err != nil { return "" }
	return creds.TokenFor(serverURL)
}
```
Self-mint needs a path that prefers the stored session credential (a `spgr_ws_` token in the credentials file,
`login.go:257`) and ignores/warns on a set `SPECGRAPH_API_KEY`. Add a session-preferring resolver (or a flag on
the existing one) used only by the self-mint/self-key commands. `newAuthenticatedHTTPClient` (`:123`) is where
the token gets injected.

---

### `cmd/specgraph/auth_user.go` (CLI) — MODIFY (resync subcommand)

**Analog:** `authUserSetRoleCmd` (`:80-99`)

Add a `resync` cobra command mirroring `set-role`'s shape: positional `<user-id>`, `--role` flag,
`--revoke-keys` bool, `printJSON`/`render` output split (`:93-97`). Calls the new `ResyncUserRole` RPC.
```go
var authUserSetRoleCmd = &cobra.Command{
	Use:  "set-role <user-id> <role>",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil { return err }
		resp, err := client.UpdateUserRole(cmd.Context(), connect.NewRequest(&specv1.UpdateUserRoleRequest{Id: args[0], Role: args[1]}))
		if err != nil { return fmt.Errorf("set role: %w", err) }
		if authJSON { return printJSON(cmd.OutOrStdout(), resp.Msg) }
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Updated %s → role %s\n", args[0], resp.Msg.GetUser().GetRole())
		return err
	},
}
```
Register under `authUserCmd` in `init()` like `set-role`/`delete`/`purge`.

---

### `proto/specgraph/v1/identity.proto` (schema) — MODIFY

**Analog:** existing API-key RPCs + messages (`:26-32`, `:126-168`)

Add 4 self RPCs + `ResyncUserRole` to the `IdentityService` block (currently ends at `UnbindOIDC`, `:32`) plus
their req/resp messages. Model `CreateMyAPIKeyRequest` on `CreateAPIKeyRequest` (`:126-131`) **minus** `user_id`
(owner is from context). Model `ResyncUserRoleRequest` on `UpdateUserRoleRequest` (`:108-112`) + a
`bool revoke_keys` field. Follow the existing `reserved`-on-removed-field convention (`:150-154`). Then
`task proto` regenerates `gen/` (committed; do NOT hand-edit — PreToolUse hook blocks it). Skill `buf-regen`
documents the flow.
```proto
message CreateAPIKeyRequest {
  string user_id = 1;
  string label = 2;
  string role_downgrade = 3;
  google.protobuf.Timestamp expires_at = 4;
}
// CreateMyAPIKeyRequest: same MINUS user_id (owner from IdentityFromContext).
```

---

### `web/src/routes/keys/+page.svelte` + `web/src/lib/keys.svelte.ts` + `client.ts` (net-new panel)

**Analogs:** read-only routes `web/src/routes/graph/+page.svelte`, `constitution/+page.svelte` (route shell);
`web/src/lib/auth.svelte.ts` (state + fetch/POST); `web/src/lib/api/client.ts:35-41` (client wiring);
`web/src/lib/components/LoginModal.svelte` (modal overlay/form for the one-time reveal).

**Client wiring** — add an `identityClient` next to the existing typed clients (`client.ts:35-41`). The generated
`identity_pb.ts` already exists under `src/lib/api/gen/specgraph/v1/`:
```ts
export const graphClient = createClient(GraphService, transport);
export const specClient = createClient(SpecService, transport);
// add:
export const identityClient = createClient(IdentityService, transport);
```
The `projectInterceptor` + `authErrorInterceptor` (`client.ts:14-28`) already handle the project header and
401→`onUnauthenticated`. For the CSRF-token echo on mutations, add a third interceptor here (double-submit
header from the non-HttpOnly cookie).

**State module** — model `keys.svelte.ts` on `auth.svelte.ts` (Svelte 5 `$state` + `fetch`/POST + status
handling, `auth.svelte.ts:7-49`):
```ts
let authenticated = $state(false);
let identity = $state<Identity | null>(null);
export async function login(key: string): Promise<boolean> {
  const resp = await fetch('/api/auth/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ key }),
  });
  // ...
}
```

**Reveal modal** — model `RevealKeyModal.svelte` on `LoginModal.svelte` (overlay + card + form, full `<style>`
block reusable at `LoginModal.svelte:66-117`). Show the plaintext ONCE with a copy button + secret-manager
instruction; no re-fetch.

---

## No Analog Found (or partial)

| File | Role | Data Flow | Note |
|------|------|-----------|------|
| `internal/auth/rolemin_test.go` | test | — | No table-test analog for role ordering exists; write fresh against the fail-closed matrix. Convention: `testify/require` as in `actions_test.go`. |
| `web/src/lib/components/RevealKeyModal.svelte` | component | — | `LoginModal.svelte` gives the overlay/form/style shell, but the one-time-reveal + copy-to-clipboard behavior is net-new (no existing reveal-once component). |
| `internal/server/csrf.go` | middleware | request-response | `cookieToAuthHeader` gives the middleware *shape*; the double-submit token logic (issue/validate) is net-new — no existing CSRF code in the repo. |

---

## Metadata

**Analog search scope:** `internal/auth/`, `internal/server/`, `internal/storage/{,postgres/}`, `internal/config/`,
`cmd/specgraph/`, `proto/specgraph/v1/`, `web/src/{routes,lib}/`
**Files scanned (read this session):** `identity_handler.go`, `auth_handler.go`, `postgres/users.go`,
`storage/users.go`, `identitystore.go`, `engine.go`, `actions.go`, `actions_test.go`, `base.cedar`, `auth.go`,
`auth_apikey.go`, `auth_user.go`, `client.go`, `identity.proto`, `global.go`, `web/src/lib/api/client.ts`,
`web/src/lib/auth.svelte.ts`, `web/src/lib/components/LoginModal.svelte`, web route/lib tree
**Key conventions grounded:** ConnectRPC handler + `identityError` code-mapping; `*AuthStore` bespoke `pool.BeginTx`
(ADR-004, NOT `RunInTransaction`); `RowsAffected()==0 → sentinel` guard; cobra `--json`/`render` output split;
Svelte 5 `$state` runes + `createConnectTransport`; `task proto`-regenerated committed `gen/`; SPDX+DCO+revive
housekeeping.
**Pattern extraction date:** 2026-07-09
