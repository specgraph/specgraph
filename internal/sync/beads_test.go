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

// mockRunner implements CommandRunner for testing.
type mockRunner struct {
	output []byte
	err    error
}

func (m *mockRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return m.output, m.err
}

func TestBeadsAdapter_Name(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{})
	if got := b.Name(); got != storage.SyncAdapterBeads {
		t.Errorf("Name() = %q, want %q", got, storage.SyncAdapterBeads)
	}
}

func TestBeadsAdapter_Available(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{
		output: []byte("bd version 0.1.0\n"),
	})
	if err := b.Available(context.Background()); err != nil {
		t.Errorf("Available() unexpected error: %v", err)
	}
}

func TestBeadsAdapter_AvailableNotInstalled(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{
		err: errors.New("exec: \"bd\": executable file not found in $PATH"),
	})
	err := b.Available(context.Background())
	if err == nil {
		t.Fatal("Available() expected error, got nil")
	}
	if !errors.Is(err, ErrAdapterNotAvailable) {
		t.Errorf("Available() error = %v, want ErrAdapterNotAvailable", err)
	}
}

func TestBeadsAdapter_Push(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{
		output: []byte(`{"id": "bead-abc123", "title": "[spec] my-spec"}`),
	})
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP2,
	}
	id, err := b.Push(context.Background(), spec)
	if err != nil {
		t.Fatalf("Push() unexpected error: %v", err)
	}
	if id != "bead-abc123" {
		t.Errorf("Push() id = %q, want %q", id, "bead-abc123")
	}
}

func TestBeadsAdapter_PushError(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{
		err: errors.New("command failed"),
	})
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP2,
	}
	_, err := b.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestBeadsAdapter_Pull(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{
		output: []byte(`{"id": "bead-abc123", "status": "in_progress"}`),
	})
	status, err := b.Pull(context.Background(), "bead-abc123")
	if err != nil {
		t.Fatalf("Pull() unexpected error: %v", err)
	}
	if status != "in_progress" {
		t.Errorf("Pull() status = %q, want %q", status, "in_progress")
	}
}

func TestBeadsAdapter_PullError(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{
		err: errors.New("command failed"),
	})
	_, err := b.Pull(context.Background(), "bead-abc123")
	if err == nil {
		t.Fatal("Pull() expected error, got nil")
	}
	if !errors.Is(err, errPullFailed) {
		t.Errorf("Pull() error = %v, want errPullFailed", err)
	}
}

func TestBeadsAdapter_PushEmptySlug(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{})
	_, err := b.Push(context.Background(), &storage.Spec{Slug: ""})
	if err == nil {
		t.Fatal("Push() expected error for empty slug, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestBeadsAdapter_PushMalformedJSON(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{
		output: []byte(`not json`),
	})
	_, err := b.Push(context.Background(), &storage.Spec{Slug: "test-spec"})
	if err == nil {
		t.Fatal("Push() expected error for malformed JSON, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestBeadsAdapter_PushMissingID(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{
		output: []byte(`{"id": ""}`),
	})
	_, err := b.Push(context.Background(), &storage.Spec{Slug: "test-spec"})
	if err == nil {
		t.Fatal("Push() expected error for empty ID, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestBeadsAdapter_PushInvalidStage(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{})
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "test",
		Stage:    storage.SpecStage("bogus"),
		Priority: storage.SpecPriorityP1,
	}
	_, err := b.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error for invalid stage, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
	if !strings.Contains(err.Error(), "invalid spec stage") {
		t.Errorf("Push() error should mention invalid stage, got: %v", err)
	}
}

func TestBeadsAdapter_PushInvalidPriority(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{})
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "test",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriority("invalid"),
	}
	_, err := b.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error for invalid priority, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
	if !strings.Contains(err.Error(), "invalid spec priority") {
		t.Errorf("Push() error should mention invalid priority, got: %v", err)
	}
}

func TestBeadsAdapter_PullInvalidID(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{})
	_, err := b.Pull(context.Background(), "--help")
	if err == nil {
		t.Fatal("Pull() expected error for invalid ID, got nil")
	}
	if !errors.Is(err, errPullFailed) {
		t.Errorf("Pull() error = %v, want errPullFailed", err)
	}
}

func TestBeadsAdapter_PullEmptyStatus(t *testing.T) {
	b := NewBeadsAdapter(&mockRunner{
		output: []byte(`{"id": "bead-abc123", "status": ""}`),
	})
	_, err := b.Pull(context.Background(), "bead-abc123")
	if err == nil {
		t.Fatal("Pull() expected error for empty status, got nil")
	}
	if !errors.Is(err, errPullFailed) {
		t.Errorf("Pull() error = %v, want errPullFailed", err)
	}
}
