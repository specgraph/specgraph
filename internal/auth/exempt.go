// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// exemptProcedures lists procedures that bypass authentication AND
// authorization entirely (health checks). Consulted by the interceptor
// before any Resolver/Authorizer work. Relocated from permissions.go (which
// the Cedar plan deletes); IsExempt is the interceptor's only dependency on
// this file.
var exemptProcedures = map[string]bool{
	specgraphv1connect.ServerServiceHealthProcedure: true,
}

// IsExempt reports whether a procedure bypasses auth.
func IsExempt(procedure string) bool {
	return exemptProcedures[procedure]
}
