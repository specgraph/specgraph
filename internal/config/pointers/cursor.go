// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import "fmt"

func syncCursor(_ string, _ Options) SyncResult {
	return errResult(".cursor/rules/specgraph-bootstrap.md", fmt.Errorf("syncCursor not implemented"))
}
