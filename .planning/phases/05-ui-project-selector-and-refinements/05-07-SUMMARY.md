---
phase: 05-ui-project-selector-and-refinements
plan: 07
subsystem: ui
tags: [svelte, shadcn, bits-ui, dialog, alert-dialog, tailwind, auth, api-keys]

# Dependency graph
requires:
  - phase: 05-01
    provides: shadcn-svelte install (Tailwind v4 tokens, bits-ui, $lib/components/ui primitives)
provides:
  - LoginModal migrated to shadcn Dialog + Input + Button (token-based, no scoped CSS)
  - RevealKeyModal migrated to shadcn Dialog (reveal) + AlertDialog (destructive revoke)
  - Reusable destructive revoke AlertDialog contract for the 05-12 keys page
affects: [05-12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Auth-gate Dialog uses controlled (non-bindable) open so it cannot be dismissed to a blank page"
    - "Reveal Dialog controlled by plaintext presence; onOpenChange forwards ESC/X/outside to onClose"
    - "Destructive confirm via AlertDialog.Action variant=destructive (UI-SPEC Copywriting Contract)"

key-files:
  created: []
  modified:
    - web/src/lib/components/LoginModal.svelte
    - web/src/lib/components/RevealKeyModal.svelte

key-decisions:
  - "LoginModal auth gate uses controlled open={true} (not bindable) — bits-ui focus-trap retained, no ESC-to-blank-page"
  - "RevealKeyModal props widened backward-compatibly: plaintext/onClose kept optional; added revokeOpen (bindable) + onRevoke for 05-12 wiring"
  - "Revoke AlertDialog lives in RevealKeyModal but stays closed by default — keys page (05-12) drives it per-row"

patterns-established:
  - "Pattern 1: shadcn Dialog as an always-open auth gate via controlled open prop"
  - "Pattern 2: one component houses both a Dialog (reveal) and an AlertDialog (destructive confirm) with independent controls"

requirements-completed: [D-12, D-13]

coverage:
  - id: D1
    description: "LoginModal renders on shadcn Dialog + Input + Button with Slate tokens (no scoped <style>, no hex); onSuccess/authError props and login/OIDC behavior preserved"
    requirement: "D-12"
    verification:
      - kind: other
        ref: "cd web && node -e '<style>/hex/Dialog guards' && pnpm build"
        status: pass
    human_judgment: true
    rationale: "Static guards + build prove structure, but a11y focus-trap/ESC and login-flow correctness in both themes need human UAT (login dialog visual + keyboard)"
  - id: D2
    description: "RevealKeyModal reveal path on shadcn Dialog; destructive revoke on AlertDialog with UI-SPEC copy (title 'Revoke this key?', Revoke variant=destructive, Cancel); one-time-plaintext reveal preserved"
    requirement: "D-13"
    verification:
      - kind: other
        ref: "cd web && node -e '<style>/hex/AlertDialog guards' && pnpm build"
        status: pass
    human_judgment: true
    rationale: "Build + guards confirm structure/copy presence; reveal→copy→close and revoke confirm/cancel flows with focus-trap require human UAT (05-12 wires the keys-page revoke trigger)"

# Metrics
duration: 18min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 07: Modal Migration (LoginModal + RevealKeyModal) Summary

**LoginModal and RevealKeyModal re-implemented on shadcn `Dialog`/`AlertDialog` + Slate tokens — auth gate, one-time key reveal, and destructive revoke contract, all scoped-CSS-free.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-07-12
- **Completed:** 2026-07-12
- **Tasks:** 2
- **Files modified:** 2 (+ deferred-items log)

## Accomplishments
- `LoginModal` migrated to shadcn `Dialog` + `Input` + `Button`; hand-rolled overlay and navy `#1a1a2e` button retired; OIDC provider links now use `Button variant="outline"` with `href`. Auth-gate stays non-dismissible via controlled `open`.
- `RevealKeyModal` reveal path migrated to shadcn `Dialog`; added the destructive revoke `AlertDialog` per the UI-SPEC Copywriting Contract (title **Revoke this key?**, body, **Revoke** `variant=destructive`, **Cancel**).
- RevealKeyModal props widened backward-compatibly (`plaintext`/`onClose` kept; `revokeOpen`/`onRevoke` added) so the 05-12 keys page can drive per-row revoke without a new component.
- One-time-plaintext reveal behavior preserved; key stays component-local (T-05-10) and is auto-escaped by Svelte `{plaintext}` (T-05-12).

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate LoginModal to shadcn Dialog + Input + Button** — `f8d1f5b7` (feat)
2. **Task 2: Migrate RevealKeyModal to Dialog reveal + AlertDialog revoke** — `4bc5f8d1` (feat)

**Deferred-item log:** `a0341387` (docs)
**Plan metadata:** _(this commit)_ (docs: complete plan)

## Files Created/Modified
- `web/src/lib/components/LoginModal.svelte` — shadcn Dialog + Input + Button; controlled-open auth gate; OIDC provider buttons; token utility classes only.
- `web/src/lib/components/RevealKeyModal.svelte` — Dialog reveal + AlertDialog revoke; backward-compatible props plus `revokeOpen`/`onRevoke`.
- `.planning/phases/05-ui-project-selector-and-refinements/deferred-items.md` — logged pre-existing `.planning/intel` fmt drift.

## Decisions Made
- **Auth-gate open is controlled, not bindable.** `LoginModal` passes `open={true}` so ESC/outside cannot dismiss the gate into a blank page; bits-ui focus-trap is retained and no custom key handling was added (per plan).
- **Revoke AlertDialog co-located in RevealKeyModal, closed by default.** The component owns the destructive contract; the keys page (05-12) triggers it per row via `revokeOpen`/`onRevoke`. Keeps 05-12 a wiring-only change.
- **Reveal Dialog controlled by `plaintext` presence** with `onOpenChange` forwarding ESC/X/outside dismissal to `onClose`, preserving the existing conditional-mount (`{#if revealed}`) behavior in the keys page.

## Deviations from Plan

None - plan executed exactly as written.

The plan's Task 2 language ("reveal + revoke behavior") anticipated a revoke path that did not yet exist in `RevealKeyModal` (revoke was previously a bare per-row button in the keys page with no confirm). Adding the `AlertDialog` revoke contract to the component is the intended plan outcome (UI-SPEC map + Copywriting Contract + 05-12 dependency), implemented via backward-compatible optional props — not a deviation.

**Total deviations:** 0.
**Impact on plan:** None — both files build and pass the automated structural guards.

## Issues Encountered
- **`task check` fails at `fmt:check`** on 139 pre-existing unformatted `.planning/intel/classifications/*.json` files. This is a documented, out-of-scope drift (same failure logged for 05-02/05-03/05-05). `dprint` has no Svelte plugin, so the two migrated `.svelte` files are not in fmt scope. `task web:build` and `pnpm build` both pass. Logged in `deferred-items.md` under `## 05-07`. Remediation: `dprint fmt` on `.planning/intel/`.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Wave-2 modal migration complete. `RevealKeyModal` now exposes the reveal Dialog and the destructive revoke `AlertDialog` that **05-12** (`/keys` restyle) depends on for per-row revoke wiring.
- Manual UAT still recommended (deferred to end-of-phase per `human_verify_mode`): login dialog + reveal/copy/close + revoke confirm/cancel with keyboard/focus-trap in both themes.

## Self-Check: PASSED
- `web/src/lib/components/LoginModal.svelte` — FOUND
- `web/src/lib/components/RevealKeyModal.svelte` — FOUND
- Commit `f8d1f5b7` — FOUND
- Commit `4bc5f8d1` — FOUND

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*
