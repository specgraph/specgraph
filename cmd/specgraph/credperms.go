// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"io"

	"github.com/specgraph/specgraph/internal/credentials"
)

// warnLooseCredentialFile emits a one-line warning to w when the credentials
// file at path is readable or writable by group or others. It is silent when
// permissions are acceptable or the file does not exist. Wired into
// `specgraph doctor` so the check runs once on demand rather than on every
// authenticated CLI call.
func warnLooseCredentialFile(w io.Writer, path string) {
	if msg := credentials.CheckPermissions(path); msg != "" {
		_, _ = fmt.Fprintln(w, msg) //nolint:errcheck // writes to user stream; an advisory warning must never fail the command
	}
}
