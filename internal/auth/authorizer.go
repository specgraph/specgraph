// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "context"

// Authorizer decides whether a resolved Identity may invoke procedure
// with the given request body. The active CedarAuthorizer impl
// (cedar_authorizer.go) delegates to a PolicyEngine that evaluates the
// request against the loaded Cedar policy set.
//
// req carries the unmarshaled request body so future authorizers
// (ownership rules) can inspect resource attributes. Today's
// CedarAuthorizer ignores req.
type Authorizer interface {
	Authorize(ctx context.Context, id *Identity, procedure string, req any) (Decision, error)
}

// Decision is the outcome of an Authorize call.
//
// Allowed=true means the handler should run. Allowed=false means the
// interceptor returns connect.CodePermissionDenied.
//
// Reason carries a short structured tag for audit emission and logging.
// CedarAuthorizer emits "cedar-allow:<comma-joined matched policy IDs>"
// (e.g., "cedar-allow:embedded:base.cedar#policy0") and "cedar-deny:<action>"
// (e.g., "cedar-deny:spec.write"). The interceptor does not parse it.
type Decision struct {
	Allowed bool
	Reason  string
}
