// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Pinned hashes for the pre-PR-D canonical .md files. Updating either of
// these constants without a deliberate decision breaks SupersedesPath
// cleanup for any user whose .cursor/rules/specgraph.md or post-stage.md
// is a verbatim copy from the pre-PR-D repo. Do not update unless you've
// also confirmed:
//
//	(a) no current dogfood user has the old bytes on disk, AND
//	(b) the vestigial bytes are being intentionally re-pinned.
const (
	pinnedHashCursorSpecgraphMD = "df39ce6a814d047573647938efc61fd38aac340265bb3cc8a1f0e6564c98f710"
	pinnedHashCursorPostStageMD = "bc3a5d349a3ac1becfcaa509a72bb51fc4d9584126bf5d20c986227e8ec743f6"
)

func TestVestigialCursorRulePriorHashPinned(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{".cursor/rules/specgraph.md", pinnedHashCursorSpecgraphMD},
		{".cursor/rules/post-stage.md", pinnedHashCursorPostStageMD},
	}
	for _, tc := range cases {
		got := vestigialCursorRulePriorHash(tc.path)
		if got != tc.want {
			t.Errorf("vestigialCursorRulePriorHash(%q) = %s, want %s\n\nIf you intentionally changed pre-rename canonical bytes, update the pinnedHash constants in this file — but note this breaks SupersedesPath cleanup for any user with the old verbatim bytes on disk.",
				tc.path, got, tc.want)
		}
	}
}

func TestVestigialCursorRulePriorHash_UnknownPathPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on unknown path; got none")
		}
	}()
	_ = vestigialCursorRulePriorHash("does/not/exist.md")
}

func TestVestigialBytesMatchTestdataFixtures(t *testing.T) {
	// Cross-check the embedded vestigial bytes against the testdata
	// fixture copies used by integration_test.go. If these diverge,
	// integration tests will silently exercise the wrong content.
	cases := []struct {
		fixture string
		embed   []byte
	}{
		{"cursor-vestigial/specgraph.md", vestigialCursorSpecgraphMD},
		{"cursor-vestigial/post-stage.md", vestigialCursorPostStageMD},
	}
	for _, tc := range cases {
		path := filepath.Join("testdata", tc.fixture)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !bytes.Equal(got, tc.embed) {
			t.Errorf("%s diverges from embedded/cursor/vestigial/%s\n\nEnsure both locations are byte-identical copies of the pre-rename canonical bytes. Update internal/config/managedfiles/embedded/cursor/vestigial/ and testdata/cursor-vestigial/ together; TestVestigialCursorRulePriorHashPinned will also fail if the embedded bytes drifted.",
				path, filepath.Base(tc.fixture))
		}
	}
}
