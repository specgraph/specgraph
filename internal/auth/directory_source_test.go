// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestDirectoryPolicySource_ReadsCedarFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.cedar"),
		[]byte(`permit (principal, action, resource) when { principal has role && principal.role == "admin" };`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"),
		[]byte("ignored"), 0o600))

	docs, err := auth.NewDirectoryPolicySource(dir).Load(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 1, "only .cedar files are loaded")
	require.Equal(t, "dir:"+filepath.Join(dir, "extra.cedar"), docs[0].Source)
}

func TestDirectoryPolicySource_MissingDirIsError(t *testing.T) {
	_, err := auth.NewDirectoryPolicySource("/no/such/dir/specgraph").Load(context.Background())
	require.Error(t, err)
}

func TestDirectoryPolicySource_EmptyDirIsOK(t *testing.T) {
	docs, err := auth.NewDirectoryPolicySource(t.TempDir()).Load(context.Background())
	require.NoError(t, err)
	require.Empty(t, docs)
}

func TestDirectoryPolicySource_Name(t *testing.T) {
	require.Equal(t, "dir:/etc/x", auth.NewDirectoryPolicySource("/etc/x").Name())
}
