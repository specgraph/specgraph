// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/seanb4t/specgraph/internal/storage"
)

func TestGitHubAdapter_Name(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "owner/repo")
	if got := g.Name(); got != storage.SyncAdapterGitHub {
		t.Errorf("Name() = %q, want %q", got, storage.SyncAdapterGitHub)
	}
}

func TestGitHubAdapter_Available(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		output: []byte("gh version 2.60.0 (2026-01-15)\n"),
	}, "owner/repo")
	if err := g.Available(); err != nil {
		t.Errorf("Available() unexpected error: %v", err)
	}
}

func TestGitHubAdapter_AvailableNotInstalled(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		err: errors.New("exec: \"gh\": executable file not found in $PATH"),
	}, "owner/repo")
	err := g.Available()
	if err == nil {
		t.Fatal("Available() expected error, got nil")
	}
	if !errors.Is(err, ErrAdapterNotAvailable) {
		t.Errorf("Available() error = %v, want ErrAdapterNotAvailable", err)
	}
}

func TestGitHubAdapter_Push(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		output: []byte("https://github.com/owner/repo/issues/42\n"),
	}, "owner/repo")
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP1,
	}
	id, err := g.Push(context.Background(), spec)
	if err != nil {
		t.Fatalf("Push() unexpected error: %v", err)
	}
	if id != "42" {
		t.Errorf("Push() id = %q, want %q", id, "42")
	}
}

func TestGitHubAdapter_PushError(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		err: errors.New("command failed"),
	}, "owner/repo")
	spec := &storage.Spec{
		Slug:   "my-spec",
		Intent: "Build a thing",
	}
	_, err := g.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error, got nil")
	}
	if !errors.Is(err, ErrPushFailed) {
		t.Errorf("Push() error = %v, want ErrPushFailed", err)
	}
}

func TestGitHubAdapter_Pull(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		output: []byte(`{"state":"OPEN"}`),
	}, "owner/repo")
	status, err := g.Pull(context.Background(), "42")
	if err != nil {
		t.Fatalf("Pull() unexpected error: %v", err)
	}
	if status != "OPEN" {
		t.Errorf("Pull() status = %q, want %q", status, "OPEN")
	}
}

func TestGitHubAdapter_PullClosed(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		output: []byte(`{"state":"CLOSED"}`),
	}, "owner/repo")
	status, err := g.Pull(context.Background(), "42")
	if err != nil {
		t.Fatalf("Pull() unexpected error: %v", err)
	}
	if status != "CLOSED" {
		t.Errorf("Pull() status = %q, want %q", status, "CLOSED")
	}
}

func TestGitHubAdapter_PullError(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		err: errors.New("command failed"),
	}, "owner/repo")
	_, err := g.Pull(context.Background(), "42")
	if err == nil {
		t.Fatal("Pull() expected error, got nil")
	}
	if !errors.Is(err, ErrPullFailed) {
		t.Errorf("Pull() error = %v, want ErrPullFailed", err)
	}
}

func TestFormatIssueBody(t *testing.T) {
	spec := &storage.Spec{
		Slug:       "my-spec",
		Intent:     "Build a thing",
		Stage:      storage.SpecStageSpark,
		Priority:   storage.SpecPriorityP1,
		Complexity: "medium",
		Version:    3,
	}
	body := formatIssueBody(spec)
	if body == "" {
		t.Fatal("formatIssueBody() returned empty string")
	}
	// Verify key fields appear in the body.
	for _, want := range []string{"my-spec", "Build a thing", "spark", "p1", "medium", "3"} {
		if !strings.Contains(body, want) {
			t.Errorf("formatIssueBody() missing %q in:\n%s", want, body)
		}
	}
}

func TestFormatLabels(t *testing.T) {
	spec := &storage.Spec{
		Stage:    storage.SpecStageApproved,
		Priority: storage.SpecPriorityP0,
	}
	labels := formatLabels(spec)
	for _, want := range []string{"specgraph", "approved", "p0"} {
		if !strings.Contains(labels, want) {
			t.Errorf("formatLabels() = %q, missing %q", labels, want)
		}
	}
}
