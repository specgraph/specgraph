// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "testing"

func TestActionName(t *testing.T) {
	cases := map[Action]string{
		ActionNoOp:      "no-op",
		ActionCreated:   "created",
		ActionRefreshed: "refreshed",
		ActionSkipped:   "skipped",
		ActionForced:    "forced",
		ActionError:     "error",
	}
	for a, want := range cases {
		if got := ActionName(a); got != want {
			t.Errorf("ActionName(%v) = %q, want %q", a, got, want)
		}
	}
	// Default branch: any Action value outside the enum returns "unknown".
	if got := ActionName(Action(999)); got != "unknown" {
		t.Errorf("ActionName(Action(999)) = %q, want \"unknown\"", got)
	}
}

func TestCountErrors(t *testing.T) {
	rs := []SyncResult{
		{Action: ActionCreated},
		{Action: ActionError},
		{Action: ActionError},
		{Action: ActionNoOp},
	}
	if n := CountErrors(rs); n != 2 {
		t.Errorf("CountErrors = %d, want 2", n)
	}
}
