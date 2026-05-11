// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package managedfiles is the framework specgraph init uses to inject
// canonical files into a user's project — and that specgraph doctor
// uses to detect drift.
//
// Every file specgraph writes is registered as a ManagedFile with a
// strategy that determines how the file is read, classified (Synced,
// Stale, Drifted, or Missing), and reconciled. The strategies are:
//
//   - StrategyJSONKeyMerge: managed keys merge into a JSON file;
//     siblings preserved (e.g. .mcp.json's mcpServers.specgraph block).
//   - StrategyMarkdownBlock: a versioned, hash-tracked block fenced by
//     <!-- specgraph:init:start ... --> / <!-- specgraph:init:end -->
//     within an otherwise-user-owned markdown file (e.g. AGENTS.md).
//   - StrategyWholeFile: the entire file is canonical; warn-and-force
//     on drift (e.g. a generated TypeScript plugin).
//
// PR A scaffolds the framework: types, sentinels, hashing, file locking,
// atomic writes, symlink rejection, supersedes-path deletion. Subsequent
// PRs in spgr-rwrp register managed files and implement strategies.
package managedfiles
