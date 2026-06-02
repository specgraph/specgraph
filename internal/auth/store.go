// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "errors"

// ErrUnauthenticated indicates a credential failure: missing, malformed,
// expired, revoked, soft-deleted user, JIT-rate-limited, allowlist
// mismatch, or any other "this principal isn't allowed to authenticate"
// condition. The interceptor maps this to connect.CodeUnauthenticated.
var ErrUnauthenticated = errors.New("auth: unauthenticated")

// ErrTransient indicates a backend failure unrelated to the credential:
// database unavailable, pool exhausted, network timeout. The interceptor
// maps this to connect.CodeUnavailable so callers know to retry.
//
// Errors of this kind wrap the underlying cause; tests use errors.Is to
// detect ErrTransient.
var ErrTransient = errors.New("auth: transient backend error")
