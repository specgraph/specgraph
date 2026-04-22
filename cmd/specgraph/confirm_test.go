// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"strings"
	"testing"
)

// confirmDestructive is the guarded prompt used by `specgraph down --purge`
// and any future destructive command. See design 2026-04-22-cli-lifecycle-split.
//
// Rules under test:
//   - --yes bypasses the prompt, regardless of TTY.
//   - On a TTY without --yes, user must type y/yes to proceed; anything else aborts.
//   - Off a TTY without --yes, the command errors (no silent destruction in scripts).

func TestConfirmDestructive_YesBypassesPrompt(t *testing.T) {
	var out bytes.Buffer
	// Even non-TTY, --yes should skip the error.
	if err := confirmDestructive(strings.NewReader(""), &out, false, true, "destroy everything?"); err != nil {
		t.Fatalf("--yes on non-TTY should not error, got: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("--yes should not prompt, but wrote: %q", out.String())
	}
}

func TestConfirmDestructive_NonTTYWithoutYesErrors(t *testing.T) {
	var out bytes.Buffer
	err := confirmDestructive(strings.NewReader(""), &out, false, false, "destroy?")
	if err == nil {
		t.Fatal("non-TTY without --yes should error, got nil")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should mention --yes, got: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("should not prompt on non-TTY, but wrote: %q", out.String())
	}
}

func TestConfirmDestructive_TTYAcceptsY(t *testing.T) {
	var out bytes.Buffer
	if err := confirmDestructive(strings.NewReader("y\n"), &out, true, false, "destroy?"); err != nil {
		t.Fatalf("y should proceed, got: %v", err)
	}
	if !strings.Contains(out.String(), "destroy?") {
		t.Errorf("should have prompted with message, wrote: %q", out.String())
	}
}

func TestConfirmDestructive_TTYAcceptsYes(t *testing.T) {
	var out bytes.Buffer
	if err := confirmDestructive(strings.NewReader("yes\n"), &out, true, false, "destroy?"); err != nil {
		t.Fatalf("yes should proceed, got: %v", err)
	}
}

func TestConfirmDestructive_TTYAcceptsCaseInsensitive(t *testing.T) {
	var out bytes.Buffer
	if err := confirmDestructive(strings.NewReader("Y\n"), &out, true, false, "destroy?"); err != nil {
		t.Fatalf("Y should proceed (case-insensitive), got: %v", err)
	}
}

func TestConfirmDestructive_TTYRejectsDefaultEnter(t *testing.T) {
	var out bytes.Buffer
	err := confirmDestructive(strings.NewReader("\n"), &out, true, false, "destroy?")
	if err == nil {
		t.Fatal("empty line (default N) should abort, got nil")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("error should say aborted, got: %v", err)
	}
}

func TestConfirmDestructive_TTYRejectsN(t *testing.T) {
	var out bytes.Buffer
	err := confirmDestructive(strings.NewReader("n\n"), &out, true, false, "destroy?")
	if err == nil {
		t.Fatal("n should abort, got nil")
	}
}

func TestConfirmDestructive_TTYRejectsRandom(t *testing.T) {
	var out bytes.Buffer
	err := confirmDestructive(strings.NewReader("maybe\n"), &out, true, false, "destroy?")
	if err == nil {
		t.Fatal("random answer should abort, got nil")
	}
}
