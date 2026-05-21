// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package export

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Schema validation
// ---------------------------------------------------------------------------

func TestImport_RejectsUnsupportedSchemaVersion(t *testing.T) {
	doc := Document{SchemaVersion: 999}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	eng := NewEngine(nil, "", "test")
	_, err = eng.Import(t.Context(), data, false, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported schema version") {
		t.Fatalf("expected 'unsupported schema version' in error, got: %v", err)
	}
}

// TestImport_RejectsSchemaVersionZero guards against silent data-loss on
// unversioned/hand-crafted documents. SchemaVersion: 0 (e.g. omitted field)
// passes the high-version check but must be rejected at the constitution
// switch's default branch so any constitution data isn't silently dropped.
func TestImport_RejectsSchemaVersionZero(t *testing.T) {
	backend := newTestBackend(t)
	doc := Document{
		SchemaVersion: 0,
		ProjectSlug:   "test-project",
		Data: Data{
			Project: &storage.Project{Slug: "test-project"},
			Constitution: &storage.Constitution{
				Layer: storage.ConstitutionLayerProject,
			},
		},
	}
	data, err := json.Marshal(doc)
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	_, err = engine.Import(context.Background(), data, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported schema version 0")
}

// ---------------------------------------------------------------------------
// HMAC signature — tested via verifySignature directly to avoid nil backend
// ---------------------------------------------------------------------------

func TestImport_ValidSignature(t *testing.T) {
	const key = "test-secret"
	doc := Document{SchemaVersion: CurrentSchemaVersion}

	dataBytes, err := json.Marshal(doc.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(dataBytes)
	doc.Signature = &Signature{
		Algorithm: "hmac-sha256",
		Digest:    hex.EncodeToString(mac.Sum(nil)),
	}

	// Marshal the full document to get raw bytes for verifySignature.
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}

	eng := &Engine{signingKey: key}
	if err := eng.verifySignature(raw, &doc, false); err != nil {
		t.Fatalf("expected no error for valid signature, got: %v", err)
	}
}

func TestImport_TamperedData(t *testing.T) {
	const key = "test-secret"
	doc := Document{SchemaVersion: CurrentSchemaVersion}

	dataBytes, err := json.Marshal(doc.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(dataBytes)
	doc.Signature = &Signature{
		Algorithm: "hmac-sha256",
		Digest:    hex.EncodeToString(mac.Sum(nil)),
	}

	// Marshal with original data + valid signature, then tamper the raw bytes.
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}
	// Tamper: modify a byte in the data section of the raw JSON.
	tampered := make([]byte, len(raw))
	copy(tampered, raw)
	// Find "data" and change a byte after it.
	idx := bytes.Index(tampered, []byte(`"data"`))
	if idx > 0 && idx+10 < len(tampered) {
		tampered[idx+10] ^= 0xFF
	}

	eng := &Engine{signingKey: key}
	err = eng.verifySignature(tampered, &doc, false)
	if err == nil {
		t.Fatal("expected HMAC mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "HMAC") && !strings.Contains(err.Error(), "extract data") {
		t.Fatalf("expected 'HMAC' in error, got: %v", err)
	}
}

func TestImport_MissingKeyWithSignature(t *testing.T) {
	// Engine has no signing key, document has a signature → can't verify, should proceed.
	doc := Document{
		SchemaVersion: CurrentSchemaVersion,
		Signature: &Signature{
			Algorithm: "hmac-sha256",
			Digest:    "deadbeef",
		},
	}

	eng := &Engine{signingKey: ""}
	if err := eng.verifySignature(nil, &doc, false); err != nil {
		t.Fatalf("expected no error when signing key absent, got: %v", err)
	}
}

func TestImport_RequireSignatureNoSignature(t *testing.T) {
	doc := Document{SchemaVersion: CurrentSchemaVersion}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	eng := NewEngine(nil, "", "test")
	_, err = eng.Import(t.Context(), raw, false, true) // requireSig=true
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsigned export") && !strings.Contains(err.Error(), "signature required") {
		t.Fatalf("expected 'unsigned export' or 'signature required' in error, got: %v", err)
	}
}

func TestImport_NoSignatureNoFlag(t *testing.T) {
	eng := &Engine{signingKey: ""}
	doc := &Document{SchemaVersion: CurrentSchemaVersion}
	if err := eng.verifySignature(nil, doc, false); err != nil {
		t.Fatalf("expected no error for unsigned doc with requireSig=false, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Referential integrity
// ---------------------------------------------------------------------------

func TestValidateRefs_BrokenEdge(t *testing.T) {
	doc := &Document{
		Data: Data{
			Specs: []*storage.Spec{{Slug: "spec-a"}},
			Edges: []Edge{
				{FromSlug: "spec-a", ToSlug: "nonexistent", Type: "DEPENDS_ON"},
			},
		},
	}

	err := validateRefs(doc)
	if err == nil {
		t.Fatal("expected error for broken edge, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("expected broken ref 'nonexistent' in error, got: %v", err)
	}
}

func TestValidateRefs_BrokenSlice(t *testing.T) {
	doc := &Document{
		Data: Data{
			Specs: []*storage.Spec{{Slug: "spec-a"}},
			Slices: []*storage.Slice{
				{Slug: "missing-parent/s1", ParentSlug: "missing-parent"},
			},
		},
	}

	err := validateRefs(doc)
	if err == nil {
		t.Fatal("expected error for broken slice parent, got nil")
	}
	if !strings.Contains(err.Error(), "missing-parent") {
		t.Fatalf("expected 'missing-parent' in error, got: %v", err)
	}
}

func TestValidateRefs_BrokenFinding(t *testing.T) {
	doc := &Document{
		Data: Data{
			Specs: []*storage.Spec{{Slug: "spec-a"}},
			Findings: []*storage.AnalyticalFinding{
				{SpecSlug: "ghost-spec"},
			},
		},
	}

	err := validateRefs(doc)
	if err == nil {
		t.Fatal("expected error for broken finding spec_slug, got nil")
	}
	if !strings.Contains(err.Error(), "ghost-spec") {
		t.Fatalf("expected 'ghost-spec' in error, got: %v", err)
	}
}

func TestValidateRefs_AllValid(t *testing.T) {
	doc := &Document{
		Data: Data{
			Specs: []*storage.Spec{
				{Slug: "spec-a"},
				{Slug: "spec-b"},
			},
			Edges: []Edge{
				{FromSlug: "spec-a", ToSlug: "spec-b", Type: "DEPENDS_ON"},
			},
			Slices: []*storage.Slice{
				{Slug: "spec-a/s1", ParentSlug: "spec-a"},
			},
			Findings: []*storage.AnalyticalFinding{
				{SpecSlug: "spec-b"},
			},
		},
	}

	if err := validateRefs(doc); err != nil {
		t.Fatalf("expected no error for valid refs, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Import — ADR-003 decision fields pass-through
// ---------------------------------------------------------------------------

// decisionCapturingBackend captures CreateDecision calls during import.
// It embeds the Backend interface so unimplemented methods panic (acceptable
// for this test since only ListSpecs and CreateDecision are exercised).
type decisionCapturingBackend struct {
	Backend
	captured []*storage.Decision
}

func (b *decisionCapturingBackend) ListSpecs(_ context.Context, _, _ string, _ int) ([]*storage.Spec, error) {
	return nil, nil
}

func (b *decisionCapturingBackend) CreateDecision(_ context.Context, slug, title, body, rationale, question string,
	rejectedAlts []storage.RejectedAlternative, confidence storage.DecisionConfidence,
	tags []string, scope storage.DecisionScope, originSpec, originStage string,
) (*storage.Decision, error) {
	dec := &storage.Decision{
		Slug:                 slug,
		Title:                title,
		Body:                 body,
		Rationale:            rationale,
		Question:             question,
		RejectedAlternatives: rejectedAlts,
		Confidence:           confidence,
		Tags:                 tags,
		Scope:                scope,
		OriginSpec:           originSpec,
		OriginStage:          originStage,
		Version:              1,
	}
	b.captured = append(b.captured, dec)
	return dec, nil
}

// ---------------------------------------------------------------------------
// Multi-layer constitution export (v2 schema)
// ---------------------------------------------------------------------------

// multiLayerExportBackend is a minimal in-memory backend for
// TestExport_MultiLayerConstitution. It stubs every method called by
// Engine.collect, returning empty data for everything except GetProject and
// the constitution methods.
type multiLayerExportBackend struct {
	Backend
	project    *storage.Project
	layers     map[storage.ConstitutionLayer]*storage.Constitution
	layerOrder []storage.ConstitutionLayer // insertion order for GetAllLayers
}

func newTestBackend(t *testing.T) *multiLayerExportBackend {
	t.Helper()
	return &multiLayerExportBackend{
		project: &storage.Project{Slug: "test-project"},
		layers:  make(map[storage.ConstitutionLayer]*storage.Constitution),
	}
}

// UpdateConstitution stores a constitution layer in order of first insertion.
func (b *multiLayerExportBackend) UpdateConstitution(_ context.Context, c *storage.Constitution) (*storage.Constitution, error) {
	if _, exists := b.layers[c.Layer]; !exists {
		b.layerOrder = append(b.layerOrder, c.Layer)
	}
	clone := *c
	clone.Version = 1
	b.layers[c.Layer] = &clone
	return &clone, nil
}

func (b *multiLayerExportBackend) GetAllLayers(_ context.Context) ([]*storage.Constitution, error) {
	out := make([]*storage.Constitution, 0, len(b.layerOrder))
	for _, l := range b.layerOrder {
		out = append(out, b.layers[l])
	}
	return out, nil
}

func (b *multiLayerExportBackend) GetProject(_ context.Context, _ string) (*storage.Project, error) {
	return b.project, nil
}

func (b *multiLayerExportBackend) ListSpecs(_ context.Context, _, _ string, _ int) ([]*storage.Spec, error) {
	return nil, nil
}

func (b *multiLayerExportBackend) ListDecisions(_ context.Context, _ storage.DecisionStatus, _ int) ([]*storage.Decision, error) {
	return nil, nil
}

func (b *multiLayerExportBackend) GetFullGraph(_ context.Context) (*storage.FullGraph, error) {
	return &storage.FullGraph{}, nil
}

func (b *multiLayerExportBackend) ListAllFindings(_ context.Context) ([]*storage.AnalyticalFinding, error) {
	return nil, nil
}

func (b *multiLayerExportBackend) ListAllChanges(_ context.Context) ([]*storage.ChangeLogEntry, error) {
	return nil, nil
}

func (b *multiLayerExportBackend) ListAllConversations(_ context.Context) ([]*storage.ConversationLogEntry, error) {
	return nil, nil
}

func (b *multiLayerExportBackend) ListSyncMappings(_ context.Context, _ storage.SyncAdapterType, _ string) ([]*storage.SyncMapping, error) {
	return nil, nil
}

func (b *multiLayerExportBackend) EnsureProject(_ context.Context, slug string) (*storage.Project, error) {
	if b.project == nil {
		b.project = &storage.Project{Slug: slug}
	}
	return b.project, nil
}

// ---------------------------------------------------------------------------
// Import — schema-version-aware constitution import (Task 7)
// ---------------------------------------------------------------------------

func TestImport_V1Document_SingleLayer(t *testing.T) {
	backend := newTestBackend(t)
	ctx := context.Background()

	// Hand-crafted v1 document with the legacy singular field.
	v1Doc := Document{
		SchemaVersion:    1,
		ProjectSlug:      "test-project",
		SpecGraphVersion: "test-version",
		Data: Data{
			Project: &storage.Project{Slug: "test-project"},
			Constitution: &storage.Constitution{
				Name:  "v1-only",
				Layer: storage.ConstitutionLayerProject,
				Principles: []storage.Principle{{ID: "p1", Statement: "P1"}},
			},
		},
	}
	data, err := json.Marshal(v1Doc)
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	result, err := engine.Import(ctx, data, false, false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Constitution, "exactly one layer imported")

	layers, err := backend.GetAllLayers(ctx)
	require.NoError(t, err)
	require.Len(t, layers, 1)
	assert.Equal(t, "v1-only", layers[0].Name)
	assert.Equal(t, storage.ConstitutionLayerProject, layers[0].Layer)
}

func TestImport_V2Document_MultipleLayers(t *testing.T) {
	backend := newTestBackend(t)
	ctx := context.Background()

	v2Doc := Document{
		SchemaVersion:    2,
		ProjectSlug:      "test-project",
		SpecGraphVersion: "test-version",
		Data: Data{
			Project: &storage.Project{Slug: "test-project"},
			Constitutions: []*storage.Constitution{
				{Name: "org", Layer: storage.ConstitutionLayerOrg, Principles: []storage.Principle{{ID: "po"}}},
				{Name: "proj", Layer: storage.ConstitutionLayerProject, Principles: []storage.Principle{{ID: "pp"}}},
			},
		},
	}
	data, err := json.Marshal(v2Doc)
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	result, err := engine.Import(ctx, data, false, false)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Constitution, "both layers imported")

	layers, err := backend.GetAllLayers(ctx)
	require.NoError(t, err)
	assert.Len(t, layers, 2)
}

func TestImport_V1Document_WithV2Field_Rejected(t *testing.T) {
	backend := newTestBackend(t)

	mismatched := Document{
		SchemaVersion: 1,
		ProjectSlug:   "test-project",
		Data: Data{
			Project: &storage.Project{Slug: "test-project"},
			Constitutions: []*storage.Constitution{
				{Layer: storage.ConstitutionLayerProject},
			},
		},
	}
	data, err := json.Marshal(mismatched)
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	_, err = engine.Import(context.Background(), data, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v1 documents must use 'constitution' field, not 'constitutions'")
}

func TestImport_V2Document_WithV1Field_Rejected(t *testing.T) {
	backend := newTestBackend(t)

	mismatched := Document{
		SchemaVersion: 2,
		ProjectSlug:   "test-project",
		Data: Data{
			Project: &storage.Project{Slug: "test-project"},
			Constitution: &storage.Constitution{
				Layer: storage.ConstitutionLayerProject,
			},
		},
	}
	data, err := json.Marshal(mismatched)
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	_, err = engine.Import(context.Background(), data, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v2 documents must use 'constitutions' field, not 'constitution'")
}

func TestExport_MultiLayerConstitution(t *testing.T) {
	backend := newTestBackend(t)
	ctx := context.Background()

	// Seed two layers.
	_, err := backend.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "org",
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{{ID: "p-org", Statement: "Org"}},
	})
	require.NoError(t, err)

	_, err = backend.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "project",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{{ID: "p-proj", Statement: "Proj"}},
	})
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	out, err := engine.Export(ctx, "test-project")
	require.NoError(t, err)

	var doc Document
	require.NoError(t, json.Unmarshal(out, &doc))

	assert.Equal(t, 2, doc.SchemaVersion)
	assert.Nil(t, doc.Data.Constitution, "v2 exports never populate the v1 field")
	require.Len(t, doc.Data.Constitutions, 2, "v2 export must contain both layers")
	assert.Equal(t, storage.ConstitutionLayerOrg, doc.Data.Constitutions[0].Layer,
		"layers in precedence order: org before project")
	assert.Equal(t, storage.ConstitutionLayerProject, doc.Data.Constitutions[1].Layer)
}

func TestImport_DecisionADR003Fields(t *testing.T) {
	back := &decisionCapturingBackend{}

	doc := Document{
		SchemaVersion: CurrentSchemaVersion,
		Data: Data{
			Decisions: []*storage.Decision{
				{
					Slug:      "use-postgres",
					Title:     "Use Postgres for tokens",
					Body:      "We will use Postgres",
					Rationale: "Mature, reliable, well-known",
					Question:  "Where should we store auth tokens?",
					RejectedAlternatives: []storage.RejectedAlternative{
						{Option: "Redis", Reason: "operational complexity"},
						{Option: "DynamoDB", Reason: "vendor lock-in"},
					},
					Confidence:  storage.DecisionConfidenceHigh,
					Tags:        []string{"auth", "storage", "backend"},
					Scope:       storage.DecisionScopeProject,
					OriginSpec:  "login-api",
					OriginStage: "specify",
				},
			},
		},
	}

	data, err := json.Marshal(doc)
	require.NoError(t, err)

	eng := NewEngine(back, "", "test")
	res, err := eng.Import(t.Context(), data, false, false)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Decisions)

	require.Len(t, back.captured, 1)
	got := back.captured[0]
	assert.Equal(t, "use-postgres", got.Slug)
	assert.Equal(t, "Use Postgres for tokens", got.Title)
	assert.Equal(t, "We will use Postgres", got.Body)
	assert.Equal(t, "Mature, reliable, well-known", got.Rationale)
	assert.Equal(t, "Where should we store auth tokens?", got.Question)
	assert.Equal(t, []storage.RejectedAlternative{
		{Option: "Redis", Reason: "operational complexity"},
		{Option: "DynamoDB", Reason: "vendor lock-in"},
	}, got.RejectedAlternatives)
	assert.Equal(t, storage.DecisionConfidenceHigh, got.Confidence)
	assert.Equal(t, []string{"auth", "storage", "backend"}, got.Tags)
	assert.Equal(t, storage.DecisionScopeProject, got.Scope)
	assert.Equal(t, "login-api", got.OriginSpec)
	assert.Equal(t, "specify", got.OriginStage)
}
