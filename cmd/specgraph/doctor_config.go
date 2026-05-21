// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/specgraph/specgraph/internal/config"
)

// ProjectReport describes the .specgraph.yaml parse + harness resolution.
type ProjectReport struct {
	OK           bool     `json:"ok"`
	Harnesses    []string `json:"harnesses"`
	StrictError  string   `json:"strictError,omitempty"`  // unknown top-level key, etc.
	UnknownNames []string `json:"unknownNames,omitempty"` // names in cfg.Harnesses that didn't resolve
}

// runProjectConfigGroup loads cfg via the lenient LoadProject path
// (matching everything else), then re-validates strictly via the new
// ValidateProjectStrict helper. The combined result is OK only if both
// pass and every Harnesses entry resolves to a known Harness.
func runProjectConfigGroup(cwd string) ProjectReport {
	rep := ProjectReport{OK: true}
	root, err := config.FindProjectRoot(cwd)
	if err != nil {
		if errors.Is(err, config.ErrProjectNotFound) {
			// No project config — treat as OK (the binary works without one).
			return ProjectReport{OK: true}
		}
		rep.OK = false
		rep.StrictError = err.Error()
		return rep
	}
	cfg, err := config.LoadProject(root)
	if err != nil {
		rep.OK = false
		rep.StrictError = err.Error()
		return rep
	}
	rep.Harnesses = cfg.Harnesses

	if err := config.ValidateProjectStrict(filepath.Join(root, ".specgraph.yaml")); err != nil {
		rep.OK = false
		rep.StrictError = err.Error()
	}

	// Resolve every Harnesses entry against the known names.
	for _, name := range cfg.Harnesses {
		switch name {
		case "claude", "cursor", "opencode":
			// resolved
		default:
			rep.OK = false
			rep.UnknownNames = append(rep.UnknownNames, name)
		}
	}
	return rep
}

// projectStatusLine renders the compact form. When validation fails only
// because of unknown harness names (StrictError empty), the message would
// otherwise render as "PROBLEM ()" — we include the offending names so
// the contributor sees what to fix.
func projectStatusLine(rep ProjectReport) string {
	if rep.OK {
		if len(rep.Harnesses) == 0 {
			return "Project config: OK (no project-level customization)"
		}
		return fmt.Sprintf("Project config: OK (%d harnesses enabled)", len(rep.Harnesses))
	}
	switch {
	case rep.StrictError != "" && len(rep.UnknownNames) > 0:
		return fmt.Sprintf("Project config: PROBLEM (%s; unknown harnesses: %s)",
			rep.StrictError, strings.Join(rep.UnknownNames, ", "))
	case rep.StrictError != "":
		return fmt.Sprintf("Project config: PROBLEM (%s)", rep.StrictError)
	case len(rep.UnknownNames) > 0:
		return fmt.Sprintf("Project config: PROBLEM (unknown harnesses: %s)",
			strings.Join(rep.UnknownNames, ", "))
	default:
		return "Project config: PROBLEM"
	}
}
