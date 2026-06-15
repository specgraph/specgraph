// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/specgraph/specgraph/internal/credentials"
	"github.com/specgraph/specgraph/internal/xdg"
)

var logoutServerFlag string

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke the stored session and remove local credentials",
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()
	serverURL := logoutServerFlag
	if serverURL == "" {
		resolved, _, err := resolveBaseURL()
		if err != nil {
			return fmt.Errorf("resolve server URL: %w", err)
		}
		serverURL = resolved
	}
	serverURL = strings.TrimRight(serverURL, "/")

	credPath := xdg.CredentialsFile()
	f, err := credentials.Load(credPath)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	token := f.TokenFor(serverURL)
	if token == "" {
		fmt.Fprintf(w, "No stored credential for %s\n", serverURL) //nolint:errcheck // writes to user stream
		return nil
	}

	if strings.HasPrefix(token, "spgr_ws_") {
		if err := revokeSession(cmd.Context(), serverURL, token); err != nil {
			fmt.Fprintf(w, "warning: server revoke failed: %v\n", err) //nolint:errcheck // writes to user stream
		}
	} else {
		fmt.Fprintf(w, "warning: removing a non-session credential (API key) for %s\n", serverURL) //nolint:errcheck // writes to user stream
	}

	delete(f.Servers, strings.TrimRight(serverURL, "/"))
	if err := f.Save(credPath); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	fmt.Fprintf(w, "Logged out of %s\n", serverURL) //nolint:errcheck // writes to user stream
	return nil
}

func revokeSession(ctx context.Context, serverURL, token string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/api/auth/logout", http.NoBody)
	if err != nil {
		return fmt.Errorf("build revoke request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func init() {
	logoutCmd.Flags().StringVar(&logoutServerFlag, "server", "", "server base URL (overrides resolved)")
	rootCmd.AddCommand(logoutCmd)
}
