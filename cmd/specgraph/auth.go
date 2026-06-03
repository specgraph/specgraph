// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage identity: users, service accounts, API keys, and OIDC bindings",
}

// authJSON backs the --json flag across the auth subtree. There is no global
// --json flag in this CLI (each command declares its own — see spec.go's
// per-command showJSON/listJSON pattern). Cobra binds a distinct --json flag
// on each leaf command to this shared var; only one command runs per
// invocation, so sharing is safe and keeps the registrations terse.
var authJSON bool

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the identity of the current credential",
	RunE:  runAuthWhoami,
}

func runAuthWhoami(cmd *cobra.Command, _ []string) error {
	client, err := identityClient()
	if err != nil {
		return err
	}
	resp, err := client.Whoami(cmd.Context(), connect.NewRequest(&specv1.WhoamiRequest{}))
	if err != nil {
		return fmt.Errorf("whoami: %w", err)
	}
	if authJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	w := cmd.OutOrStdout()
	if _, err = fmt.Fprintf(w, "Subject:        %s\n", resp.Msg.GetSubject()); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "User ID:        %s\n", resp.Msg.GetUserId()); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "Display name:   %s\n", resp.Msg.GetDisplayName()); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "Role:           %s\n", resp.Msg.GetRole()); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "Effective role: %s\n", resp.Msg.GetEffectiveRole()); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "Email:          %s\n", resp.Msg.GetEmail()); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Source:         %s\n", resp.Msg.GetSource())
	return err
}

func init() {
	authWhoamiCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authCmd.AddCommand(authWhoamiCmd)
	rootCmd.AddCommand(authCmd)
}
