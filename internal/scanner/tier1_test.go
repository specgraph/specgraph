// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanb4t/specgraph/internal/scanner"
	"github.com/stretchr/testify/require"
)

func TestTier1Scan(t *testing.T) {
	dir := t.TempDir()

	// Create a Go file with an interface and a struct.
	authDir := filepath.Join(dir, "internal", "auth")
	require.NoError(t, os.MkdirAll(authDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(authDir, "handler.go"), []byte(`package auth

type Authenticator interface {
	Authenticate(token string) (bool, error)
	Revoke(token string) error
}

type UserInfo struct {
	ID    string
	Email string
	Role  string
}
`), 0o600))

	result, err := scanner.ScanTier1(dir)
	require.NoError(t, err)

	// Verify packages found.
	require.NotEmpty(t, result.Packages)
	var foundPkg bool
	for _, pkg := range result.Packages {
		if pkg.Name == "auth" {
			foundPkg = true
			require.Equal(t, filepath.Join("internal", "auth"), pkg.Path)
		}
	}
	require.True(t, foundPkg, "expected to find package 'auth'")

	// Verify interfaces found.
	require.NotEmpty(t, result.Interfaces)
	var foundIface bool
	for _, iface := range result.Interfaces {
		if iface.Name == "Authenticator" {
			foundIface = true
			require.Equal(t, "auth", iface.Package)
			require.ElementsMatch(t, []string{"Authenticate", "Revoke"}, iface.Methods)
		}
	}
	require.True(t, foundIface, "expected to find interface 'Authenticator'")

	// Verify structs found.
	require.NotEmpty(t, result.Structs)
	var foundStruct bool
	for _, s := range result.Structs {
		if s.Name == "UserInfo" {
			foundStruct = true
			require.Equal(t, "auth", s.Package)
			require.ElementsMatch(t, []string{"ID", "Email", "Role"}, s.Fields)
		}
	}
	require.True(t, foundStruct, "expected to find struct 'UserInfo'")
}

func TestTier1Scan_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()

	pkgDir := filepath.Join(dir, "pkg")
	require.NoError(t, os.MkdirAll(pkgDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "service.go"), []byte(`package pkg

type Service struct {
	Name string
}
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "service_test.go"), []byte(`package pkg

type TestHelper struct {
	MockName string
}
`), 0o600))

	result, err := scanner.ScanTier1(dir)
	require.NoError(t, err)

	// Should find Service but not TestHelper.
	names := make([]string, 0, len(result.Structs))
	for _, s := range result.Structs {
		names = append(names, s.Name)
	}
	require.Contains(t, names, "Service")
	require.NotContains(t, names, "TestHelper")
}

func TestTier1Scan_SkipsVendor(t *testing.T) {
	dir := t.TempDir()

	vendorDir := filepath.Join(dir, "vendor", "lib")
	require.NoError(t, os.MkdirAll(vendorDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte(`package lib

type VendoredType struct {
	X int
}
`), 0o600))

	result, err := scanner.ScanTier1(dir)
	require.NoError(t, err)
	require.Empty(t, result.Structs)
	require.Empty(t, result.Packages)
}
