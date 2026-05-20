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
