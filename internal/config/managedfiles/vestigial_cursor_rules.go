// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	_ "embed"
	"fmt"
)

// Vestigial pre-rename Cursor rule bytes. Preserved for hash-guarded
// supersedes cleanup of pre-PR-D `.md` files that users may have copied
// from the repo before PR D landed the embed-and-write managed flow.
//
// These bytes are byte-for-byte copies of what `plugin/cursor/.cursor/rules/`
// shipped before PR D's rename to `.mdc`. They are NOT on the production
// write path; they exist solely so `supersedesGuardedDelete` can recognize
// verbatim user copies and safely remove them while preserving any
// user-edited variants.
//
// Sunset trigger (mirrors renderV1CursorBlockBody): once `task plugin:check`
// reports zero pre-rename `.md` files in the dogfood repo for two
// consecutive releases, both vars and the helper below can be removed.
//
// Mirror copies at testdata/cursor-vestigial/ serve integration_test.go,
// which declares `package managedfiles_test` (external test package) and
// cannot reach these package-private vars. The two locations must be
// byte-identical; TestVestigialBytesMatchTestdataFixtures enforces this.

//go:embed embedded/cursor/vestigial/specgraph.md
var vestigialCursorSpecgraphMD []byte

//go:embed embedded/cursor/vestigial/post-stage.md
var vestigialCursorPostStageMD []byte

// vestigialCursorRulePriorHash returns the expected prior-canonical hash
// for a SupersedesPath value that points at one of the pre-rename Cursor
// rule files. Mirrors computePriorCanonical in markdownblock.go but reads
// from static embedded bytes (not a renderer + ProjectParams).
//
// Panics on unknown supersedesPath — every SupersedesPath in the manifest
// must have a corresponding case here.
func vestigialCursorRulePriorHash(supersedesPath string) string {
	switch supersedesPath {
	case ".cursor/rules/specgraph.md":
		return HashExcludingSentinel(CommentNone, vestigialCursorSpecgraphMD)
	case ".cursor/rules/post-stage.md":
		return HashExcludingSentinel(CommentNone, vestigialCursorPostStageMD)
	default:
		panic(fmt.Sprintf("no vestigial prior-canonical bytes for SupersedesPath %q", supersedesPath))
	}
}
