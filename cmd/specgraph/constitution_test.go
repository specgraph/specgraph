// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstitutionLayerStringToProto(t *testing.T) {
	tests := []struct {
		input string
		want  specv1.ConstitutionLayer
	}{
		{"user", specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER},
		{"User", specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER},
		{"USER", specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER},
		{"org", specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
		{"project", specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
		{"domain", specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN},
		{"", specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED},
		{"unknown", specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := constitutionLayerStringToProto(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConstitutionRefTypeToProto(t *testing.T) {
	tests := []struct {
		input string
		want  specv1.ReferenceType
	}{
		{"adr", specv1.ReferenceType_REFERENCE_TYPE_ADR},
		{"ADR", specv1.ReferenceType_REFERENCE_TYPE_ADR},
		{"spec", specv1.ReferenceType_REFERENCE_TYPE_SPEC},
		{"doc", specv1.ReferenceType_REFERENCE_TYPE_DOC},
		{"url", specv1.ReferenceType_REFERENCE_TYPE_URL},
		{"", specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED},
		{"unknown", specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := constitutionRefTypeToProto(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConstitutionConfigToProto_BasicFields(t *testing.T) {
	cc := &config.ConstitutionConfig{
		Name:  "My Constitution",
		Layer: "project",
		Principles: []config.ConstitutionPrinciple{
			{ID: "p1", Statement: "Keep it simple", Rationale: "YAGNI"},
		},
		Constraints:  []string{"no vendor lock-in"},
		Antipatterns: []config.ConstitutionAntipattern{{Pattern: "god object", Why: "hard to test"}},
		References: []config.ConstitutionReference{
			{Type: "adr", Path: "docs/adr/001.md"},
		},
	}

	pb := constitutionConfigToProto(cc)

	assert.Equal(t, "My Constitution", pb.GetName())
	assert.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT, pb.GetLayer())
	assert.Equal(t, []string{"no vendor lock-in"}, pb.GetConstraints())

	require := assert.New(t)
	require.Len(pb.GetPrinciples(), 1)
	assert.Equal(t, "p1", pb.GetPrinciples()[0].GetId())
	assert.Equal(t, "Keep it simple", pb.GetPrinciples()[0].GetStatement())

	require.Len(pb.GetAntipatterns(), 1)
	assert.Equal(t, "god object", pb.GetAntipatterns()[0].GetPattern())

	require.Len(pb.GetReferences(), 1)
	assert.Equal(t, specv1.ReferenceType_REFERENCE_TYPE_ADR, pb.GetReferences()[0].GetReferenceType())
	assert.Equal(t, "docs/adr/001.md", pb.GetReferences()[0].GetPath())
}

func TestConstitutionConfigToProto_TechConfig(t *testing.T) {
	cc := &config.ConstitutionConfig{
		Name:  "Tech Constitution",
		Layer: "domain",
		Tech: config.ConstitutionTech{
			Languages: config.ConstitutionLangs{
				Primary:   "go",
				Allowed:   []string{"go", "python"},
				Forbidden: []string{"php"},
			},
		},
	}

	pb := constitutionConfigToProto(cc)
	tech := pb.GetTech()
	if tech == nil {
		t.Fatal("expected Tech to be non-nil")
	}
	langs := tech.GetLanguages()
	if langs == nil {
		t.Fatal("expected Languages to be non-nil")
	}
	assert.Equal(t, "go", langs.GetPrimary())
	assert.Equal(t, []string{"go", "python"}, langs.GetAllowed())
	assert.Equal(t, []string{"php"}, langs.GetForbidden())
}

func TestConstitutionConfigToProto_EmptyTech(t *testing.T) {
	cc := &config.ConstitutionConfig{
		Name:  "No Tech",
		Layer: "org",
	}

	pb := constitutionConfigToProto(cc)
	assert.Nil(t, pb.GetTech(), "expected Tech to be nil when no tech fields set")
}

// --- fake server helper ---

func startFakeConstitutionServer(t *testing.T, h specgraphv1connect.ConstitutionServiceHandler) {
	t.Helper()
	startFakeServer[specgraphv1connect.ConstitutionServiceHandler](t, h, specgraphv1connect.NewConstitutionServiceHandler)
}

// --- fake handlers ---

type fakeConstitutionShowHandler struct {
	specgraphv1connect.UnimplementedConstitutionServiceHandler
}

func (fakeConstitutionShowHandler) GetConstitution(_ context.Context, _ *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.GetConstitutionResponse], error) {
	return connect.NewResponse(&specv1.GetConstitutionResponse{
		Constitution: &specv1.Constitution{Name: "test-project"},
	}), nil
}

type fakeConstitutionShowErrorHandler struct {
	specgraphv1connect.UnimplementedConstitutionServiceHandler
}

func (fakeConstitutionShowErrorHandler) GetConstitution(_ context.Context, _ *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.GetConstitutionResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("constitution not found"))
}

type fakeConstitutionEmitHandler struct {
	specgraphv1connect.UnimplementedConstitutionServiceHandler
}

func (fakeConstitutionEmitHandler) EmitToolFiles(_ context.Context, _ *connect.Request[specv1.EmitToolFilesRequest]) (*connect.Response[specv1.EmitToolFilesResponse], error) {
	return connect.NewResponse(&specv1.EmitToolFilesResponse{
		Content:  "# Constitution\ntest content",
		Filename: "CLAUDE.md",
	}), nil
}

type fakeConstitutionEmitErrorHandler struct {
	specgraphv1connect.UnimplementedConstitutionServiceHandler
}

func (fakeConstitutionEmitErrorHandler) EmitToolFiles(_ context.Context, _ *connect.Request[specv1.EmitToolFilesRequest]) (*connect.Response[specv1.EmitToolFilesResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("emit failed"))
}

// --- constitution show tests ---

func TestRunConstitutionShow_HappyPath(t *testing.T) {
	startFakeConstitutionServer(t, fakeConstitutionShowHandler{})

	oldJSON := constitutionShowJSON
	constitutionShowJSON = false
	t.Cleanup(func() { constitutionShowJSON = oldJSON })

	err := runConstitutionShow(&cobra.Command{}, nil)
	require.NoError(t, err)
}

func TestRunConstitutionShow_HappyPath_JSON(t *testing.T) {
	startFakeConstitutionServer(t, fakeConstitutionShowHandler{})

	oldJSON := constitutionShowJSON
	constitutionShowJSON = true
	t.Cleanup(func() { constitutionShowJSON = oldJSON })

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runConstitutionShow(cmd, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "test-project")
}

func TestRunConstitutionShow_RPCError(t *testing.T) {
	startFakeConstitutionServer(t, fakeConstitutionShowErrorHandler{})

	oldJSON := constitutionShowJSON
	constitutionShowJSON = false
	t.Cleanup(func() { constitutionShowJSON = oldJSON })

	err := runConstitutionShow(&cobra.Command{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get constitution")
}

func TestRunConstitutionShow_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runConstitutionShow(&cobra.Command{}, nil)
	require.Error(t, err)
}

// --- constitution emit tests ---

func TestRunConstitutionEmit_HappyPath_Stdout(t *testing.T) {
	startFakeConstitutionServer(t, fakeConstitutionEmitHandler{})

	oldFmt := emitFormat
	oldOut := emitOutput
	emitFormat = "claude-md"
	emitOutput = ""
	t.Cleanup(func() { emitFormat = oldFmt; emitOutput = oldOut })

	err := runConstitutionEmit(nil, nil)
	require.NoError(t, err)
}

func TestRunConstitutionEmit_HappyPath_File(t *testing.T) {
	startFakeConstitutionServer(t, fakeConstitutionEmitHandler{})

	// t.Chdir changes cwd for this test only and restores on cleanup.
	t.Chdir(t.TempDir())

	oldFmt := emitFormat
	oldOut := emitOutput
	emitFormat = "claude-md"
	emitOutput = "out.md"
	t.Cleanup(func() { emitFormat = oldFmt; emitOutput = oldOut })

	err := runConstitutionEmit(nil, nil)
	require.NoError(t, err)

	data, err := os.ReadFile("out.md")
	require.NoError(t, err)
	assert.Contains(t, string(data), "# Constitution")
	assert.Contains(t, string(data), "test content")
}

func TestRunConstitutionEmit_PathTraversal(t *testing.T) {
	startFakeConstitutionServer(t, fakeConstitutionEmitHandler{})

	t.Chdir(t.TempDir())

	oldFmt := emitFormat
	oldOut := emitOutput
	emitFormat = "claude-md"
	emitOutput = "../outside-cwd.md"
	t.Cleanup(func() { emitFormat = oldFmt; emitOutput = oldOut })

	err := runConstitutionEmit(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside current directory")
}

func TestRunConstitutionEmit_InvalidFormat(t *testing.T) {
	oldFmt := emitFormat
	oldOut := emitOutput
	emitFormat = "bogus"
	emitOutput = ""
	t.Cleanup(func() { emitFormat = oldFmt; emitOutput = oldOut })

	err := runConstitutionEmit(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestRunConstitutionEmit_RPCError(t *testing.T) {
	startFakeConstitutionServer(t, fakeConstitutionEmitErrorHandler{})

	oldFmt := emitFormat
	oldOut := emitOutput
	emitFormat = "claude-md"
	emitOutput = ""
	t.Cleanup(func() { emitFormat = oldFmt; emitOutput = oldOut })

	err := runConstitutionEmit(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "emit tool files")
}

func TestRunConstitutionEmit_ClientError(t *testing.T) {
	setMissingConfig(t)

	oldFmt := emitFormat
	oldOut := emitOutput
	emitFormat = "claude-md"
	emitOutput = ""
	t.Cleanup(func() { emitFormat = oldFmt; emitOutput = oldOut })

	err := runConstitutionEmit(nil, nil)
	require.Error(t, err)
}

// --- outputFormatMap tests ---

func TestOutputFormatMap_Completeness(t *testing.T) {
	expected := []string{"claude-md", "cursorrules", "agents-md"}
	for _, key := range expected {
		_, ok := outputFormatMap[key]
		assert.True(t, ok, "expected key %q in outputFormatMap", key)
	}
	assert.Len(t, outputFormatMap, len(expected), "outputFormatMap has unexpected entries")
}
