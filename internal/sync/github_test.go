// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
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
	if err := g.Available(context.Background()); err != nil {
		t.Errorf("Available() unexpected error: %v", err)
	}
}

func TestGitHubAdapter_AvailableNotInstalled(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		err: errors.New("exec: \"gh\": executable file not found in $PATH"),
	}, "owner/repo")
	err := g.Available(context.Background())
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
	if id != "https://github.com/owner/repo/issues/42" {
		t.Errorf("Push() id = %q, want %q", id, "https://github.com/owner/repo/issues/42")
	}
}

func TestGitHubAdapter_PushError(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		err: errors.New("command failed"),
	}, "owner/repo")
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP1,
	}
	_, err := g.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
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
	if !errors.Is(err, errPullFailed) {
		t.Errorf("Pull() error = %v, want errPullFailed", err)
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
		found := false
		for _, label := range labels {
			if label == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("formatLabels() = %v, missing %q", labels, want)
		}
	}
}

func TestGitHubAdapter_PushEmptySlug(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "owner/repo")
	_, err := g.Push(context.Background(), &storage.Spec{Slug: ""})
	if err == nil {
		t.Fatal("Push() expected error for empty slug, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestGitHubAdapter_PushInvalidURL(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		output: []byte("not-a-url\n"),
	}, "owner/repo")
	spec := &storage.Spec{Slug: "my-spec", Intent: "test", Stage: storage.SpecStageSpark, Priority: storage.SpecPriorityP1}
	_, err := g.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error for invalid URL, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestGitHubAdapter_PushNonNumericIssueNumber(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		output: []byte("https://github.com/owner/repo/issues/notanumber\n"),
	}, "owner/repo")
	spec := &storage.Spec{Slug: "my-spec", Intent: "test", Stage: storage.SpecStageSpark, Priority: storage.SpecPriorityP1}
	_, err := g.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error for non-numeric issue number, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestGitHubAdapter_PullEmptyState(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		output: []byte(`{"state":""}`),
	}, "owner/repo")
	_, err := g.Pull(context.Background(), "42")
	if err == nil {
		t.Fatal("Pull() expected error for empty state, got nil")
	}
	if !errors.Is(err, errPullFailed) {
		t.Errorf("Pull() error = %v, want errPullFailed", err)
	}
}

func TestGitHubAdapter_PullMalformedJSON(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		output: []byte("not json at all"),
	}, "owner/repo")
	_, err := g.Pull(context.Background(), "42")
	if err == nil {
		t.Fatal("Pull() expected error for malformed JSON, got nil")
	}
	if !errors.Is(err, errPullFailed) {
		t.Errorf("Pull() error = %v, want errPullFailed", err)
	}
}

type seqMockRunner struct {
	responses []struct {
		output []byte
		err    error
	}
	callIdx int
}

func (m *seqMockRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	if m.callIdx >= len(m.responses) {
		return nil, errors.New("seqMockRunner: no more responses")
	}
	r := m.responses[m.callIdx]
	m.callIdx++
	return r.output, r.err
}

func TestGitHubAdapter_AvailableNotAuthenticated(t *testing.T) {
	g := NewGitHubAdapter(&seqMockRunner{
		responses: []struct {
			output []byte
			err    error
		}{
			{output: []byte("gh version 2.60.0\n")},
			{err: errors.New("not logged in")},
		},
	}, "owner/repo")
	err := g.Available(context.Background())
	if err == nil {
		t.Fatal("Available() expected error, got nil")
	}
	if !errors.Is(err, ErrAdapterNotAvailable) {
		t.Errorf("Available() error = %v, want ErrAdapterNotAvailable", err)
	}
}

func TestGitHubAdapter_PushInvalidStage(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "owner/repo")
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "test",
		Stage:    storage.SpecStage("bogus"),
		Priority: storage.SpecPriorityP1,
	}
	_, err := g.Push(context.Background(), spec)
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

func TestGitHubAdapter_PushInvalidPriority(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "owner/repo")
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "test",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriority("invalid"),
	}
	_, err := g.Push(context.Background(), spec)
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

func TestGitHubAdapter_PullEmptyExternalID(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "")
	_, err := g.Pull(context.Background(), "")
	if err == nil {
		t.Fatal("Pull() expected error for empty externalID, got nil")
	}
	if !errors.Is(err, errPullFailed) {
		t.Errorf("Pull() error = %v, want errPullFailed", err)
	}
}

func TestGitHubAdapter_PullInvalidIssueRef(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "")
	_, err := g.Pull(context.Background(), "not-a-number")
	if err == nil {
		t.Fatal("Pull() expected error for non-numeric externalID, got nil")
	}
	if !errors.Is(err, errPullFailed) {
		t.Errorf("Pull() error = %v, want errPullFailed", err)
	}
}

func TestGitHubAdapter_PushEmptyRepo(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "")
	spec := &storage.Spec{
		Slug:     "test-spec",
		Intent:   "test",
		Stage:    storage.SpecStageApproved,
		Priority: storage.SpecPriorityP2,
	}
	_, err := g.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error for empty repo, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestGitHubAdapter_PushEmptyOutput(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{output: []byte("")}, "owner/repo")
	spec := &storage.Spec{
		Slug:     "test-spec",
		Intent:   "test",
		Stage:    storage.SpecStageApproved,
		Priority: storage.SpecPriorityP2,
	}
	_, err := g.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error for empty output, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestGitHubAdapter_PushBadScheme(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{output: []byte("http://github.com/owner/repo/issues/42")}, "owner/repo")
	spec := &storage.Spec{
		Slug:     "test-spec",
		Intent:   "test",
		Stage:    storage.SpecStageApproved,
		Priority: storage.SpecPriorityP2,
	}
	_, err := g.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error for non-https scheme, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestGitHubAdapter_PushInvalidHost(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{output: []byte("https://evil.example.com/owner/repo/issues/42")}, "owner/repo")
	spec := &storage.Spec{
		Slug:     "test-spec",
		Intent:   "test",
		Stage:    storage.SpecStageApproved,
		Priority: storage.SpecPriorityP2,
	}
	_, err := g.Push(context.Background(), spec)
	if err == nil {
		t.Fatal("Push() expected error for non-github.com host, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("Push() error = %v, want errPushFailed", err)
	}
}

func TestGitHubAdapter_PullInvalidHost(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "owner/repo")
	_, err := g.Pull(context.Background(), "https://evil.example.com/issues/42")
	if err == nil {
		t.Fatal("Pull() expected error for non-github.com host, got nil")
	}
	if !errors.Is(err, errPullFailed) {
		t.Errorf("Pull() error = %v, want errPullFailed", err)
	}
}

func TestGitHubAdapter_Pull_FullURL(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{
		output: []byte(`{"state":"OPEN"}`),
	}, "owner/repo")
	status, err := g.Pull(context.Background(), "https://github.com/owner/repo/issues/123")
	if err != nil {
		t.Fatalf("Pull() unexpected error for full URL: %v", err)
	}
	if status != "OPEN" {
		t.Errorf("Pull() status = %q, want %q", status, "OPEN")
	}
}

func TestGitHubAdapter_AvailableNoRepo(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "")
	err := g.Available(context.Background())
	if err == nil {
		t.Fatal("Available() expected error for empty repo, got nil")
	}
	if !errors.Is(err, ErrAdapterNotAvailable) {
		t.Errorf("Available() error = %v, want ErrAdapterNotAvailable", err)
	}
}

func TestGitHubAdapter_FindOrCreate_ExistingItem(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: []byte(`[{"number":42,"url":"https://github.com/owner/repo/issues/42"}]`), err: nil},
		},
	}
	g := NewGitHubAdapter(runner, "owner/repo")
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP1,
	}
	id, created, err := g.FindOrCreate(context.Background(), spec)
	if err != nil {
		t.Fatalf("FindOrCreate() unexpected error: %v", err)
	}
	if created {
		t.Error("FindOrCreate() created = true, want false")
	}
	if id != "https://github.com/owner/repo/issues/42" {
		t.Errorf("FindOrCreate() id = %q, want URL", id)
	}
}

func TestGitHubAdapter_FindOrCreate_NewItem(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: []byte(`[]`), err: nil},
			{output: []byte("https://github.com/owner/repo/issues/99\n"), err: nil},
		},
	}
	g := NewGitHubAdapter(runner, "owner/repo")
	spec := &storage.Spec{
		Slug:     "new-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP1,
	}
	id, created, err := g.FindOrCreate(context.Background(), spec)
	if err != nil {
		t.Fatalf("FindOrCreate() unexpected error: %v", err)
	}
	if !created {
		t.Error("FindOrCreate() created = false, want true")
	}
	if id != "https://github.com/owner/repo/issues/99" {
		t.Errorf("FindOrCreate() id = %q, want URL", id)
	}
}

func TestGitHubAdapter_FindOrCreate_EmptySlug(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "owner/repo")
	spec := &storage.Spec{Slug: ""}
	_, _, err := g.FindOrCreate(context.Background(), spec)
	if err == nil {
		t.Fatal("FindOrCreate() expected error for empty slug, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("FindOrCreate() error = %v, want errPushFailed", err)
	}
}

func TestGitHubAdapter_FindOrCreate_EmptyRepo(t *testing.T) {
	g := NewGitHubAdapter(&mockRunner{}, "")
	spec := &storage.Spec{Slug: "my-spec"}
	_, _, err := g.FindOrCreate(context.Background(), spec)
	if err == nil {
		t.Fatal("FindOrCreate() expected error for empty repo, got nil")
	}
	if !errors.Is(err, errPushFailed) {
		t.Errorf("FindOrCreate() error = %v, want errPushFailed", err)
	}
}

func TestGitHubAdapter_FindOrCreate_InvalidJSON(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: []byte(`not-json`), err: nil},
		},
	}
	g := NewGitHubAdapter(runner, "owner/repo")
	spec := &storage.Spec{
		Slug:   "my-spec",
		Intent: "Build a thing",
		Stage:  storage.SpecStageSpark,
	}
	_, _, err := g.FindOrCreate(context.Background(), spec)
	if err == nil {
		t.Fatal("FindOrCreate() expected error for invalid JSON, got nil")
	}
}

func TestGitHubAdapter_FindOrCreate_PushError(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: []byte(`[]`), err: nil},
			{output: nil, err: errors.New("gh issue create failed")},
		},
	}
	g := NewGitHubAdapter(runner, "owner/repo")
	spec := &storage.Spec{
		Slug:     "new-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP1,
	}
	_, _, err := g.FindOrCreate(context.Background(), spec)
	if err == nil {
		t.Fatal("FindOrCreate() expected error when push fails, got nil")
	}
}

func TestGitHubAdapter_FindOrCreate_SearchError(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: nil, err: errors.New("gh search failed")},
		},
	}
	g := NewGitHubAdapter(runner, "owner/repo")
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP1,
	}
	_, _, err := g.FindOrCreate(context.Background(), spec)
	if err == nil {
		t.Fatal("FindOrCreate() expected error, got nil")
	}
}
