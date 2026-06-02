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
