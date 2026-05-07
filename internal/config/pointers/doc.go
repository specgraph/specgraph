// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package pointers renders and synchronizes managed-block-fenced markdown
// pointer files (AGENTS.md and .cursor/rules/specgraph-bootstrap.md) that
// direct an agent harness at a running SpecGraph MCP server.
//
// The mutation primitive is managed-block fencing: a single project-level
// block delimited by <!-- specgraph:init:start v=1 --> /
// <!-- specgraph:init:end --> markers. Content outside the block is owned
// by the user; content inside is reset to the canonical render every Sync.
//
// Sync also actively purges legacy per-slug blocks that the deprecated
// specgraph inject command used to write into AGENTS.md (markers of the
// shape <!-- specgraph:<slug>:start --> / <!-- specgraph:<slug>:end -->).
package pointers
