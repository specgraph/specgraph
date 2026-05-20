// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	_ "embed"
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

// registerVestigialCursorPriors populates the unified priors registry
// with the pre-PR-D Cursor rule canonical hashes. Each entry maps a
// current managed-file Path (the post-rename `.mdc`) to the canonical
// SHA256 hex of the pre-rename `.md` bytes. After PR E's unification
// (Task 9), all priors lookups — PR C's computePriorCanonical, PR D's
// vestigial map, and PR E's JSON priors — route through `priorsFor`.
//
// DO NOT change the embedded vestigial bytes without a deliberate
// decision: doing so would regress SupersedesPath cleanup for any user
// whose `.cursor/rules/specgraph.md` or `.cursor/rules/post-stage.md`
// is a verbatim copy from the pre-PR-D repo. The pinned-hash regression
// test in vestigial_cursor_rules_test.go guards against accidental
// drift of the resulting hash values.
//
// Implemented as a package-level variable initializer (rather than an
// init() function) so registration completes before any package init()
// runs — in particular, before manifest.go's init() validates that each
// WholeFile entry with a SupersedesPath has a registered prior. The `_`
// receiver keeps the side-effect call alive while satisfying lint's
// unused-variable check.
var _ = func() bool {
	registerPrior(
		".cursor/rules/specgraph.mdc",
		HashExcludingSentinel(CommentNone, vestigialCursorSpecgraphMD),
	)
	registerPrior(
		".cursor/rules/specgraph-post-stage.mdc",
		HashExcludingSentinel(CommentNone, vestigialCursorPostStageMD),
	)
	return true
}()
