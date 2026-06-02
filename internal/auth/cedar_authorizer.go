// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"fmt"
	"strings"
)

// CedarAuthorizer implements Authorizer by delegating to a PolicyEngine.
// It replaces StaticTableAuthorizer; because both satisfy Authorizer, the
// interceptor and serve.go wiring change only in which constructor is called
// — the interceptor's Authorize call site is byte-identical.
type CedarAuthorizer struct {
	engine PolicyEngine
}

// NewCedarAuthorizer wraps a PolicyEngine as an Authorizer.
func NewCedarAuthorizer(engine PolicyEngine) *CedarAuthorizer {
	return &CedarAuthorizer{engine: engine}
}

// Authorize maps the RPC procedure to a stable action name, builds an
// EvalRequest, and asks the engine. An unconfigured procedure is an error
// (mirrors StaticTableAuthorizer; the interceptor maps it to CodeInternal),
// which is deliberately distinct from a clean Deny.
//
// req (the unmarshaled request body) is accepted for the Authorizer
// interface and future ownership rules; the migration's role-only policies
// do not inspect it.
func (a *CedarAuthorizer) Authorize(ctx context.Context, id *Identity, procedure string, _ any) (Decision, error) {
	action, ok := ActionForProcedure(procedure)
	if !ok {
		return Decision{}, fmt.Errorf("cedar: unconfigured procedure %q", procedure)
	}
	domain := actionDomain(action)
	dec, err := a.engine.Evaluate(ctx, EvalRequest{
		Identity: id,
		Action:   action,
		Resource: ResourceRef{Type: domain, ID: domain},
	})
	if err != nil {
		return Decision{}, fmt.Errorf("cedar: evaluate %s: %w", action, err)
	}
	if dec.Allowed {
		return Decision{
			Allowed: true,
			Reason:  "cedar-allow:" + strings.Join(dec.MatchedPolicies, ","),
		}, nil
	}
	return Decision{
		Allowed: false,
		Reason:  "cedar-deny:" + action,
	}, nil
}
