// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

func TestDoctorReport_BinaryGroupAllHealthy(t *testing.T) {
	rep := DoctorReport{}
	rep.Binary = runBinaryGroup() // builds the group; no external deps
	if !rep.Binary.OK {
		t.Errorf("Binary group not OK: %+v", rep.Binary)
	}
}

func TestDoctorReport_Render_CompactWhenAllGreen(t *testing.T) {
	rep := DoctorReport{
		Binary: BinaryReport{OK: true, Version: "0.7.3", Commit: "abc1234"},
	}
	var buf bytes.Buffer
	renderText(&buf, &rep, false /*verbose*/)
	out := buf.String()
	if !strings.Contains(out, "Binary:") || !strings.Contains(out, "0.7.3") {
		t.Errorf("compact render missing binary line: %s", out)
	}
}

func TestDoctorReport_Render_JSONStableSchema(t *testing.T) {
	rep := DoctorReport{
		ExitCode: 0,
		Binary:   BinaryReport{OK: true, Version: "0.7.3", Commit: "abc1234"},
	}
	var buf bytes.Buffer
	renderJSON(&buf, &rep)
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if got["exitCode"].(float64) != 0 {
		t.Errorf("exitCode = %v, want 0", got["exitCode"])
	}
	if groups, ok := got["groups"].(map[string]any); !ok {
		t.Errorf("missing groups object: %s", buf.String())
	} else {
		if _, ok := groups["binary"]; !ok {
			t.Errorf("missing groups.binary")
		}
	}
}

func TestDoctorReport_ExitZeroForcesZero(t *testing.T) {
	rep := DoctorReport{ExitCode: 1}
	if code := finalExitCode(&rep, true /*exitZero*/); code != 0 {
		t.Errorf("--exit-zero with unhealthy state: exit = %d, want 0", code)
	}
	if code := finalExitCode(&rep, false); code != 1 {
		t.Errorf("normal mode with unhealthy state: exit = %d, want 1", code)
	}
}

func TestDoctorReport_ProjectGroup_NoProjectIsOK(t *testing.T) {
	dir := t.TempDir() // empty — no .specgraph.yaml anywhere up the tree
	rep := runProjectConfigGroup(dir)
	if !rep.OK {
		t.Errorf("no-project case: OK = false, want true (%+v)", rep)
	}
}

func TestDoctorReport_ProjectGroup_UnknownKeyReported(t *testing.T) {
	dir := t.TempDir()
	yaml := `project: x
fnord: 42
`
	if err := os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	rep := runProjectConfigGroup(dir)
	if rep.OK {
		t.Errorf("unknown key not flagged: %+v", rep)
	}
	if !strings.Contains(rep.StrictError, "fnord") {
		t.Errorf("StrictError missing 'fnord': %q", rep.StrictError)
	}
}

func TestDoctorReport_Render_ServerOKLine(t *testing.T) {
	rep := DoctorReport{
		Binary:  BinaryReport{OK: true, Version: "0.7.3", Commit: "abc1234"},
		Project: ProjectReport{OK: true},
		Server: ServerReport{
			OK:           true,
			Reachable:    true,
			Version:      "0.7.3",
			MCPHandshake: "ok",
			SkillsCount:  6,
		},
	}
	var buf bytes.Buffer
	renderText(&buf, &rep, false /*verbose*/)
	out := buf.String()
	if !strings.Contains(out, "Server:") {
		t.Errorf("render missing Server line: %s", out)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("render missing OK in Server line: %s", out)
	}
	if !strings.Contains(out, "0.7.3") {
		t.Errorf("render missing version in Server line: %s", out)
	}
	if !strings.Contains(out, "skills=6") {
		t.Errorf("render missing skills count in Server line: %s", out)
	}
}

func TestServerStatusLine_UnreachableExpanded(t *testing.T) {
	rep := ServerReport{
		OK:           false,
		Reachable:    false,
		MCPHandshake: "skipped",
		Error:        "connection refused",
	}
	line := serverStatusLine(rep)
	if !strings.Contains(line, "UNREACHABLE") {
		t.Errorf("expected UNREACHABLE in line: %s", line)
	}
	if !strings.Contains(line, "connection refused") {
		t.Errorf("expected error text in line: %s", line)
	}
}

func TestCountSkillsFromJSON(t *testing.T) {
	input := `[{"name":"a","summary":"s1","uri":"specgraph://skills/a"},{"name":"b","summary":"s2","uri":"specgraph://skills/b"}]`
	count := countSkillsFromJSON(input)
	if count != 2 {
		t.Errorf("countSkillsFromJSON = %d, want 2", count)
	}
	if got := countSkillsFromJSON("not-json"); got != -1 {
		t.Errorf("countSkillsFromJSON(invalid) = %d, want -1", got)
	}
}

func TestManagedStatusLine_AllSynced(t *testing.T) {
	rep := ManagedReport{
		OK:     true,
		Synced: 14,
		Total:  14,
	}
	line := managedStatusLine(rep)
	if !strings.Contains(line, "14/14 synced") {
		t.Errorf("expected '14/14 synced' in line: %s", line)
	}
	if strings.Contains(line, "—") {
		t.Errorf("expected no breakdown when all synced: %s", line)
	}
}

func TestManagedStatusLine_Mixed(t *testing.T) {
	rep := ManagedReport{
		OK:     false,
		Synced: 11,
		Total:  14,
		Files: []managedfiles.FileState{
			{Path: "a", State: managedfiles.StateMissing},
			{Path: "b", State: managedfiles.StateStale},
			{Path: "c", State: managedfiles.StateDrifted},
			// 11 synced files omitted from the slice for brevity; the
			// breakdown derives from non-Synced entries only.
		},
	}
	line := managedStatusLine(rep)
	if !strings.Contains(line, "11/14 synced") {
		t.Errorf("expected '11/14 synced' in line: %s", line)
	}
	if !strings.Contains(line, "1 missing") {
		t.Errorf("expected '1 missing' in line: %s", line)
	}
	if !strings.Contains(line, "1 stale") {
		t.Errorf("expected '1 stale' in line: %s", line)
	}
	if !strings.Contains(line, "1 drifted") {
		t.Errorf("expected '1 drifted' in line: %s", line)
	}
}

func TestIsHostPinned(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"AGENTS.md", true},
		{".cursor/rules/specgraph.mdc", true},
		{".mcp.json", true},
		{".cursor/mcp.json", true},
		{"opencode.json", true},
		{".claude/settings.json", true},
		{".specgraph/agents/claude/routing-guide.md", false},
		{".specgraph/agents/opencode/specgraph.ts", false},
		{".specgraph/agents/claude/hooks/specgraph-post-stage.sh", false},
	}
	for _, c := range cases {
		if got := isHostPinned(c.path); got != c.want {
			t.Errorf("isHostPinned(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestDoctorReport_ProjectGroup_UnknownHarnessReported(t *testing.T) {
	dir := t.TempDir()
	yaml := `project: x
harnesses: [bogus]
`
	if err := os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	rep := runProjectConfigGroup(dir)
	if rep.OK {
		t.Errorf("unknown harness not flagged: %+v", rep)
	}
	if len(rep.UnknownNames) != 1 || rep.UnknownNames[0] != "bogus" {
		t.Errorf("UnknownNames = %v, want [bogus]", rep.UnknownNames)
	}
}

func TestHealthAlias_DispatchesAndEmitsDeprecationNotice(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	// Ignore the error — Server RPC will fail without a live server.
	_ = runHealth(nil, nil)

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "specgraph health: deprecated") {
		t.Errorf("stderr missing deprecation notice: %q", buf.String())
	}
}
