// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"testing"
)

func TestWholeFileStubReturnsNotImplemented(t *testing.T) {
	_, err := wholeFileStrategy{}.Sync("", ManagedFile{}, ProjectParams{}, SyncOptions{})
	if !errors.Is(err, errNotImplemented) {
		t.Errorf("WholeFile stub should still return errNotImplemented; got %v", err)
	}
}
