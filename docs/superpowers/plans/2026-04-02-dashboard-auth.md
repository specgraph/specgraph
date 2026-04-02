# Dashboard Authentication Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add cookie-based authentication to the SpecGraph web dashboard so it works when API keys are configured.

**Architecture:** Server-side login/logout/whoami HTTP endpoints validate API keys and set httpOnly session cookies. The existing ConnectRPC interceptor and REST middleware gain cookie fallback. The SvelteKit frontend adds an auth state module and login modal that gates all dashboard content.

**Tech Stack:** Go (net/http handlers), SvelteKit 5 (Svelte runes), ConnectRPC (connect-web interceptors)

**Spec:** `docs/superpowers/specs/2026-04-02-dashboard-auth-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `internal/server/auth_handler.go` | Login/logout/whoami HTTP handlers, cookie helpers |
| `internal/server/auth_handler_test.go` | Handler tests |
| `web/src/lib/auth.svelte.ts` | Reactive auth state, login/logout/checkAuth functions |
| `web/src/lib/components/LoginModal.svelte` | API key input modal |

### Modified Files

| File | Change |
|------|--------|
| `internal/auth/interceptor.go` | Cookie fallback in `resolveIdentity`, remove local bypass |
| `internal/auth/interceptor_test.go` | Update tests for cookie fallback + no local bypass |
| `internal/auth/middleware.go` | Cookie fallback in `authenticate`, change signature, remove local bypass |
| `internal/auth/middleware_test.go` | Update tests for cookie fallback + no local bypass |
| `cmd/specgraph/serve.go` | Register auth endpoints |
| `web/src/lib/api/client.ts` | Add auth error interceptor |
| `web/src/routes/+layout.svelte` | Auth check + conditional LoginModal |

---

## Chunk 1: Server-Side (Tasks 1-4)

### Task 1: Auth Handler — Login/Logout/Whoami

**Files:**

- Create: `internal/server/auth_handler.go`
- Create: `internal/server/auth_handler_test.go`

**Reference:** Read `internal/server/api_handler.go` for the existing REST handler
pattern. Read `internal/auth/auth.go` for the `Identity` type. Read
`internal/auth/store.go` for `IdentityStore` interface.

- [ ] **Step 1: Write tests for login handler**

Create `internal/server/auth_handler_test.go`. Tests:
- `TestLogin_ValidKey` — POST with valid key returns 200 + identity JSON + Set-Cookie header
- `TestLogin_InvalidKey` — POST with bad key returns 401
- `TestLogin_MissingKey` — POST with empty body returns 400
- `TestLogin_WrongContentType` — POST with `application/x-www-form-urlencoded` returns 415

Use a test `IdentityStore` that recognizes one key. Check the `Set-Cookie`
header for `specgraph_session`, `HttpOnly`, `SameSite=Strict`, `Path=/api`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -run TestLogin -v`

- [ ] **Step 3: Implement auth_handler.go**

Create `internal/server/auth_handler.go`:

```go
// RegisterAuthHandlers registers login/logout/whoami endpoints.
// Login and logout are unauthenticated. Whoami requires auth middleware.
func RegisterAuthHandlers(mux *http.ServeMux, store auth.IdentityStore, authMW func(http.Handler) http.Handler)
```

**Login handler:**
1. Validate `Content-Type: application/json` (return 415 if not)
2. Decode `{"key": "..."}` from body
3. Call `store.ResolveAPIKey(ctx, key)`
4. On success: `http.SetCookie` with specgraph_session cookie, write identity JSON
5. On failure: 401

**Cookie helper:**
```go
func sessionCookie(value string, r *http.Request) *http.Cookie {
    return &http.Cookie{
        Name:     "specgraph_session",
        Value:    value,
        Path:     "/api",
        HttpOnly: true,
        SameSite: http.SameSiteStrictMode,
        Secure:   r.TLS != nil,
    }
}
```

**Logout handler:** Set cookie with `MaxAge: -1` to delete it. Return 204.

**Whoami handler:** Read cookie, resolve via store, return identity. Wrapped
in `authMW` — the middleware handles the 401 if no valid credential.
Actually, whoami should read the identity from context (already resolved by
middleware). Extract via `auth.IdentityFromContext(r.Context())`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestLogin -v`

- [ ] **Step 5: Write tests for logout and whoami**

Tests:
- `TestLogout_ClearsCookie` — POST returns 204 + Set-Cookie with MaxAge=-1
- `TestWhoami_Authenticated` — GET with valid cookie returns 200 + identity
- `TestWhoami_Unauthenticated` — GET with no cookie returns 401

- [ ] **Step 6: Run tests, verify pass**

Run: `go test ./internal/server/ -run "TestLogout|TestWhoami" -v`

- [ ] **Step 7: Commit**

```
feat(auth): add login/logout/whoami HTTP handlers for dashboard auth
```

---

### Task 2: Interceptor Cookie Fallback + Remove Local Bypass

**Files:**

- Modify: `internal/auth/interceptor.go`
- Modify: `internal/auth/interceptor_test.go`

**Reference:** Read `internal/auth/interceptor.go` (current implementation,
107 lines). Read `internal/auth/interceptor_test.go` for test patterns.

- [ ] **Step 1: Update existing tests**

In `interceptor_test.go`:
- Change test for "no auth header + no auth configured" — previously expected
  200 with local identity. Now should expect 401 (CodeUnauthenticated).
- Add test: "cookie auth + valid key" — request with `Cookie: specgraph_session=<valid-key>`
  header returns 200.
- Add test: "cookie auth + invalid key" — returns 401.
- Add test: "bearer header takes precedence over cookie" — request with both
  Authorization header and cookie uses the header.

- [ ] **Step 2: Run tests to verify failures** (old behavior doesn't match new expectations)

Run: `go test ./internal/auth/ -run TestAuth -v`

- [ ] **Step 3: Modify resolveIdentity in interceptor.go**

Replace lines 64-93:

```go
func resolveIdentity(ctx context.Context, store IdentityStore, headers http.Header) (*Identity, error) {
    authHeader := headers.Get("Authorization")

    if authHeader != "" {
        // Parse "Bearer <token>"
        scheme, token, ok := strings.Cut(authHeader, " ")
        token = strings.TrimSpace(token)
        if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
            return nil, connect.NewError(connect.CodeUnauthenticated, nil)
        }
        return resolveToken(ctx, store, token)
    }

    // Fallback: check session cookie.
    r := &http.Request{Header: headers}
    cookie, err := r.Cookie("specgraph_session")
    if err == nil && cookie.Value != "" {
        return resolveToken(ctx, store, cookie.Value)
    }

    return nil, connect.NewError(connect.CodeUnauthenticated, nil)
}

func resolveToken(ctx context.Context, store IdentityStore, token string) (*Identity, error) {
    id, err := store.ResolveAPIKey(ctx, token)
    if err == nil {
        return id, nil
    }
    if errors.Is(err, ErrUnknownKey) {
        return nil, connect.NewError(connect.CodeUnauthenticated, nil)
    }
    return nil, connect.NewError(connect.CodeInternal, nil)
}
```

Remove `localIdentity()` function entirely. Remove `"os/user"` import.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -run TestAuth -v`

- [ ] **Step 5: Commit**

```
feat(auth): add cookie fallback to ConnectRPC interceptor, remove local bypass
```

---

### Task 3: Middleware Cookie Fallback + Remove Local Bypass

**Files:**

- Modify: `internal/auth/middleware.go`
- Modify: `internal/auth/middleware_test.go`

**Reference:** Read `internal/auth/middleware.go` (50 lines). Read
`internal/auth/middleware_test.go` for test patterns.

- [ ] **Step 1: Update existing tests**

In `middleware_test.go`:
- Change test for "no auth + no keys" — previously 200 with local identity.
  Now should be 401.
- Add test: "cookie auth" — request with session cookie returns 200.
- Add test: "header takes precedence over cookie" — both present, header wins.

- [ ] **Step 2: Run tests to verify failures**

Run: `go test ./internal/auth/ -run TestMiddleware -v`

- [ ] **Step 3: Modify authenticate in middleware.go**

Change `authenticate` signature from `(ctx, store, authHeader string)` to
`(ctx context.Context, store IdentityStore, r *http.Request)`:

```go
func authenticate(ctx context.Context, store IdentityStore, r *http.Request) (*Identity, bool) {
    authHeader := r.Header.Get("Authorization")
    if authHeader != "" {
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

    // Fallback: session cookie.
    cookie, err := r.Cookie("specgraph_session")
    if err == nil && cookie.Value != "" {
        id, storeErr := store.ResolveAPIKey(ctx, cookie.Value)
        if storeErr != nil {
            return nil, false
        }
        return id, true
    }

    return nil, false
}
```

Update `RequireAuth` caller from `authenticate(r.Context(), store, r.Header.Get("Authorization"))`
to `authenticate(r.Context(), store, r)`.

Remove `localIdentity()` usage. Note: `localIdentity` is defined in
`interceptor.go`, not `middleware.go`, so just remove the call — don't
delete the function here (Task 2 already deleted it).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -v`

- [ ] **Step 5: Commit**

```
feat(auth): add cookie fallback to REST middleware, remove local bypass
```

---

### Task 4: Register Auth Endpoints in serve.go

**Files:**

- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Add RegisterAuthHandlers call**

After line 228 (`server.RegisterAPIHandlers(...)`), add:

```go
server.RegisterAuthHandlers(mux, compositeStore, auth.RequireAuth(compositeStore))
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/...`

- [ ] **Step 3: Commit**

```
feat(auth): register dashboard auth endpoints in serve command
```

---

## Chunk 2: Frontend (Tasks 5-7)

### Task 5: Auth State Module

**Files:**

- Create: `web/src/lib/auth.svelte.ts`

- [ ] **Step 1: Create auth state module**

```typescript
// Auth state for the dashboard.
// Uses Svelte 5 runes for reactive state.

interface Identity {
  subject: string;
  displayName: string;
  role: string;
}

let authenticated = $state(false);
let identity = $state<Identity | null>(null);

export const auth = {
  get authenticated() { return authenticated; },
  get identity() { return identity; },
};

export async function checkAuth(): Promise<void> {
  try {
    const resp = await fetch('/api/auth/whoami');
    if (resp.ok) {
      const data = await resp.json();
      identity = data.identity;
      authenticated = true;
    } else {
      identity = null;
      authenticated = false;
    }
  } catch {
    identity = null;
    authenticated = false;
  }
}

export async function login(key: string): Promise<boolean> {
  const resp = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key }),
  });
  if (resp.ok) {
    const data = await resp.json();
    identity = data.identity;
    authenticated = true;
    return true;
  }
  return false;
}

export async function logout(): Promise<void> {
  await fetch('/api/auth/logout', { method: 'POST' });
  identity = null;
  authenticated = false;
}

export function onUnauthenticated(): void {
  identity = null;
  authenticated = false;
}
```

- [ ] **Step 2: Commit**

```
feat(web): add auth state module with login/logout/checkAuth
```

---

### Task 6: Login Modal + Auth Error Interceptor

**Files:**

- Create: `web/src/lib/components/LoginModal.svelte`
- Modify: `web/src/lib/api/client.ts`

- [ ] **Step 1: Create LoginModal component**

Create `web/src/lib/components/LoginModal.svelte`:

```svelte
<script>
  import { login } from '$lib/auth.svelte';

  let key = $state('');
  let error = $state('');
  let loading = $state(false);

  let { onSuccess } = $props();

  async function handleSubmit(e: Event) {
    e.preventDefault();
    error = '';
    loading = true;
    const ok = await login(key);
    loading = false;
    if (ok) {
      onSuccess();
    } else {
      error = 'Invalid API key. Check your key and try again.';
      key = '';
    }
  }
</script>

<div class="overlay">
  <form class="login-card" onsubmit={handleSubmit}>
    <h2>SpecGraph</h2>
    <p>Enter your API key to continue.</p>
    <input
      type="password"
      bind:value={key}
      placeholder="spgr_sk_..."
      autocomplete="off"
      disabled={loading}
    />
    {#if error}
      <p class="error">{error}</p>
    {/if}
    <button type="submit" disabled={!key || loading}>
      {loading ? 'Authenticating...' : 'Sign in'}
    </button>
  </form>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .login-card {
    background: white;
    border-radius: 8px;
    padding: 2rem;
    width: 360px;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.15);
  }
  h2 { margin: 0 0 0.5rem; color: #1a1a2e; }
  p { color: #64748b; font-size: 0.9rem; margin: 0 0 1rem; }
  input {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid #d1d5db;
    border-radius: 4px;
    font-size: 0.9rem;
    box-sizing: border-box;
    margin-bottom: 0.75rem;
  }
  input:focus { outline: 2px solid #3b82f6; border-color: transparent; }
  .error { color: #ef4444; font-size: 0.85rem; }
  button {
    width: 100%;
    padding: 0.5rem;
    background: #1a1a2e;
    color: white;
    border: none;
    border-radius: 4px;
    font-size: 0.9rem;
    cursor: pointer;
  }
  button:hover:not(:disabled) { background: #2d2d4e; }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
```

- [ ] **Step 2: Add auth error interceptor to client.ts**

Modify `web/src/lib/api/client.ts`:

Add import:
```typescript
import { ConnectError, Code } from '@connectrpc/connect';
import { onUnauthenticated } from '$lib/auth.svelte';
```

Add interceptor:
```typescript
const authErrorInterceptor: Interceptor = (next) => async (req) => {
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

Update transport interceptors:
```typescript
interceptors: [projectInterceptor, authErrorInterceptor],
```

- [ ] **Step 3: Commit**

```
feat(web): add LoginModal component and auth error interceptor
```

---

### Task 7: Layout Integration

**Files:**

- Modify: `web/src/routes/+layout.svelte`

- [ ] **Step 1: Update layout to gate on auth**

Replace the `<script>` block and conditional rendering:

```svelte
<script>
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { auth, checkAuth } from '$lib/auth.svelte';
  import { project, loadProjects } from '$lib/project.svelte';
  import LoginModal from '$lib/components/LoginModal.svelte';

  let { children } = $props();
  let ready = $state(false);

  onMount(async () => {
    await checkAuth();
    if (auth.authenticated) {
      await loadProjects();
    }
    ready = true;
  });

  async function handleLoginSuccess() {
    await loadProjects();
  }
</script>
```

Update the `<main>` section:

```svelte
{#if !ready}
  <main><p class="loading">Connecting...</p></main>
{:else if !auth.authenticated}
  <LoginModal onSuccess={handleLoginSuccess} />
{:else}
  <nav>
    <!-- existing nav content unchanged -->
  </nav>
  <main>
    {#if project.loaded}
      {@render children()}
    {:else}
      <p class="loading">Loading projects...</p>
    {/if}
  </main>
{/if}
```

The nav is now inside the authenticated block — you don't see navigation
until logged in.

- [ ] **Step 2: Verify the frontend builds**

Run: `cd web && pnpm build`

- [ ] **Step 3: Commit**

```
feat(web): gate dashboard on authentication with login modal
```

---

## Task Dependency Graph

```text
Task 1 (auth handlers)
  └→ Task 2 (interceptor cookie fallback)
       └→ Task 3 (middleware cookie fallback)
            └→ Task 4 (register in serve.go)

Task 5 (auth state module) — independent
  └→ Task 6 (LoginModal + error interceptor)
       └→ Task 7 (layout integration)

Tasks 1-4 and 5-7 are independent branches that can be parallelized.
```
