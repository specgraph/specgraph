// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestWriteHeader(t *testing.T) {
	var sb strings.Builder
	writeHeader(&sb)
	got := sb.String()

	if !strings.Contains(got, "# CLI Reference") {
		t.Error("header missing title")
	}
	if !strings.Contains(got, "Auto-generated") {
		t.Error("header missing auto-generated notice")
	}
}

func TestWriteTOC(t *testing.T) {
	var sb strings.Builder
	writeTOC(&sb)
	got := sb.String()

	// Every group name should appear as a link.
	for _, g := range commandGroups {
		if !strings.Contains(got, g.Name) {
			t.Errorf("TOC missing group %q", g.Name)
		}
	}
	// "Drift & Linting" anchor should not have double-hyphen.
	if strings.Contains(got, "drift--linting") {
		t.Error("TOC anchor has double-hyphen for 'Drift & Linting'")
	}
	if !strings.Contains(got, "#drift-linting") {
		t.Error("TOC missing expected anchor #drift-linting")
	}
}

func TestWriteCommand_Short(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test-cmd <arg>",
		Short: "A test command",
	}

	var sb strings.Builder
	writeCommand(&sb, cmd, "###")
	got := sb.String()

	if !strings.Contains(got, "### test-cmd") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "A test command") {
		t.Error("missing short description")
	}
	if !strings.Contains(got, "test-cmd <arg>") {
		t.Error("missing usage line")
	}
}

func TestWriteCommand_LongOverridesShort(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test-cmd",
		Short: "short desc",
		Long:  "long description here",
	}

	var sb strings.Builder
	writeCommand(&sb, cmd, "###")
	got := sb.String()

	if !strings.Contains(got, "long description here") {
		t.Error("missing long description")
	}
	// Long should be used instead of short, so short should not appear
	// as a standalone paragraph (it may appear elsewhere in the heading).
	lines := strings.Split(got, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "short desc" {
			t.Error("short description should not appear when long is set")
		}
	}
}

func TestWriteCommand_WithFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "flagged",
		Short: "has flags",
	}
	cmd.Flags().Bool("verbose", false, "enable verbose output")

	var sb strings.Builder
	writeCommand(&sb, cmd, "###")
	got := sb.String()

	if !strings.Contains(got, "**Flags:**") {
		t.Error("missing flags section")
	}
	if !strings.Contains(got, "--verbose") {
		t.Error("missing --verbose flag")
	}
}

func TestWriteCommand_WithExample(t *testing.T) {
	cmd := &cobra.Command{
		Use:     "example-cmd",
		Short:   "has example",
		Example: "specgraph example-cmd --foo bar",
	}

	var sb strings.Builder
	writeCommand(&sb, cmd, "###")
	got := sb.String()

	if !strings.Contains(got, "**Examples:**") {
		t.Error("missing examples section")
	}
	if !strings.Contains(got, "specgraph example-cmd --foo bar") {
		t.Error("missing example content")
	}
}

func TestWriteCommand_RecursesSubcommands(t *testing.T) {
	parent := &cobra.Command{Use: "parent", Short: "parent cmd"}
	child := &cobra.Command{Use: "child", Short: "child cmd"}
	parent.AddCommand(child)

	var sb strings.Builder
	writeCommand(&sb, parent, "###")
	got := sb.String()

	if !strings.Contains(got, "#### parent child") {
		t.Error("missing subcommand heading")
	}
	if !strings.Contains(got, "child cmd") {
		t.Error("missing subcommand description")
	}
}

func TestWriteCommand_SkipsHelp(t *testing.T) {
	parent := &cobra.Command{Use: "parent", Short: "parent cmd"}
	child := &cobra.Command{Use: "real", Short: "real cmd"}
	parent.AddCommand(child)
	// Cobra auto-adds a help subcommand.

	var sb strings.Builder
	writeCommand(&sb, parent, "###")
	got := sb.String()

	if strings.Contains(got, "#### parent help") {
		t.Error("help subcommand should be skipped")
	}
}

func TestWriteGroups_AllCommandsGrouped(t *testing.T) {
	var sb strings.Builder
	writeGroups(&sb, rootCmd)
	got := sb.String()

	// Should NOT have "Other" section since all commands are grouped.
	if strings.Contains(got, "## Other") {
		t.Error("unexpected Other section — all commands should be grouped")
	}
}

func TestWriteGroups_OtherCatchall(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.AddCommand(&cobra.Command{Use: "ungrouped", Short: "not in any group"})

	var sb strings.Builder
	writeGroups(&sb, root)
	got := sb.String()

	if !strings.Contains(got, "## Other") {
		t.Error("missing Other section for ungrouped command")
	}
	if !strings.Contains(got, "ungrouped") {
		t.Error("ungrouped command not in Other section")
	}
}

func TestRunDocsCli_WritesFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "subdir", "reference.md")

	err := runDocsCli(docsCliCmd, []string{outPath})
	if err != nil {
		t.Fatalf("runDocsCli: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# CLI Reference") {
		t.Error("output missing header")
	}
	if !strings.Contains(content, "## Spec Management") {
		t.Error("output missing Spec Management group")
	}
}

func TestRunDocsCli_DefaultPath(t *testing.T) {
	// Verify the default path is set correctly.
	if docsCliDefaultPath != filepath.Join("site", "docs", "cli-reference.md") {
		t.Errorf("unexpected default path: %s", docsCliDefaultPath)
	}
}
