// SpecGraph base authorization policies.
//
// Migrated from the static rpcPermissions table. Roles gate VERBS via action
// groups: every concrete action (e.g. SpecGraph::Action::"spec.read") is a
// member of its verb group (SpecGraph::Action::"read") via the entity graph
// the engine builds at construction. New RPCs join a group by being added to
// internal/auth/actions.go — these policies do not change.
//
// Cedar semantics: default-deny; any matching permit allows; an explicit
// forbid (none here) would beat every permit. Discrete per-action and
// ownership policies for later stories layer on top via additional sources.

permit (
	principal,
	action in SpecGraph::Action::"read",
	resource
) when {
	principal has role &&
	(principal.role == "reader" || principal.role == "writer" || principal.role == "admin")
};

permit (
	principal,
	action in SpecGraph::Action::"write",
	resource
) when {
	principal has role &&
	(principal.role == "writer" || principal.role == "admin")
};

permit (
	principal,
	action in SpecGraph::Action::"delete",
	resource
) when {
	principal has role && principal.role == "admin"
};

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

// Self-service operations (apikey.self): any AUTHENTICATED principal may act on
// their OWN API keys. Cedar only sees principal.role/id/email — it cannot see
// Source or the minted role floor — so this permit is deliberately broad and
// does NOT gate on principal.role == "admin". The handler further restricts it:
// it rejects Source == "apikey" (no key-mints-key laundering) and floors the
// minted role via auth.RoleMin. Landed in the same commit as the "self" verb in
// knownVerbs (engine.go) and the apikey.self action map (actions.go) so the
// engine boots consistently.
permit (
	principal,
	action in SpecGraph::Action::"self",
	resource
) when {
	principal has role
};
