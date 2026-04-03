// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// analyticalPassTestBackend embeds stubBackend and overrides GetSpec,
// StoreFindings, and ListFindings with in-memory implementations.
type analyticalPassTestBackend struct {
	stubBackend
	mu       sync.Mutex
	specs    map[string]*storage.Spec
	findings map[string][]storage.AnalyticalFinding // key: "slug:passType" or "slug:" for all
	nextID   int
}

func newAnalyticalPassTestBackend() *analyticalPassTestBackend {
	return &analyticalPassTestBackend{
		specs:    make(map[string]*storage.Spec),
		findings: make(map[string][]storage.AnalyticalFinding),
	}
}

func (b *analyticalPassTestBackend) CreateSpec(_ context.Context, slug, intent, priority, complexity string) (*storage.Spec, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	spec := &storage.Spec{
		Slug:        slug,
		Intent:      intent,
		Stage:       storage.SpecStageSpark,
		Priority:    storage.SpecPriority(priority),
		Complexity:  storage.SpecComplexity(complexity),
		Version:     1,
		ContentHash: strings.Repeat("a", 32),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	b.specs[slug] = spec
	return spec, nil
}

func (b *analyticalPassTestBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	spec, ok := b.specs[slug]
	if !ok {
		return nil, storage.ErrSpecNotFound
	}
	return spec, nil
}

func (b *analyticalPassTestBackend) StoreFindings(_ context.Context, slug string, passType storage.PassType, findings []storage.AnalyticalFindingInput) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.specs[slug]; !ok {
		return nil, storage.ErrSpecNotFound
	}
	key := fmt.Sprintf("%s:%s", slug, passType)
	stored := make([]storage.AnalyticalFinding, len(findings))
	ids := make([]string, len(findings))
	for i, f := range findings {
		b.nextID++
		id := fmt.Sprintf("finding-%d", b.nextID)
		stored[i] = storage.AnalyticalFinding{
			ID:         id,
			PassType:   passType,
			Severity:   f.Severity,
			Summary:    f.Summary,
			Detail:     f.Detail,
			Constraint: f.Constraint,
			Resolution: f.Resolution,
			Version:    b.specs[slug].Version,
			CreatedAt:  time.Now(),
		}
		ids[i] = id
	}
	b.findings[key] = stored
	return ids, nil
}

func (b *analyticalPassTestBackend) ListFindings(_ context.Context, slug string, passType storage.PassType) ([]storage.AnalyticalFinding, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.specs[slug]; !ok {
		return nil, storage.ErrSpecNotFound
	}
	if passType == "" {
		// Return all findings for the slug across all pass types.
		var all []storage.AnalyticalFinding
		prefix := slug + ":"
		for k, v := range b.findings {
			if strings.HasPrefix(k, prefix) {
				all = append(all, v...)
			}
		}
		return all, nil
	}
	key := fmt.Sprintf("%s:%s", slug, passType)
	return b.findings[key], nil
}

func setupAnalyticalPassServer(t *testing.T, backend storage.ScopedBackend) specgraphv1connect.AnalyticalPassServiceClient {
	t.Helper()
	scoper := &testScoper{backend: backend}
	mux := http.NewServeMux()
	server.RegisterAnalyticalPassService(mux, scoper, "")
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewAnalyticalPassServiceClient(http.DefaultClient, srv.URL)
}

func TestRunAnalyticalPass_ReturnsPromptAndTools(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServer(t, backend)

	resp, err := client.RunAnalyticalPass(context.Background(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
	}))
	require.NoError(t, err)

	msg := resp.Msg
	require.Contains(t, msg.PromptTemplate, "Constitution Compliance Reviewer")
	require.NotEmpty(t, msg.Tools)
	for _, tool := range msg.Tools {
		assert.NotContains(t, tool.Command, "--json",
			"tool %q command should not include --json (LLM subagents read markdown)", tool.Name)
	}
	require.Contains(t, msg.InitialMessage, "my-spec")
	require.Equal(t, "spark", msg.Stage)
	require.Equal(t, specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK, msg.PassType)
}

func TestRunAnalyticalPass_UnknownPassType(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServer(t, backend)

	_, err = client.RunAnalyticalPass(context.Background(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType(999),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRunAnalyticalPass_SpecNotFound(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	client := setupAnalyticalPassServer(t, backend)

	_, err := client.RunAnalyticalPass(context.Background(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     "does-not-exist",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestStoreAndListFindings_RoundTrip(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServer(t, backend)

	storeResp, err := client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
		Findings: []*specv1.AnalyticalFindingInput{
			{
				Severity:   specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
				Summary:    "Missing constraint coverage",
				Detail:     "The spec does not address constraint X",
				Constraint: "constitution.layer.constraint-x",
				Resolution: "Add a section covering constraint X",
			},
		},
	}))
	require.NoError(t, err)
	require.Len(t, storeResp.Msg.Ids, 1)
	require.NotEmpty(t, storeResp.Msg.Ids[0])

	listResp, err := client.ListFindings(context.Background(), connect.NewRequest(&specv1.ListFindingsRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Findings, 1)

	f := listResp.Msg.Findings[0]
	require.NotEmpty(t, f.Id)
	require.Equal(t, specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK, f.PassType)
	require.Equal(t, specv1.FindingSeverity_FINDING_SEVERITY_WARNING, f.Severity)
	require.Equal(t, "Missing constraint coverage", f.Summary)
	require.Equal(t, "The spec does not address constraint X", f.Detail)
	require.Equal(t, "constitution.layer.constraint-x", f.Constraint)
	require.Equal(t, "Add a section covering constraint X", f.Resolution)
}

func TestListFindings_EmptyPassType(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServer(t, backend)

	// Store findings for two different pass types.
	_, err = client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
		Findings: []*specv1.AnalyticalFindingInput{
			{
				Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
				Summary:  "Constitution finding",
			},
		},
	}))
	require.NoError(t, err)

	_, err = client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType_PASS_TYPE_RED_TEAM,
		Findings: []*specv1.AnalyticalFindingInput{
			{
				Severity: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
				Summary:  "Red team finding",
			},
		},
	}))
	require.NoError(t, err)

	// List with UNSPECIFIED pass_type — should return all findings.
	listResp, err := client.ListFindings(context.Background(), connect.NewRequest(&specv1.ListFindingsRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Findings, 2)
}

func TestStoreFindings_UnknownPassType(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServer(t, backend)

	_, err = client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType(999),
		Findings: []*specv1.AnalyticalFindingInput{
			{
				Severity: specv1.FindingSeverity_FINDING_SEVERITY_NOTE,
				Summary:  "Should not be stored",
			},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestStoreFindings_SpecNotFound(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	client := setupAnalyticalPassServer(t, backend)

	_, err := client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "does-not-exist",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
		Findings: []*specv1.AnalyticalFindingInput{
			{
				Severity: specv1.FindingSeverity_FINDING_SEVERITY_NOTE,
				Summary:  "Should not be stored",
			},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestStoreFindings_NullFindingElement(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServer(t, backend)

	_, err = client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
		Findings: []*specv1.AnalyticalFindingInput{nil},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestStoreFindings_TooManyFindings(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServer(t, backend)

	findings := make([]*specv1.AnalyticalFindingInput, 101)
	for i := range findings {
		findings[i] = &specv1.AnalyticalFindingInput{
			Severity: specv1.FindingSeverity_FINDING_SEVERITY_NOTE,
			Summary:  fmt.Sprintf("finding %d", i),
		}
	}

	_, err = client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
		Findings: findings,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRunAnalyticalPass_EmptySlug(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	client := setupAnalyticalPassServer(t, backend)

	_, err := client.RunAnalyticalPass(context.Background(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     "",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestListFindings_EmptySlug(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	client := setupAnalyticalPassServer(t, backend)

	_, err := client.ListFindings(context.Background(), connect.NewRequest(&specv1.ListFindingsRequest{
		Slug:     "",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestListFindings_UnknownPassType(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServer(t, backend)

	_, err = client.ListFindings(context.Background(), connect.NewRequest(&specv1.ListFindingsRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType(999),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func setupAnalyticalPassServerWithOverrideDir(t *testing.T, backend storage.ScopedBackend, overrideDir string) specgraphv1connect.AnalyticalPassServiceClient {
	t.Helper()
	scoper := &testScoper{backend: backend}
	mux := http.NewServeMux()
	server.RegisterAnalyticalPassService(mux, scoper, overrideDir)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewAnalyticalPassServiceClient(http.DefaultClient, srv.URL)
}

func TestRunAnalyticalPass_TemplateOverride(t *testing.T) {
	overrideDir := t.TempDir()
	overrideContent := "# Custom Constitution Check\n\nThis is a local override."
	err := os.WriteFile(filepath.Join(overrideDir, "constitution_check.md"), []byte(overrideContent), 0o644) //nolint:gosec // test file
	require.NoError(t, err)

	backend := newAnalyticalPassTestBackend()
	_, err = backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServerWithOverrideDir(t, backend, overrideDir)

	resp, err := client.RunAnalyticalPass(context.Background(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
	}))
	require.NoError(t, err)
	require.Equal(t, overrideContent, resp.Msg.PromptTemplate)
}

func TestRunAnalyticalPass_TemplateOverrideFallback(t *testing.T) {
	// Override dir exists but has no file for red_team — should fall back to embedded.
	overrideDir := t.TempDir()

	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "my-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)

	client := setupAnalyticalPassServerWithOverrideDir(t, backend, overrideDir)

	resp, err := client.RunAnalyticalPass(context.Background(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     "my-spec",
		PassType: specv1.PassType_PASS_TYPE_RED_TEAM,
	}))
	require.NoError(t, err)
	require.Contains(t, resp.Msg.PromptTemplate, "Red Team")
}
