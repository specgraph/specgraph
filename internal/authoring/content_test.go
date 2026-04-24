// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import "testing"

func TestEmbeddedContent_Present(t *testing.T) {
	names := []string{
		"persona.md",
		"orchestration.md",
		"conversation-recording.md",
		"quality-heuristics.md",
		"stage-spark.md",
		"stage-shape.md",
		"stage-specify.md",
		"stage-decompose.md",
		"stage-approve.md",
	}
	for _, n := range names {
		data, err := Content(n)
		if err != nil {
			t.Errorf("content/%s: %v", n, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("content/%s: empty", n)
		}
	}
}
