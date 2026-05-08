// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// Unlocker releases a file lock acquired via acquireFileLock. Returns
// any errors from the underlying flock LOCK_UN (Unix) or LockFileEx
// release (Windows) plus any error closing the lock-file handle.
//
// Callers MUST invoke the Unlocker via a deferred wrapper that propagates
// the error — leaving a lock unreleased breaks subsequent acquires.
type Unlocker func() error
