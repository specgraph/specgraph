# Dashboard Authentication Design

**Date:** 2026-04-02
**Status:** Approved

## Summary

Add authentication to the SpecGraph web dashboard. Users enter an API key in a
login modal; the server validates it and sets an httpOnly session cookie. All
subsequent requests (ConnectRPC and REST) carry the cookie automatically. No
local-mode bypass — all clients authenticate, even on localhost.

## Motivation

The dashboard currently makes unauthenticated API calls. It only works when auth
is not configured (local-mode admin bypass). With API keys configured, every
dashboard request returns 401. This blocks any non-local deployment.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Token storage | httpOnly cookie | XSS-safe, automatic on every request, paves road to OIDC |
| Login UI | Inline modal on 401 | Single field, keeps user in context, minimal overhead |
| Local mode bypass | No bypass | Consistent with CLI policy — all clients authenticate |
| CSRF protection | SameSite=Strict + CORS | ConnectRPC uses POST+JSON (not simple form), preflight blocks cross-origin |
| Session state | Stateless (raw key in cookie) | Server re-validates key on every request via store, no session table |
| Cookie expiry | Session cookie (no Max-Age) | Cleared on browser close or explicit logout |

## Server-Side Changes

### New Endpoints

Three new HTTP endpoints registered alongside `/api/projects`:

**`POST /api/auth/login`**

Request: `{"key": "spgr_sk_..."}`

Response (200): `{"identity": {"subject": "apikey:admin1", "display_name": "Admin", "role": "admin"}}`

Response (401): `{"error": "invalid API key"}`

Flow:
1. Parse JSON body, extract `key`
2. Call `store.ResolveAPIKey(ctx, key)`
3. On success: set `specgraph_session` cookie, return identity
4. On failure: return 401

**`POST /api/auth/logout`**

Response: 204 No Content

Flow: Set `specgraph_session` cookie with `Max-Age=0` (immediate expiry).

**`GET /api/auth/whoami`**

Response (200): `{"identity": {"subject": "...", "display_name": "...", "role": "..."}}`

Response (401): `{"error": "not authenticated"}`

Flow: Read cookie, resolve identity via store, return it. Used by the frontend
to check auth state on page load.

### Cookie Configuration

```
Name:     specgraph_session
Value:    <raw API key>
HttpOnly: true
SameSite: Strict
Secure:   true (when request is HTTPS)
Path:     /api
```

`Path: /api` limits the cookie to API endpoints — it is not sent for static
assets or SvelteKit page loads. The raw API key is visible in browser devtools
cookie inspector; this is inherent to the stateless session design and acceptable
for API key auth. OIDC migration will replace the raw key with an opaque session
token.

No `Max-Age` or `Expires` — session cookie, cleared when browser closes.

The `Secure` flag is set when the request uses HTTPS. For `localhost` development
over plain HTTP, `Secure` is omitted so the cookie works without TLS.

### Interceptor Change

`internal/auth/interceptor.go` — `resolveIdentity`:

Current flow:
1. Read `Authorization: Bearer <token>` header
2. If missing and `!HasAuth()`: return local identity
3. If missing and `HasAuth()`: return 401

New flow:
1. Read `Authorization: Bearer <token>` header
2. If present: validate token (unchanged)
3. If missing: read `specgraph_session` cookie
4. If cookie present: validate cookie value via `store.ResolveAPIKey(ctx, cookieValue)`
5. If neither header nor cookie: return 401

The local-mode bypass (`!HasAuth()` → admin) is removed. All requests must
authenticate via header or cookie. This is a deliberate breaking change — see
Migration below.

**Implementation note:** `resolveIdentity` receives `http.Header`. Extract the
cookie by parsing the `Cookie` header: `(&http.Request{Header: headers}).Cookie("specgraph_session")`.
This avoids changing the function signature.

### REST Middleware Change

`internal/auth/middleware.go` — `authenticate`:

Same fallback: if no `Authorization` header, check the `specgraph_session`
cookie. The current signature is `authenticate(ctx, store, authHeader string)`.
Change to `authenticate(ctx, store, r *http.Request)` to access both the
Authorization header and cookies from the request.

### CSRF Analysis

No CSRF token is needed because:

1. **SameSite=Strict** prevents the browser from sending the cookie on
   cross-origin requests.
2. **ConnectRPC** uses `POST` with `Content-Type: application/json`, which is
   not a "simple" request. Browsers send a CORS preflight (OPTIONS) for
   non-simple requests. Without the server allowing the attacker's origin,
   the preflight fails and the request is never sent.
3. **REST endpoints** (`/api/auth/*`, `/api/projects`) also use JSON bodies
   with POST, triggering the same preflight protection.

**Content-Type enforcement:** The login handler MUST validate
`Content-Type: application/json` and reject other types (especially
`application/x-www-form-urlencoded`). Without this, the CORS preflight
protection is bypassed for simple form submissions and `SameSite=Strict`
becomes the sole CSRF defense.

### Migration: Local Bypass Removal

Removing the local-mode bypass is a deliberate breaking change. Previously,
`specgraph serve` with no auth config auto-granted admin to all requests.
Now all requests must authenticate.

The migration path is already in place:
1. On first `specgraph serve`, `auth.Bootstrap` generates a default admin key
   (`spgr_sk_...`) and stores it in the credentials file
2. The key is printed to stderr on interactive terminals
3. CLI users set the key via `SPECGRAPH_API_KEY` env var or `--api-key` flag
4. Dashboard users paste the key into the login modal

No config changes needed — bootstrap handles the key generation automatically.

### Auth Endpoint Registration

New file: `internal/server/auth_handler.go`

Registered in `serve.go` alongside `RegisterAPIHandlers`. The login/logout
endpoints do NOT go through the auth middleware (login must work without
credentials). The whoami endpoint does go through auth middleware.

Actually — login validates the key itself, so it handles its own auth. The
pattern:

```
/api/auth/login   — no middleware (validates key in handler)
/api/auth/logout  — no middleware (just clears cookie)
/api/auth/whoami  — auth middleware (returns identity from cookie)
```

## Frontend Changes

### Auth State Module

New file: `web/src/lib/auth.svelte.ts`

Reactive state using Svelte 5 runes:

```typescript
let authenticated = $state(false);
let identity = $state<{subject: string; displayName: string; role: string} | null>(null);
```

Functions:
- `checkAuth()` — GET `/api/auth/whoami`, update state
- `login(key: string): Promise<boolean>` — POST `/api/auth/login`, update state
- `logout(): Promise<void>` — POST `/api/auth/logout`, clear state
- `onUnauthenticated()` — called by error interceptor, flips `authenticated` to false

### Login Modal Component

New file: `web/src/lib/components/LoginModal.svelte`

- Single masked text input for the API key
- Submit button
- Error message on invalid key ("Invalid API key. Check your key and try again.")
- Cannot be dismissed — must authenticate
- Matches existing dashboard styling

### ConnectRPC Error Interceptor

Update `web/src/lib/api/client.ts`:

Add an interceptor that catches `Code.Unauthenticated` responses and calls
`onUnauthenticated()` to trigger the login modal:

```typescript
const authInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (err) {
    if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
      onUnauthenticated();
    }
    throw err;
  }
};
```

### Layout Integration

Update `web/src/routes/+layout.svelte`:

```
onMount:
  1. checkAuth()
  2. if authenticated: loadProjects()
  3. if not authenticated: show LoginModal

LoginModal onSuccess:
  1. loadProjects()
  2. render dashboard
```

The layout conditionally renders either the LoginModal or the dashboard content
based on `authenticated` state. When `onUnauthenticated()` fires (from a 401
during use), the modal reappears.

**Note:** A 401 from `whoami` on initial page load is the expected "not logged
in" signal, not an error. The frontend should not display error UI for this case.
The auth error interceptor for ConnectRPC calls should trigger the modal without
also showing an error toast — the re-throw is for callers that need to abort
their operation, not for user-facing error display.

## What Stays Unchanged

- API key config format (`auth.api_keys` in config.yaml)
- Key generation and bootstrap (`auth.Bootstrap`)
- OIDC provider config and resolution
- CompositeStore routing logic
- CLI authentication (uses `Authorization` header, unaffected by cookie changes)
- Permission model and RPC permission mapping
- Project middleware (`X-Specgraph-Project` header)

## OIDC Migration Path

When OIDC is added to the dashboard later:

1. Replace the login modal with a "Sign in with [Provider]" button
2. Add an OAuth2 authorization code flow: redirect to provider → callback
   endpoint → exchange code for tokens → set session cookie with JWT
3. The interceptor already handles JWTs via `CompositeStore.ResolveJWT`
4. The cookie-based session infrastructure built here is reused — only the
   login mechanism changes (API key form → OIDC redirect)

## Files Changed

### New Files

| File | Purpose |
|------|---------|
| `internal/server/auth_handler.go` | Login/logout/whoami HTTP handlers |
| `web/src/lib/auth.svelte.ts` | Auth state module |
| `web/src/lib/components/LoginModal.svelte` | Login modal component |

### Modified Files

| File | Change |
|------|--------|
| `internal/auth/interceptor.go` | Add cookie fallback in `resolveIdentity`, remove local bypass |
| `internal/auth/middleware.go` | Add cookie fallback in `authenticate`, remove local bypass |
| `cmd/specgraph/serve.go` | Register auth endpoints |
| `web/src/lib/api/client.ts` | Add auth error interceptor |
| `web/src/routes/+layout.svelte` | Add auth check + conditional LoginModal |

### Test Files

| File | Purpose |
|------|---------|
| `internal/server/auth_handler_test.go` | Login/logout/whoami handler tests |
| `internal/auth/interceptor_test.go` | Update for cookie fallback + no local bypass |
| `internal/auth/middleware_test.go` | Update for cookie fallback + no local bypass |
