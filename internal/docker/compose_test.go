// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package docker

import (
	"reflect"
	"testing"
)

// The argv tests assert the exact flags passed to `docker compose`. They are
// regression guards against accidental re-introduction of destructive flags
// like `-v` on `down` (see 2026-04-22-cli-lifecycle-split-design.md).

func TestComposeUpArgsOmitsForceRecreateAndRenewVolumes(t *testing.T) {
	got := composeUpArgs("/tmp/compose.yaml")
	want := []string{"compose", "-f", "/tmp/compose.yaml", "up", "-d", "--wait"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("composeUpArgs produced wrong argv\n got:  %v\n want: %v", got, want)
	}
}

func TestComposeDownArgsOmitsVolumeFlag(t *testing.T) {
	got := composeDownArgs("/tmp/compose.yaml")
	want := []string{"compose", "-f", "/tmp/compose.yaml", "down", "--timeout", "10"}
	// CRITICAL: no -v. The -v flag removes named volumes and was the cause of
	// silent data loss on every `specgraph down`. Do not re-add it here.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("composeDownArgs produced wrong argv\n got:  %v\n want: %v", got, want)
	}
}

func TestComposeStopArgsUsesStopNotDown(t *testing.T) {
	got := composeStopArgs("/tmp/compose.yaml")
	want := []string{"compose", "-f", "/tmp/compose.yaml", "stop", "--timeout", "10"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("composeStopArgs produced wrong argv\n got:  %v\n want: %v", got, want)
	}
}

func TestComposeDownWithVolumesIncludesVolumeFlag(t *testing.T) {
	got := composeDownWithVolumesArgs("/tmp/compose.yaml")
	want := []string{"compose", "-f", "/tmp/compose.yaml", "down", "--timeout", "10", "-v"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("composeDownWithVolumesArgs produced wrong argv\n got:  %v\n want: %v", got, want)
	}
}
