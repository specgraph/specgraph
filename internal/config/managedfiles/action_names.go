// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// ActionName returns the canonical lowercase string for an Action,
// suitable for human-readable CLI output. PR G's doctor uses the
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
