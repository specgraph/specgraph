// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import "fmt"

func syncAgents(_ string, _ Options) SyncResult {
	return errResult("AGENTS.md", fmt.Errorf("syncAgents not implemented"))
}
