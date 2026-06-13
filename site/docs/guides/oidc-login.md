# Interactive OIDC Login

SpecGraph can present a browser-based **"Sign in with &lt;provider&gt;"** flow so
that human operators authenticate through your identity provider (IdP) instead
of pasting a bearer token. This guide covers enabling interactive login,
configuring real providers (Microsoft Entra ID, Okta, GitHub via a broker), and
troubleshooting the common failure modes.

## Overview

Interactive login runs a **server-side OAuth2 Authorization Code flow with
PKCE**. SpecGraph drives the handshake against the provider, validates the
returned ID token, and mints a **SpecGraph-issued opaque server-side session**.

- The session lives server-side; the browser cookie holds only an opaque
  session id. All handshake state (PKCE verifier, nonce, requested provider)
  is kept server-side, so the flow is **multi-replica safe** — no sticky
  sessions and no session affinity are required at your load balancer.
- Session lifetime is controlled by `auth.oidc.session_ttl` (default `12h`).

This is **distinct from the existing bearer-token OIDC verification** used by
the CLI and API. That path validates an ID token presented in the
`Authorization` header against the provider's JWKS. Interactive login is
**opt-in per provider** — set `interactive: true` on the provider you want to
expose as a sign-in button. A provider can serve bearer-token verification,
interactive login, or both.

The interactive flow uses three endpoints:

- `GET /api/auth/oidc/providers` — lists the providers with `interactive: true`
  (used to render the sign-in buttons).
- `GET /api/auth/oidc/{provider}/start` — begins the Authorization Code + PKCE
  handshake and redirects the browser to the IdP.
- `GET /api/auth/oidc/callback` — the IdP redirect target; exchanges the code,
  validates the token, and establishes the session.

## Common Configuration

The following settings apply to every interactive provider regardless of the
IdP behind it.

### Redirect URI

The redirect URI is **always** `https://<host>/api/auth/oidc/callback`. Register
**exactly** this URI at the IdP — an exact match, with no wildcards. All
providers share this single callback path.

### `base_url` behind a reverse proxy

!!! warning "Required behind a host-rewriting proxy or load balancer"
    `auth.oidc.base_url` is **REQUIRED** whenever SpecGraph sits behind a
    reverse proxy or load balancer that rewrites the `Host` header.

Without `base_url`, SpecGraph derives the redirect URI from request headers
(`X-Forwarded-Host` / `X-Forwarded-Proto`). Behind an untrusted or
host-rewriting proxy, that derivation can produce a **wrong redirect URI** — the
handshake will then fail the IdP's exact-match check, or (worse) be steered to
an attacker-influenced host. Setting `base_url` makes the redirect URI
deterministic and independent of inbound headers. It is the **recommended
production setting**.

```yaml
auth:
  oidc:
    base_url: https://specgraph.example.com
```

### Trusted proxy and rate limiting

The login start and callback endpoints are protected by a **per-IP rate
limiter**. When SpecGraph sits behind a trusted load balancer, set
`server.trusted_proxy: true` so the limiter keys on the **real client IP** from
`X-Forwarded-For` rather than the proxy's IP. Without this, all clients share
the proxy IP and a single user can exhaust the limit for everyone.

```yaml
server:
  trusted_proxy: true
```

!!! danger "Only enable `trusted_proxy` behind a trusted proxy"
    `X-Forwarded-For` is client-spoofable when the request reaches SpecGraph
    directly. Enable `trusted_proxy` **only** when a trusted load balancer
    terminates and rewrites the header.

### Client secrets

Provide the OAuth client secret via the environment, never in committed config:

```yaml
providers:
  - id: entra
    client_secret_env: SPECGRAPH_OIDC_ENTRA_SECRET
```

`client_secret_env` names an environment variable that SpecGraph reads at
startup. Do **not** commit a plaintext `client_secret` to your config file. If
the named environment variable is unset, SpecGraph treats it as a **fatal
startup error** and refuses to boot — failing closed rather than running a
provider with no secret.

### JIT provisioning and the email allowlist

Interactive logins **bypass the per-issuer JIT (just-in-time) rate limiter**.
That limiter exists to throttle bearer-token first-contact provisioning;
interactive logins are user-driven and already gated by the IdP, PKCE, and a
server-side nonce, so they are not subject to it.

Because that throttle does not apply, **any internet-facing deployment with JIT
enabled must bound who can self-provision** using
`auth.oidc.jit_create.email_domain_allowlist`:

```yaml
auth:
  oidc:
    jit_create:
      enabled: true
      default_role: reader
      email_domain_allowlist: [example.com]
```

Only verified-email users whose domain is on the allowlist will have an account
created on first sign-in. Everyone else is rejected (see Troubleshooting).

### Logout caveat

Logout revokes the **SpecGraph server session** — the opaque session id is
invalidated and the cookie cleared. It does **not** terminate the **IdP SSO
session**. A subsequent click on "Sign in" may **silently re-authenticate** the
same user if the IdP still considers them logged in.

!!! note "Shared-machine caveat"
    On shared or kiosk machines, SpecGraph logout alone does not sign the user
    out of the IdP. Operators on shared hardware should also sign out at the
    IdP (or close the browser session) to prevent silent re-authentication.

## Microsoft Entra ID

A complete provider block for Microsoft Entra ID (formerly Azure AD):

```yaml
auth:
  oidc:
    base_url: https://specgraph.example.com
    session_ttl: 12h
    providers:
      - id: entra
        interactive: true
        display_name: Microsoft Entra
        issuer: https://login.microsoftonline.com/<tenant-id>/v2.0
        client_id: <application-client-id>
        client_secret_env: SPECGRAPH_OIDC_ENTRA_SECRET
        scopes: [openid, profile, email]
        claims_mapping:
          - { claim: groups, value: <group-object-id>, role: admin }
    jit_create:
      enabled: true
      default_role: reader
      email_domain_allowlist: [example.com]
```

### Entra notes

- **Use a tenant-specific issuer:**
  `https://login.microsoftonline.com/<tenant-id>/v2.0` — **not** `common` or
  `organizations`. SpecGraph routes providers by exact `iss` match. The
  multi-tenant endpoints return a **templated** issuer
  (`.../{tenantid}/v2.0`) that will not match the concrete `iss` value in an
  issued token, so sign-in fails. Substitute your real tenant GUID.
- **Emit the `groups` claim.** For `groups` → role mapping to work, the app
  registration must be configured to include the `groups` claim in the ID
  token: **App registration → Token configuration → Add groups claim**. Be
  aware of **"groups overage"** — users who belong to many groups get a claim
  pointing at the Graph API instead of the inline group list, which breaks
  direct claim matching. Prefer mapping on a small number of curated security
  groups, or assign app roles instead.
- **App registration setup:**
    1. Add the redirect URI
       `https://specgraph.example.com/api/auth/oidc/callback` under the **Web**
       platform.
    2. Create a **client secret** and place its value in the
       `SPECGRAPH_OIDC_ENTRA_SECRET` environment variable.
    3. Copy the **Application (client) ID** into `client_id` and the **Directory
       (tenant) ID** into the issuer URL.

## Okta

Register an OIDC **Web** application in Okta, then configure the provider:

```yaml
auth:
  oidc:
    base_url: https://specgraph.example.com
    session_ttl: 12h
    providers:
      - id: okta
        interactive: true
        display_name: Okta
        issuer: https://<org>.okta.com/oauth2/default
        client_id: <okta-client-id>
        client_secret_env: SPECGRAPH_OIDC_OKTA_SECRET
        scopes: [openid, profile, email]
        claims_mapping:
          - { claim: groups, value: specgraph-admins, role: admin }
    jit_create:
      enabled: true
      default_role: reader
      email_domain_allowlist: [example.com]
```

### Okta notes

- **Application type:** create an **OIDC → Web Application** in the Okta admin
  console (it must hold a client secret for the Authorization Code flow).
- **Issuer:** use your authorization server's issuer. The built-in default
  server is `https://<org>.okta.com/oauth2/default`. If you use a **custom** or
  **org** authorization server, use its issuer URL instead — and confirm it
  matches the `iss` in issued tokens exactly.
- **Redirect URI:** add `https://specgraph.example.com/api/auth/oidc/callback`
  to the application's **Sign-in redirect URIs** (exact match, no wildcards).
- **Groups claim:** to use `groups` → role mapping, add a **groups claim** to
  the authorization server (**Security → API → Authorization Servers → Claims**)
  that emits group names (e.g. a filter matching `specgraph-*`) in the ID
  token, and ensure the `groups` scope/claim is returned.

## GitHub (via an OIDC broker)

!!! danger "GitHub is not an OIDC provider for user sign-in"
    GitHub **cannot** be used directly with SpecGraph's OIDC login. GitHub
    OAuth issues an **opaque access token and no `id_token`**, and there is no
    user-facing `.well-known/openid-configuration` discovery document. SpecGraph
    requires a verifiable ID token, so GitHub-direct does not work.

### Supported path: federate through a broker

Put an OIDC broker that **does** issue ID tokens in front of GitHub, configure
GitHub as an **upstream identity provider** in that broker, and point SpecGraph
at the broker. To SpecGraph, the broker looks like any other OIDC provider.

Brokers that work this way include:

- **Microsoft Entra External ID**
- **Auth0**
- **Keycloak**
- **Dex**

The flow is: browser → SpecGraph → broker → GitHub (and back). SpecGraph only
ever talks to the broker, which mints a standard ID token:

```yaml
auth:
  oidc:
    base_url: https://specgraph.example.com
    session_ttl: 12h
    providers:
      - id: github-via-broker
        interactive: true
        display_name: Sign in with GitHub
        issuer: https://broker.example.com/realms/specgraph
        client_id: <broker-client-id>
        client_secret_env: SPECGRAPH_OIDC_BROKER_SECRET
        scopes: [openid, profile, email]
```

Configure GitHub as the upstream IdP **inside the broker** (the broker holds the
GitHub OAuth app's client id/secret and the GitHub callback). SpecGraph's
redirect URI is still `https://specgraph.example.com/api/auth/oidc/callback`,
registered at the **broker**, not at GitHub.

!!! note "Deferred follow-up"
    Native GitHub-direct support — a generic OAuth2 + `userinfo` provider that
    does not require an ID token — is a deferred follow-up and is not yet
    available.

## Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `redirect_uri` mismatch at the IdP | The registered URI does not match what SpecGraph sent | Register the exact URI `https://<host>/api/auth/oidc/callback`; set `auth.oidc.base_url` so the sent URI is deterministic. |
| `iss` mismatch / "no provider for issuer" | Using the Entra `common`/`organizations` endpoint, or a mismatched Okta authorization server | Use a **tenant-specific** Entra issuer (`.../<tenant-id>/v2.0`); for Okta, match the exact authorization-server issuer. |
| Login loop or `auth_error=unauthorized` after a successful IdP sign-in | JIT provisioning disabled, or the user's email domain is not allowlisted | Enable `auth.oidc.jit_create`, or add the user's domain to `email_domain_allowlist`. |
| Wrong redirect URI / rate limiting keyed on the proxy IP | Running behind a reverse proxy without proxy settings | Set `auth.oidc.base_url` and `server.trusted_proxy: true`. |

For deeper diagnosis, check the server logs around the callback request — the
validated `iss`, the resolved redirect URI, and the JIT decision are recorded
there.
