// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// identityClient builds an authenticated IdentityService client using the
// shared credential/base-URL resolution (newClient).
//
// It is a package-level var (not a plain func) so tests can substitute a client
// pointed at an in-process stub server; production callers invoke it unchanged.
var identityClient = func() (specgraphv1connect.IdentityServiceClient, error) {
	return newClient(specgraphv1connect.NewIdentityServiceClient)
}
