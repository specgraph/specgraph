// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

// BinaryReport describes the running specgraph binary.
type BinaryReport struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// runBinaryGroup reports the running binary's identity. version and
// commit are goreleaser-injected ldflags variables declared in main.go.
// OK iff both are non-empty.
func runBinaryGroup() BinaryReport {
	rep := BinaryReport{
		Version: version, // existing ldflags var
		Commit:  commit,  // existing ldflags var
	}
	rep.OK = rep.Version != "" && rep.Commit != ""
	return rep
}
