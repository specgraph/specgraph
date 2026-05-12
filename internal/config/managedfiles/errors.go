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

