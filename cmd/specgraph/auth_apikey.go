// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"io"
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
	apiKeyRotateUser    string
	apiKeyRotateExpires string
	apiKeyRevokeUser    string
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

// printSelfMintedKey renders the emit-once secret block for a self-minted key
// (self create or self rotate). The plaintext is shown exactly once; the command
// never writes it to disk or runs `export` — the user is told to store it in a
// secret manager or their ${SPECGRAPH_API_KEY} shell profile themselves
// (T-02-25: keep the secret out of shell history / on-disk files).
func printSelfMintedKey(w io.Writer, headline string, key *specv1.APIKey, plaintext string) error {
	if _, err := fmt.Fprintf(w, "%s %s (prefix %s).\n", headline, key.GetId(), key.GetPrefix()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n  %s\n\n", plaintext); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "Store this token now — it will not be shown again. "+
		"Save it in a secret manager or your ${SPECGRAPH_API_KEY} shell profile; "+
		"this command never writes it to disk.")
	return err
}

var authAPIKeyCmd = &cobra.Command{Use: "api-key", Short: "Manage API keys"}

var authAPIKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys (your own by default; --user <id> lists another user's, admin only)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		// No --user → self path: list the caller's own keys via ListMyAPIKeys,
		// authenticating with the stored login session (Finding D).
		if apiKeyListUser == "" {
			warnIfEnvKeyIgnored(cmd.ErrOrStderr())
			client, err := identitySessionClient()
			if err != nil {
				return err
			}
			resp, err := client.ListMyAPIKeys(cmd.Context(), connect.NewRequest(&specv1.ListMyAPIKeysRequest{}))
			if err != nil {
				return fmt.Errorf("list my api keys: %w", err)
			}
			if authJSON {
				return printJSON(cmd.OutOrStdout(), resp.Msg)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), render.APIKeyList(resp.Msg.GetKeys()))
			return err
		}
		// --user <id> → admin path (unchanged).
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
	Long: "Create an API key and print its secret exactly once.\n\n" +
		"With no --user, mints one of YOUR OWN keys (self-service) via the stored " +
		"login session. Unlike an admin mint, a self-minted key's expiry is " +
		"MANDATORY: omit --expires-at to accept the server default (90 days); the " +
		"server rejects any value beyond its hard maximum (180 days).\n\n" +
		"With --user <id>, an admin mints a key owned by another user (expiry " +
		"optional).",
	RunE: func(cmd *cobra.Command, _ []string) error {
		expiresAt, err := parseExpiresAt(apiKeyCreateExpires)
		if err != nil {
			return err
		}
		// No --user → self path: mint the caller's own key via CreateMyAPIKey,
		// owner derived server-side from the session (no user_id on the wire).
		if apiKeyCreateUser == "" {
			warnIfEnvKeyIgnored(cmd.ErrOrStderr())
			client, clientErr := identitySessionClient()
			if clientErr != nil {
				return clientErr
			}
			resp, callErr := client.CreateMyAPIKey(cmd.Context(), connect.NewRequest(&specv1.CreateMyAPIKeyRequest{
				Label: apiKeyCreateLabel, RoleDowngrade: apiKeyCreateDown, ExpiresAt: expiresAt,
			}))
			if callErr != nil {
				return fmt.Errorf("create my api key: %w", callErr)
			}
			if authJSON {
				return printJSON(cmd.OutOrStdout(), resp.Msg)
			}
			return printSelfMintedKey(cmd.OutOrStdout(), "Created API key", resp.Msg.GetKey(), resp.Msg.GetPlaintext())
		}
		// --user <id> → admin path (unchanged).
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
	Short: "Revoke an API key (your own by default; --user <id> revokes another user's, admin only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// No --user → self path: revoke one of the caller's own keys via
		// RevokeMyAPIKey (owner-scoped; a foreign/missing key surfaces NotFound).
		if apiKeyRevokeUser == "" {
			warnIfEnvKeyIgnored(cmd.ErrOrStderr())
			client, err := identitySessionClient()
			if err != nil {
				return err
			}
			if _, revokeErr := client.RevokeMyAPIKey(cmd.Context(), connect.NewRequest(&specv1.RevokeMyAPIKeyRequest{KeyId: args[0]})); revokeErr != nil {
				return fmt.Errorf("revoke my api key: %w", revokeErr)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Revoked %s\n", args[0])
			return err
		}
		// --user <id> → admin path (unchanged): revoke any key by id.
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
		"to set the new secret's validity window (omit to keep the old expiry).\n\n" +
		"With no --user, rotates one of YOUR OWN keys (self-service) via the stored " +
		"login session; the new secret's expiry is re-clamped to the self-service " +
		"policy (default 90 days, hard max 180 days). With --user <id>, an admin " +
		"rotates any key by id.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		expiresAt, err := parseExpiresAt(apiKeyRotateExpires)
		if err != nil {
			return err
		}
		// No --user → self path: rotate one of the caller's own keys.
		if apiKeyRotateUser == "" {
			warnIfEnvKeyIgnored(cmd.ErrOrStderr())
			client, clientErr := identitySessionClient()
			if clientErr != nil {
				return clientErr
			}
			resp, callErr := client.RotateMyAPIKey(cmd.Context(), connect.NewRequest(&specv1.RotateMyAPIKeyRequest{
				KeyId: args[0], ExpiresAt: expiresAt,
			}))
			if callErr != nil {
				return fmt.Errorf("rotate my api key: %w", callErr)
			}
			if authJSON {
				return printJSON(cmd.OutOrStdout(), resp.Msg)
			}
			return printSelfMintedKey(cmd.OutOrStdout(), "Rotated → new key", resp.Msg.GetKey(), resp.Msg.GetPlaintext())
		}
		// --user <id> → admin path (unchanged).
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
	authAPIKeyListCmd.Flags().StringVar(&apiKeyListUser, "user", "", "another user's ID to list (admin only; omit to list your own keys)")
	authAPIKeyListCmd.Flags().BoolVar(&apiKeyListRevoked, "include-revoked", false, "include revoked keys (admin --user path)")
	authAPIKeyListCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateUser, "user", "", "another user's ID to own the key (admin only; omit to mint your own key)")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateLabel, "label", "", "human-friendly label")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateDown, "role-downgrade", "", "cap the key's effective role")
	authAPIKeyCreateCmd.Flags().StringVar(&apiKeyCreateExpires, "expires-at", "", "expiry as RFC3339 (e.g. 2026-01-02T15:04:05Z); self-mint omit → 90d default (180d max), admin --user omit → no expiry")
	authAPIKeyCreateCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyRevokeCmd.Flags().StringVar(&apiKeyRevokeUser, "user", "", "another user's key to revoke (admin only; omit to revoke your own key)")
	// Rotation preserves owner/label/role-downgrade from the old key, so only
	// the new secret's expiry is settable here.
	authAPIKeyRotateCmd.Flags().StringVar(&apiKeyRotateUser, "user", "", "rotate another user's key (admin only; omit to rotate your own key)")
	authAPIKeyRotateCmd.Flags().StringVar(&apiKeyRotateExpires, "expires-at", "", "new secret's expiry as RFC3339; omit to keep the old key's expiry")
	authAPIKeyRotateCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authAPIKeyCmd.AddCommand(authAPIKeyListCmd, authAPIKeyCreateCmd, authAPIKeyRevokeCmd, authAPIKeyRotateCmd)
	authCmd.AddCommand(authAPIKeyCmd)
}
