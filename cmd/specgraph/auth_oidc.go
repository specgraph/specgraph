// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

var (
	oidcListUser    string
	oidcUnbindUser  string
	oidcUnbindForce bool
)

var authOIDCCmd = &cobra.Command{Use: "oidc", Short: "Manage OIDC bindings"}

var authOIDCListCmd = &cobra.Command{
	Use:   "list",
	Short: "List a user's OIDC bindings",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if oidcListUser == "" {
			return fmt.Errorf("--user is required")
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.ListOIDCBindings(cmd.Context(), connect.NewRequest(&specv1.ListOIDCBindingsRequest{UserId: oidcListUser}))
		if err != nil {
			return fmt.Errorf("list oidc bindings: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), render.OIDCBindingList(resp.Msg.GetBindings()))
		return err
	},
}

var authOIDCUnbindCmd = &cobra.Command{
	Use:   "unbind <binding-id>",
	Short: "Remove an OIDC binding",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if oidcUnbindUser == "" {
			return fmt.Errorf("--user is required (owner of the binding)")
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		if _, unbindErr := client.UnbindOIDC(cmd.Context(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: args[0], UserId: oidcUnbindUser, Force: oidcUnbindForce})); unbindErr != nil {
			return fmt.Errorf("unbind oidc: %w", unbindErr)
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Unbound %s\n", args[0])
		return err
	},
}

func init() {
	authOIDCListCmd.Flags().StringVar(&oidcListUser, "user", "", "user ID (required)")
	authOIDCListCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authOIDCUnbindCmd.Flags().StringVar(&oidcUnbindUser, "user", "", "owner of the binding (required)")
	authOIDCUnbindCmd.Flags().BoolVar(&oidcUnbindForce, "force", false, "allow removing the user's only credential")
	authOIDCCmd.AddCommand(authOIDCListCmd, authOIDCUnbindCmd)
	authCmd.AddCommand(authOIDCCmd)
}
