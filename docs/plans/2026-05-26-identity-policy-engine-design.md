# Identity Policy Engine Adoption (Cedar)

**Date:** 2026-05-26
**Status:** Approved (2026-05-26)
**Part of:** Identity, RBAC & Audit epic (`spgr-rjrt`). Companion designs: Identity Storage, Identity Authn, Bootstrap & UX. Supersedes the retired Self-Service Authz design (`spgr-qe74`).

## Problem

Authorization in SpecGraph is a static map from RPC procedure → required permission string, evaluated with a small wildcard-matching helper. This shape:

- Cannot express "principal can do X to their own resources, otherwise needs permission Y." Self-service requires either a special skip-and-defer mechanism (bug-prone) or relaxing permissions globally (privilege escalation).
- Cannot express project-scoped roles ("Alice is admin of project A, reader of project B") without new tables and per-RPC code paths.
- Cannot express resource ownership ("only the spec creator can supersede") without per-handler logic.
- Buries policy in code, making review of "what can who do?" a code-archaeology exercise.

Stories #3 (project-scoped RBAC) and #4 (resource ownership rules) compound the problem if built on this foundation. Both will require their own static-table extensions and per-RPC handlers.

A policy engine — Cedar specifically — collapses all of this into one mechanism: policies expressed in a declarative language, evaluated against `(principal, action, resource, context)` tuples at request time. The conditional logic that would have been special-cased in handlers becomes a policy expression. New stories land as new policies, not new code paths.

## Scope

Adoption of Cedar as SpecGraph's authorization engine. Covers the engine choice, where policies live, how SpecGraph entities project into Cedar's principal/resource/action model, the integration seam between the resolved Identity and the engine, and the migration path from the existing static table.

Out of scope: specific policy contents for stories #3 and #4 (those stories author their own policies against this engine); Cedar's internal optimizations; non-authz use of Cedar (we're not adopting it for config validation or admission).

## Engine choice

Cedar (`cedar-policy/cedar-go`). Decided based on:

- Purpose-built for authz (vs OPA's general-purpose Rego).
- Embeddable as a Go library — no separate process, no sidecar.
- Static type checking on policies at load time.
- Decision logging is a first-class concept, useful for audit.
- The policy DSL is readable for the authz domain specifically.

The Go port is younger than the Rust original, but functional and actively maintained. If a feature gap emerges during implementation that blocks SpecGraph's needs, the fallback is OPA embedded; this is a recoverable choice.

## Architectural shape

The engine is embedded. One process, one binary. The flow at request time:

1. Interceptor authenticates the request (per Authn design); resolves an `Identity`.
2. Interceptor maps `(service, method)` to a Cedar action name (see "Entity model" below).
3. Interceptor builds a Cedar `(principal, action, resource, context)` tuple from the request and identity.
4. Engine evaluates against the loaded policy set. Returns `Allow` or `Deny`.
5. Interceptor either invokes the handler (Allow) or returns a `PermissionDenied` (Deny).
6. The engine's decision (with the matched policies) is emitted as a structured event for the audit sink.

The handler itself runs without further policy checks. No skip-and-defer mechanism. No handler-level principal-vs-target comparisons. The engine decides; the handler executes.

**Cedar is wrapped behind a SpecGraph-owned interface.** The rest of the codebase imports `auth.PolicyEngine`, not Cedar types directly. The wrapper is thin (load policies, evaluate, return a decision struct) but the indirection is load-bearing for two reasons: it keeps Cedar-specific types out of handler code, and it provides a swap point if cedar-go falls behind or a different engine becomes preferable later. The wrapper is the only file that imports `cedar-policy/cedar-go`.

```text
type PolicyEngine interface {
    Evaluate(ctx context.Context, req EvalRequest) (Decision, error)
    Reload(ctx context.Context) error           // re-load from all sources
}
```

The `EvalRequest` and `Decision` types are SpecGraph-defined (not Cedar types), populated from the resolved Identity and the request shape. The mapping from these to Cedar's native types happens inside the wrapper.

## Policy lifecycle

Policy storage is **abstracted behind an interface** so the source can evolve independently of the engine. Today's source is filesystem; tomorrow's may be a database table, a remote URL, an object store, or any composition of those. The design pins the seam, not the implementation.

```text
type PolicySource interface {
    // Load returns the policy text and a stable identifier for this source.
    // Sources may return empty content (no policies from this source) without error.
    Load(ctx context.Context) ([]Policy, error)
    Name() string                              // for diagnostics / decision logs
}
```

**Sources are composable.** The server loads from a configured ordered list of sources at start. Built-in policies (shipped with the binary, typically via `embed.FS`) are one source. Operator-supplied filesystem directories are another. A future database-backed source is another. The composition rule is straightforward: every source contributes its policy set; Cedar's standard evaluation semantics handle the merged set (explicit `Deny` always wins over `Permit`). Conflicts that violate operator intent surface as policy authoring problems, not as engine-level configuration drift.

Source loading semantics:

- The built-in source MUST load successfully. Failure here is a programming error (the binary was built wrong) and the server refuses to start.
- Operator-supplied sources MAY be configured as required or best-effort. A best-effort remote source that's temporarily unreachable warns at start and continues; a required source that fails refuses start. The default for filesystem sources is required; the default for network sources is best-effort.

**Hot-reload is out of scope for this story.** Editing policies requires a server restart, the same constraint that applied to YAML role definitions in the prior design. The resolve hot path stays lock-free. Future stories can add a reload signal that re-queries sources without invalidating in-flight requests.

The initial implementation ships two source implementations:

- `EmbeddedPolicySource` — built-in policies compiled into the binary.
- `DirectoryPolicySource` — filesystem directory (configured via `auth.policies.extra_dirs`).

The interface is the load-bearing piece. DB-backed and URL-backed sources are deliberate follow-ups, requiring no changes to the engine or the resolver — only a new `PolicySource` implementation.

## Entity model in Cedar terms

Cedar requires a schema declaring the entity types it reasons about. SpecGraph's projection:

- **Principal types:** `User::Human`, `User::ServiceAccount`. Attributes include `role` (the assigned role string), `id`, `email`, and `bootstrap` (the system-identity flag). Both types share a parent `User` for policies that don't care about kind.
- **Resource types:** vary by what's being acted on. For this story's scope, the relevant ones are `User` (the user being managed), `APIKey` (with attribute `owner_user_id`), and `OIDCBinding` (with attribute `user_id`). Stories #3 and #4 add `Project`, `Spec`, `Decision`, etc.
- **Actions:** stable, namespaced identifiers decoupled from RPC method names. E.g., `Action::"identity.rotate_api_key"` rather than `Action::"UserService.RotateAPIKey"`. A small mapping table inside the resolver translates `(service, method)` → action name at request time. This survives proto refactors (renaming an RPC method does not require rewriting every policy) and lets a single action be reused across protocols (RPC + REST + future GraphQL would all map to the same action). The static `rpcPermissions` table is replaced by this `(service, method) → action` map plus the action declarations in the Cedar schema.
- **Context:** the request's transient attributes that aren't owned by the principal or resource — e.g., the project slug from the `X-Specgraph-Project` header, the time of day if any policy ever cares, the source IP if any future policy cares.

Entity attributes come from the resolved User row plus whatever the request handler can cheaply look up. For Pattern A operations (target in request), the resource attributes come from the request directly. For Pattern B operations (target via lookup), the handler does a single read to populate resource attributes before invoking the engine — same lookup pattern the Self-Service Authz design described, but the result is fed to Cedar instead of evaluated by hand.

## Integration with the resolver

The Authn design's resolver produces an `Identity`. That Identity is the source of truth for the Cedar `principal`. The "Permission computation" section in the Authn design — which described deriving permissions from a server-start snapshot of role→perms — is **superseded** by this design. Cedar replaces that snapshot entirely. The Identity no longer needs a `Permissions` map; it carries `Role`, `UserID`, and whatever attributes Cedar policies reference.

Concretely:

- `Identity.Role` continues to exist (it's an attribute Cedar policies reference: `principal.role == "admin"`).
- `Identity.Permissions map[string]bool` is removed. Permission decisions come from Cedar, not from the Identity.
- `HasPermission(perms, required)` and its wildcard logic in `internal/auth/auth.go` are deleted.

## Migration from the static rpcPermissions table

The existing table maps procedure → permission string. Each entry translates to a Cedar action declaration plus an initial policy. Examples (illustrative, not exact syntax):

```text
// Today:
SpecService.GetSpec → "spec:read"

// Cedar (conceptual):
action Action::"SpecService.GetSpec" applies to principal User, resource Spec;
permit (principal, action == Action::"SpecService.GetSpec", resource)
  when { principal.role in ["admin", "writer", "reader"] };
```

The translation is mechanical for the existing simple rules. The payoff arrives when the rule needs to express conditions the static table couldn't:

```text
// Self-service rotate-own-API-key (replaces the Self-Service Authz design's mechanism):
permit (principal, action == Action::"UserService.RotateAPIKey", resource)
  when { resource.owner_user_id == principal.id };
permit (principal, action == Action::"UserService.RotateAPIKey", resource)
  when { principal.role == "admin" };
```

No skip-and-defer. No runtime registry. The policy expresses the rule directly.

The migration runs as a single commit that lands with this design's implementation: the static table goes away, the policy files arrive, the resolver wires Cedar in. No deprecation cycle on the static table (it's an internal mechanism with no external API).

## Decision logging and audit

Every Cedar evaluation produces a structured decision (`Allow` / `Deny`, matched policies, principal, action, resource attributes). This is the natural source for audit-log story #1 — the audit sink subscribes to decisions and persists them. Until #1 lands, decisions are emitted at `slog.Debug` for trace-level observability.

This collapses three things that were separate in earlier designs: (a) the audit emission interface from the Self-Service Authz design, (b) the per-handler "did the actor have permission to do this?" question, (c) the per-RPC permission requirement. All become "what did Cedar decide, and why?"

## How stories #3 and #4 land

**Story #3 — project-scoped RBAC.** A new `Project` resource type and a `project_user_roles` mapping table (still required as data). Policies reference `principal.projects[context.project_slug].role` or similar. Example:

```text
permit (principal, action == Action::"SpecService.UpdateSpec", resource)
  when { principal.projects has context.project_slug
       && principal.projects[context.project_slug].role in ["admin", "writer"] };
```

No new authorization code paths in handlers. The story becomes "define the data model for project role bindings; write the policies; teach the resolver to populate `principal.projects` from the new table."

**Story #4 — resource ownership.** Policies reference `resource.owner_user_id == principal.id` (or `resource.creator_user_id`, depending on the rule). Same pattern: data model + policies, no per-handler code.

Both stories shrink from "build new mechanism" to "extend the schema and write the right policies."

## Non-goals

- Hot-reload of policies (server restart required to apply policy changes).
- Cedar features SpecGraph doesn't use immediately (templates, policy stores, partial evaluation). They're available; we don't take a dependency on them in v1.
- Using Cedar for non-authz decisions (admission control, config validation, etc.). Single-purpose adoption.
- A policy editor / UI for end users. Policies are authored as files, reviewed via git.

## Sequencing

This design replaces the retired Self-Service Authz design and supersedes the "Permission computation" section of the approved Authn design. Storage and Bootstrap & UX are unaffected at the design level (the entity model they specify projects cleanly into Cedar's principal/resource model).

Implementation order, once approved:

1. Add cedar-go dependency; introduce the engine wrapper and entity schema loader.
2. Translate the existing rpcPermissions table to policies; delete the static table and `HasPermission` helper.
3. Wire the engine into the resolver (replace the snapshot-based permission computation).
4. Add decision-logging hook; emit at Debug until audit story #1 lands a real sink.
5. Adapt the few RPCs that need owner-based or self-service rules; write their policies (using existing resource attributes).

Stories #3 and #4 build on this foundation as separate epics.
