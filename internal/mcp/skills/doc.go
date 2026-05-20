// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package skills serves the SpecGraph SKILL.md packages embedded in the
// CLI binary. The package exposes a small read-only Source interface
// (List, Get, Search) with one implementation, embeddedSource, backed by
// //go:embed embedded/*/SKILL.md.
//
// SKILL.md schema follows agentskills.io with one SpecGraph-local
// extension: a required 'summary' field (≤120 chars after YAML decode)
// that the catalog tools surface separately from the longer
// 'description' paragraph. The skillvalidate package owns the
// schema invariants — see skillvalidate.NameRegex for the canonical
// skill-name pattern that this package imports rather than redefining.
//
// Source is read-only by design. Future implementations (a dirSource
// reading .specgraph/skills/, a remoteSource fetching from a registry,
// a compositeSource fanning out across both) plug in without touching
// the MCP handlers in internal/mcp. See
// docs/plans/2026-05-20-spgr-rwrp-pr-f-skills-mcp-design.md for the
// full design.
package skills
