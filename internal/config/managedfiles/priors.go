// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "sync"

// priorsRegistry maps managed-file path → list of canonical SHA256 hashes
// (hex-encoded) that should classify the on-disk file as Stale-managed
// rather than Drifted-userowned. Each entry corresponds to a canonical
// version that has shipped at some point in SpecGraph's history.
//
// Population: package-level init registers all known priors. Both PR D's
// cursor-rule priors and PR E's JSON priors route through this registry.
type priorsRegistry struct {
	mu sync.RWMutex
	m  map[string][]string
}

var globalPriors = &priorsRegistry{m: map[string][]string{}}

// priorsFor returns the priors hashes registered for `path`. Empty slice
// if the path has no known priors. Always safe to call from any
// goroutine after package init.
func priorsFor(path string) []string {
	globalPriors.mu.RLock()
	defer globalPriors.mu.RUnlock()
	out := make([]string, len(globalPriors.m[path]))
	copy(out, globalPriors.m[path])
	return out
}

// registerPrior adds a prior hash for a managed-file path. Called from
// package init (priors_init.go) and the vestigial-cursor-rules
// translation. Idempotent: duplicate hashes are silently deduped.
func registerPrior(path, hashHex string) {
	globalPriors.mu.Lock()
	defer globalPriors.mu.Unlock()
	for _, h := range globalPriors.m[path] {
		if h == hashHex {
			return
		}
	}
	globalPriors.m[path] = append(globalPriors.m[path], hashHex)
}
