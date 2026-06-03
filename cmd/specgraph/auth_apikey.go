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
	apiKeyListUser    string
	apiKeyListRevoked bool
	apiKeyCreateUser  string
	apiKeyCreateLabel string
	apiKeyCreateDown  string
	apiKeyRotateUser  string
	apiKeyRotateLabel string
	apiKeyRotateDown  string
)

var authAPIKeyCmd = &cobra.Command{Use: "api-key", Short: "Manage API keys"}

var authAPIKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.ListAPIKeys(cmd.Context(), connect.NewRequest(&specv1.ListAPIKeysRequest{
			UserId: apiKeyListUser, IncludeRevoked: apiKeyListRevoked,
		}))
		if err != nil {
			return fmt.Errorf("list api keys: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), render.APIKeyList(resp.Msg.GetKeys()))
		return err
	},
}

var authAPIKeyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API key (prints the secret once)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if apiKeyCreateUser == "" {
			return fmt.Errorf("--user is required")
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateAPIKey(cmd.Context(), connect.NewRequest(&specv1.CreateAPIKeyRequest{
			UserId: apiKeyCreateUser, Label: apiKeyCreateLabel, RoleDowngrade: apiKeyCreateDown,
		}))
		if err != nil {
			return fmt.Errorf("create api key: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		w := cmd.OutOrStdout()
		if _, err = fmt.Fprintf(w, "Created API key %s (prefix %s).\n", resp.Msg.GetKey().GetId(), resp.Msg.GetKey().GetPrefix()); err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "\n  %s\n\n", resp.Msg.GetPlaintext()); err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, "Store this token now — it will not be shown again.")
		return err
	},
}

var authAPIKeyRevokeCmd = &cobra.Command{
	Use:   "revoke <key-id>",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		if _, revokeErr := client.RevokeAPIKey(cmd.Context(), connect.NewRequest(&specv1.RevokeAPIKeyRequest{KeyId: args[0]})); revokeErr != nil {
			return fmt.Errorf("revoke api key: %w", revokeErr)
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Revoked %s\n", args[0])
		return err
	},
}

var authAPIKeyRotateCmd = &cobra.Command{
	Use:   "rotate <key-id>",
	Short: "Rotate an API key (revokes the old, prints the new secret once)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// storage's RotateAPIKey requires the new key's owner + metadata (it
		// does not copy from the old key; there is no get-key-by-id method).
		if apiKeyRotateUser == "" {
			return fmt.Errorf("--user is required (owner of the key being rotated)")
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.RotateAPIKey(cmd.Context(), connect.NewRequest(&specv1.RotateAPIKeyRequest{
			KeyId: args[0], UserId: apiKeyRotateUser, Label: apiKeyRotateLabel, RoleDowngrade: apiKeyRotateDown,
		}))
		if err != nil {
			return fmt.Errorf("rotate api key: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		w := cmd.OutOrStdout()
		if _, err = fmt.Fprintf(w, "Rotated → new key %s (prefix %s).\n", resp.Msg.GetKey().GetId(), resp.Msg.GetKey().GetPrefix()); err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "\n  %s\n\n", resp.Msg.GetPlaintext()); err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, "Store this token now — it will not be shown again.")
		return err
	},
}

func init() {
	authAPIKeyListCmd.Flags().StringVar(&apiKeyListUser, "user", "", "filter by user ID (omit to list all keys, admin only)")
	authAPIKeyListCmd.Flags().BoolVar(&apiKeyListRevoked, "include-revoked", false, "include revoked keys")
	authAPIKeyListCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateUser, "user", "", "user ID to own the key (required)")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateLabel, "label", "", "human-friendly label")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateDown, "role-downgrade", "", "cap the key's effective role")
	authAPIKeyCreateCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyRotateCmd.Flags().StringVar(&apiKeyRotateUser, "user", "", "owner of the key being rotated (required)")
	authAPIKeyRotateCmd.Flags().StringVar(&apiKeyRotateLabel, "label", "", "label for the new key")
	authAPIKeyRotateCmd.Flags().StringVar(&apiKeyRotateDown, "role-downgrade", "", "role downgrade for the new key")
	authAPIKeyRotateCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyCmd.AddCommand(authAPIKeyListCmd, authAPIKeyCreateCmd, authAPIKeyRevokeCmd, authAPIKeyRotateCmd)
	authCmd.AddCommand(authAPIKeyCmd)
}
