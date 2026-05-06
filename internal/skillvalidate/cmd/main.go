// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Command skillvalidate is the CLI entry point invoked by `task skills:validate`.
// It walks each path argument looking for SKILL.md files and reports their
// conformance with the agentskills.io minimal contract: required name +
// description frontmatter, name matching directory, non-empty body.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/specgraph/specgraph/internal/skillvalidate"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: skillvalidate <path>...")
		os.Exit(2)
	}

	results, err := skillvalidate.ValidateRoots(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillvalidate: %v\n", err)
		os.Exit(2)
	}

	var report strings.Builder
	allOK := skillvalidate.Summarize(results, &report)
	fmt.Print(report.String())

	if !allOK {
		os.Exit(1)
	}
}
