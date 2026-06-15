// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package browser opens a URL in the user's default browser.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openCommand returns the platform command + args to open url.
func openCommand(goos, url string) (name string, args []string) {
	switch goos {
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		return "open", []string{url}
	default:
		return "xdg-open", []string{url}
	}
}

// Open launches the default browser at url. It returns an error if the platform
// opener cannot be started; callers should fall back to printing the URL.
func Open(url string) error {
	name, args := openCommand(runtime.GOOS, url)
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	// Reap the opener once it exits so it doesn't linger as a zombie for the
	// lifetime of the (potentially minutes-long) login wait.
	go func() { _ = cmd.Wait() }() //nolint:errcheck // best-effort reap
	return nil
}
