// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr string // substring; empty means no error
	}{
		{name: "valid simple", slug: "my-spec"},
		{name: "valid with numbers", slug: "spec-v2"},
		{name: "valid with underscore", slug: "my_spec"},
		{name: "valid with slash", slug: "org/my-spec"},
		{name: "valid single char", slug: "a"},
		{name: "empty", slug: "", wantErr: "slug is required"},
		{name: "spaces", slug: "my spec", wantErr: "invalid characters"},
		{name: "uppercase", slug: "MySpec", wantErr: "invalid characters"},
		{name: "special chars", slug: "spec@v1", wantErr: "invalid characters"},
		{name: "path traversal", slug: "foo/../bar", wantErr: "path traversal"},
		{name: "starts with hyphen", slug: "-spec", wantErr: "invalid characters"},
		{name: "ends with hyphen", slug: "spec-", wantErr: "invalid characters"},
		{name: "at max length", slug: strings.Repeat("a", maxSlugLength)},
		{name: "exceeds max length", slug: strings.Repeat("a", maxSlugLength+1), wantErr: "exceeds maximum length"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSlug(tt.slug)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateRequiredField(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		wantErr string
	}{
		{name: "valid", field: "reason", value: "some reason"},
		{name: "empty rejected", field: "reason", value: "", wantErr: "reason is required"},
		{name: "at max length", field: "desc", value: strings.Repeat("x", maxFieldLen)},
		{name: "exceeds max", field: "desc", value: strings.Repeat("x", maxFieldLen+1), wantErr: "exceeds maximum length"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequiredField(tt.field, tt.value)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
