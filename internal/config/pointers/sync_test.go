// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func defaultOpts() Options {
	return Options{
		ServerURL:   "http://127.0.0.1:7890",
		ProjectSlug: "specgraph",
	}
}

func TestSync_CreatesAgentsMD(t *testing.T) {
	dir := t.TempDir()
	results := Sync(dir, defaultOpts())
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	r := results[0]
	if r.Path != "AGENTS.md" {
		t.Errorf("results[0].Path = %q, want AGENTS.md", r.Path)
	}
	if r.Action != ActionCreated {
		t.Errorf("results[0].Action = %q, want %q", r.Action, ActionCreated)
	}
	if r.Err != nil {
		t.Errorf("results[0].Err = %v, want nil", r.Err)
	}

	body, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	bs := string(body)
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("AGENTS.md missing start marker:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:end -->") {
		t.Errorf("AGENTS.md missing end marker:\n%s", bs)
	}
	if !strings.Contains(bs, "http://127.0.0.1:7890") {
		t.Errorf("AGENTS.md missing serverURL:\n%s", bs)
	}
	if !strings.HasSuffix(bs, "\n") {
		t.Errorf("AGENTS.md must end with newline; got last bytes %q", bs[max(0, len(bs)-5):])
	}
}

func TestSync_NoOpWhenIdentical(t *testing.T) {
	dir := t.TempDir()
	if r := Sync(dir, defaultOpts())[0]; r.Action != ActionCreated {
		t.Fatalf("first run: Action = %q, want %q", r.Action, ActionCreated)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionNoOp {
		t.Errorf("second run: Action = %q, want %q", r.Action, ActionNoOp)
	}
}

func TestSync_UpdatesWhenContentDiffers(t *testing.T) {
	dir := t.TempDir()
	Sync(dir, defaultOpts())
	r := Sync(dir, Options{ServerURL: "http://example.com:8080", ProjectSlug: "specgraph"})[0]
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(body), "http://example.com:8080") {
		t.Errorf("AGENTS.md does not reflect new serverURL:\n%s", body)
	}
}

func TestSync_PreservesUserContentAroundBlock(t *testing.T) {
	dir := t.TempDir()
	const userTop = "# My project\n\nUser preamble.\n\n"
	const userBottom = "\n## My footer\n\nUser tail.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(userTop+userBottom), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	bs := string(body)
	if !strings.Contains(bs, "User preamble.") || !strings.Contains(bs, "User tail.") {
		t.Errorf("user content not preserved:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing:\n%s", bs)
	}
}

func TestSync_OverwritesUserContentInsideBlock(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start v=1 -->\nUSER NOTES THAT MUST DISAPPEAR\n<!-- specgraph:init:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	Sync(dir, defaultOpts())
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(string(body), "USER NOTES THAT MUST DISAPPEAR") {
		t.Errorf("inside-block user content was not overwritten:\n%s", body)
	}
}

func TestSync_AppendsBlockToFileWithoutMarkers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# User AGENTS\n\nbody.\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	bs := string(body)
	if !strings.HasPrefix(bs, "# User AGENTS\n\nbody.\n") {
		t.Errorf("user content not preserved at top:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing:\n%s", bs)
	}
}
