# Custom-role downgrade: fail-closed clamp (spgr-rjrt.9)

## Problem

An API key carries an optional `RoleDowngrade` — a cap that should make the key
*less* privileged than its owning user. The resolver computes the key's
`EffectiveRole` as `clampedRole(user.Role, key.RoleDowngrade)` and hands it to
the Cedar policy engine.

`clampedRole` orders only the built-in roles (`reader < writer < admin`) via the
`roleRank` map; `roleLessThan` returns `false` for any role not in that map.
The current implementation returns `userRole` whenever the downgrade is not
provably lower:

```go
func clampedRole(userRole, downgrade Role) Role {
    if downgrade == "" {
        return userRole
    }
    if roleLessThan(downgrade, userRole) {
        return downgrade
    }
    return userRole
}
```

So when **either** operand is a custom (unranked) role, the comparison is
`false` and the function returns the user's *fuller* role. A key whose
`RoleDowngrade` is a custom role — or any key on a user whose role is custom —
silently keeps the broader role instead of being restricted. This is a
fail-**open** privilege relaxation: a cap intended to restrict is a no-op.

Cedar (the policy engine) is default-deny on the role attribute and faithfully
enforces whatever `EffectiveRole` it is given; the downgrade is computed
upstream of Cedar, so this is not fixed by the policy layer. It must be fixed
where `EffectiveRole` is derived.

## Decisions

- **`RoleDowngrade` targets built-in roles only.** A downgrade is a cap and is
  only meaningful where a privilege order exists. Scoping a key to a custom role
  is a separate concern (see Non-goals).
- **The resolver fails closed.** When a downgrade is set but the two roles
  cannot be ordered (a custom role on either side), the effective role becomes
  the most-restrictive built-in, `reader`. Never the user's fuller role.

## Design

Two layers: enforce on read (the security guarantee) and validate on write
(prevention plus a clear error).

### 1. Resolver — fail-closed `clampedRole`

```text
clampedRole(userRole, downgrade):
  downgrade == ""                        -> userRole                 # no cap; key == owner
  userRole and downgrade both ranked     -> min(userRole, downgrade) # correct clamp; never elevates
  otherwise (a custom role either side)  -> RoleReader               # cap set but unorderable -> floor
```

"Ranked" means present in `roleRank`. The `min` branch preserves today's
correct behavior, including the anti-escalation case (a downgrade higher than
the user's role yields the user's role). The floor branch is the fix: an
unorderable-but-present downgrade resolves to `reader` rather than to the user's
role.

This holds at the enforcement point for every key — including legacy keys that
were created with a custom downgrade before write-side validation existed, and
keys on users whose role is custom.

The empty-downgrade path is unchanged: a key with no cap is exactly its owner's
role, custom or not.

### 2. Handler — validate `RoleDowngrade` at key creation

In the `CreateAPIKey` handler: if `role_downgrade` is non-empty and not one of
`reader`, `writer`, `admin`, return `InvalidArgument` with a message naming the
allowed values. This fails fast with a clear error instead of silently flooring
the key, and prevents new custom-downgrade keys from being created.

`RotateAPIKey` does not accept `role_downgrade` (the field was reserved when
rotation was made identity-preserving); rotation inherits the old key's
downgrade, so no validation is added there. Any legacy custom downgrade carried
forward by rotation is still contained by the resolver floor.

## Non-goals (YAGNI)

- Custom-role downgrade targets.
- A configurable privilege rank for custom roles.
- Changing an existing key's `role_downgrade` (tracked separately).
- Expressing role ordering inside Cedar.

## Testing

- `clampedRole` table (direct): empty -> user; `writer`+`reader` -> reader;
  `reader`+`writer` -> reader (no escalation); `admin`+`reader` -> reader;
  custom-user + `reader` -> reader (floor); built-in-user + custom downgrade ->
  reader (floor); custom-user + empty -> custom (unchanged).
- Resolver (end-to-end): a key with a custom downgrade floors to `reader`.
- `CreateAPIKey` handler: custom `role_downgrade` -> `InvalidArgument`;
  built-in -> ok; empty -> ok.
