// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package export

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
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
