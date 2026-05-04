// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// confirmDestructive returns nil only when the caller has consent to proceed
// with a destructive operation. --yes bypasses the prompt; on a TTY the user
// must type y/yes; off a TTY without --yes, the command errors rather than
// silently destroying data.
func confirmDestructive(in io.Reader, out io.Writer, isTTY, yes bool, prompt string) error {
	if yes {
		return nil
	}
	if !isTTY {
		return errors.New("destructive operation requires --yes when not on a TTY")
	}
	_, _ = fmt.Fprintf(out, "%s [y/N] ", prompt) //nolint:errcheck // writes to user stream
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return errors.New("aborted — no input")
	}
	ans := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if ans != "y" && ans != "yes" {
		return errors.New("aborted — no changes made")
	}
	return nil
}

// stdinIsTTY reports whether stdin is connected to a terminal. Production
// callers pass this into confirmDestructive; tests inject a bool directly.
func stdinIsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
