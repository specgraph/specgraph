// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// ActionName returns the canonical lowercase string for an Action,
// suitable for human-readable CLI output. The doctor command uses the
// same names to keep init and doctor output aligned.
func ActionName(a Action) string {
	switch a {
	case ActionNoOp:
		return "no-op"
	case ActionCreated:
		return "created"
	case ActionRefreshed:
		return "refreshed"
	case ActionSkipped:
		return "skipped"
	case ActionForced:
		return "forced"
	case ActionError:
		return "error"
	default:
		return "unknown"
	}
}

// StateName returns the canonical lowercase string for a State,
// suitable for human-readable CLI output. Mirrors ActionName for symmetry.
func StateName(s State) string {
	switch s {
	case StateSynced:
		return "synced"
	case StateMissing:
		return "missing"
	case StateStale:
		return "stale"
	case StateDrifted:
		return "drifted"
	default:
		return "unknown"
	}
}

// CountErrors returns the number of SyncResults with Action == ActionError.
func CountErrors(rs []SyncResult) int {
	n := 0
	for _, r := range rs {
		if r.Action == ActionError {
			n++
		}
	}
	return n
}
