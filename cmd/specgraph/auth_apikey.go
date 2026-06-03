// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

var (
	apiKeyListUser      string
	apiKeyListRevoked   bool
	apiKeyCreateUser    string
	apiKeyCreateLabel   string
	apiKeyCreateDown    string
	apiKeyCreateExpires string
	apiKeyRotateExpires string
)

// parseExpiresAt converts an --expires-at flag value to a protobuf timestamp.
// An empty value yields (nil, nil), meaning "no expiry" on create and "inherit
// the old key's expiry" on rotate. A non-empty value must be RFC3339.
func parseExpiresAt(s string) (*timestamppb.Timestamp, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("invalid --expires-at %q (want RFC3339, e.g. 2026-01-02T15:04:05Z): %w", s, err)
	}
	return timestamppb.New(t), nil
}

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
		expiresAt, err := parseExpiresAt(apiKeyCreateExpires)
		if err != nil {
			return err
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateAPIKey(cmd.Context(), connect.NewRequest(&specv1.CreateAPIKeyRequest{
			UserId: apiKeyCreateUser, Label: apiKeyCreateLabel, RoleDowngrade: apiKeyCreateDown,
			ExpiresAt: expiresAt,
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
	Long: "Rotate an API key: mints a new secret and revokes the old key. Owner, " +
		"label, and role-downgrade are preserved from the old key. Use --expires-at " +
		"to set the new secret's validity window (omit to keep the old expiry).",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		expiresAt, err := parseExpiresAt(apiKeyRotateExpires)
		if err != nil {
			return err
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.RotateAPIKey(cmd.Context(), connect.NewRequest(&specv1.RotateAPIKeyRequest{
			KeyId: args[0], ExpiresAt: expiresAt,
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
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateExpires, "expires-at", "", "expiry as RFC3339 (e.g. 2026-01-02T15:04:05Z); omit for no expiry")
	authAPIKeyCreateCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	// Rotation preserves owner/label/role-downgrade from the old key, so only
	// the new secret's expiry is settable here.
	authAPIKeyRotateCmd.Flags().StringVar(&apiKeyRotateExpires, "expires-at", "", "new secret's expiry as RFC3339; omit to keep the old key's expiry")
	authAPIKeyRotateCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyCmd.AddCommand(authAPIKeyListCmd, authAPIKeyCreateCmd, authAPIKeyRevokeCmd, authAPIKeyRotateCmd)
	authCmd.AddCommand(authAPIKeyCmd)
}
