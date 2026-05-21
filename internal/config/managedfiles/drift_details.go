// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// Detail strings emitted in SyncResult.Detail and FileState.Detail.
// Strategy implementations populate these in SyncResult.Detail (surfaced
// by `specgraph init`'s per-file output) and FileState.Detail (surfaced
// by `specgraph doctor`'s expanded output). Kept stable and exported so
// downstream consumers can match prefixes if needed.
const (
	// DriftDetailNoSentinel is emitted when WholeFile + CommentNone
	// finds an on-disk file whose hash matches neither the canonical
	// nor any registered prior — i.e. user-owned content at a path the
	// framework manages.
	DriftDetailNoSentinel = "no sentinel"

	// DriftDetailFrontmatterParseErrorPrefix prefixes detail strings emitted
	// when WholeFile + HasFrontmatter finds malformed frontmatter surrounding
	// the sentinel line. The full prefix is "frontmatter parse error: ".
	DriftDetailFrontmatterParseErrorPrefix = "frontmatter parse error: "

	// DriftDetailSupersedesPath prefixes detail strings emitted when a
	// superseded path is left in place because its content has drifted
	// from the prior canonical.
	DriftDetailSupersedesPath = "supersedes path "
)
