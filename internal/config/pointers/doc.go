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
//
// Symlink rejection is best-effort, not a security boundary. The
// rejectSymlinkComponents walk and the subsequent open are not atomic;
// a process that can write to the project directory between the walk
// and the open could swap a path component for a symlink. On Unix we
// reduce the window by opening read targets with O_NOFOLLOW; on
// Windows we rely on the walk only. Treat the project directory as a
// trust boundary and run specgraph init from a directory you own.
package pointers
