// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import "testing"

func TestProviderIssuer(t *testing.T) {
	tests := []struct {
		name string
		pc   OIDCProviderConfig
		want string
	}{
		{
			name: "oidc kind uses configured issuer",
			pc:   OIDCProviderConfig{ID: "entra", Kind: "oidc", Issuer: "https://login.microsoftonline.com/tid/v2.0"},
			want: "https://login.microsoftonline.com/tid/v2.0",
		},
		{
			name: "empty kind defaults to oidc issuer",
			pc:   OIDCProviderConfig{ID: "okta", Issuer: "https://okta.example.com"},
			want: "https://okta.example.com",
		},
		{
			name: "oauth2 with explicit issuer uses it",
			pc:   OIDCProviderConfig{ID: "github", Kind: "oauth2", Issuer: "https://github.com"},
			want: "https://github.com",
		},
		{
			name: "oauth2 without issuer derives synthetic id from provider id",
			pc:   OIDCProviderConfig{ID: "github", Kind: "oauth2"},
			want: "oauth2:github",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProviderIssuer(tc.pc); got != tc.want {
				t.Fatalf("ProviderIssuer(%+v) = %q, want %q", tc.pc, got, tc.want)
			}
		})
	}
}

func TestValidateMCPResourceURI(t *testing.T) {
	valid := []string{
		"https://mcp.example.com/mcp",
		"https://host:8443/mcp",
	}
	for _, v := range valid {
		if err := ValidateMCPResourceURI(v); err != nil {
			t.Errorf("ValidateMCPResourceURI(%q) = %v, want nil", v, err)
		}
	}

	invalid := []string{
		"http://mcp.example.com/mcp",       // not https
		"https://mcp.example.com/mcp#frag", // fragment
		"https:///mcp",                     // missing host
		"mcp.example.com/mcp",              // no scheme
	}
	for _, v := range invalid {
		if err := ValidateMCPResourceURI(v); err == nil {
			t.Errorf("ValidateMCPResourceURI(%q) = nil, want error", v)
		}
	}
}
