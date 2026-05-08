// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "errors"

// ErrCorruptedSentinel indicates a managed file's sentinel line is
// malformed (unparseable, unknown version, missing required fields).
// The framework refuses to mutate corrupted-sentinel files.
var ErrCorruptedSentinel = errors.New("corrupted managed-file sentinel")

// ErrSymlinkRejected is returned when init/inspect encounters a symlink
// in a managed-file path component. We refuse to follow symlinks to avoid
// confused-deputy attacks (planting a symlink to an unrelated file then
// triggering init).
var ErrSymlinkRejected = errors.New("symlink in managed-file path rejected")

// ErrPriorCanonicalMismatch is returned by supersedesGuardedDelete when
// the on-disk content of a SupersedesPath does NOT match the prior
// canonical the framework would have produced. Indicates user content
// at the old path; init refuses to delete it.
var ErrPriorCanonicalMismatch = errors.New("supersedes-path content differs from prior canonical")

// errNotImplemented is returned by strategy stubs in PR A. PRs B and onward
// replace the stubs with real implementations; the manifest is empty in PR A
// so this error is never surfaced end-to-end.
//
// Unexported because it's a transient, PR-A-only API surface — exposing it
// would invite external callers to depend on a stub state that disappears
// in PR B.
var errNotImplemented = errors.New("strategy not implemented in this PR")
