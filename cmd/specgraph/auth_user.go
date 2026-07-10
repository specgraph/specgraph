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
	userListKind       string
	userListRole       string
	userListIncludeDel bool
	userForce          bool

	userResyncRole       string
	userResyncRevokeKeys bool
)

var authUserCmd = &cobra.Command{Use: "user", Short: "Manage users"}

var authUserListCmd = &cobra.Command{
	Use:   "list",
	Short: "List users",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		req := &specv1.ListUsersRequest{
			Role:           userListRole,
			IncludeDeleted: userListIncludeDel,
		}
		switch userListKind {
		case "human":
			req.Kind = specv1.UserKind_USER_KIND_HUMAN
		case "service_account":
			req.Kind = specv1.UserKind_USER_KIND_SERVICE_ACCOUNT
		case "":
			// all kinds
		default:
			return fmt.Errorf("invalid --kind %q (want human|service_account)", userListKind)
		}
		resp, err := client.ListUsers(cmd.Context(), connect.NewRequest(req))
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), render.UserList(resp.Msg.GetUsers()))
		return err
	},
}

var authUserShowCmd = &cobra.Command{
	Use:   "show <user-id>",
	Short: "Show a single user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.GetUser(cmd.Context(), connect.NewRequest(&specv1.GetUserRequest{Id: args[0]}))
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), render.UserList([]*specv1.User{resp.Msg.GetUser()}))
		return err
	},
}

var authUserSetRoleCmd = &cobra.Command{
	Use:   "set-role <user-id> <role>",
	Short: "Change a user's role",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.UpdateUserRole(cmd.Context(), connect.NewRequest(&specv1.UpdateUserRoleRequest{Id: args[0], Role: args[1]}))
		if err != nil {
			return fmt.Errorf("set role: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Updated %s → role %s\n", args[0], resp.Msg.GetUser().GetRole())
		return err
	},
}

var authUserResyncCmd = &cobra.Command{
	Use:   "resync <user-id>",
	Short: "Force re-apply a user's role and optionally revoke their keys (AUTH-02)",
	Long: "Force a re-sync of a user's authoritative role. The role write clamps " +
		"every standing API key on its next request via the live-role read; pass " +
		"--revoke-keys for a hard off-board that also revokes the user's active keys.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		resp, err := client.ResyncUserRole(cmd.Context(), connect.NewRequest(&specv1.ResyncUserRoleRequest{
			Id:         args[0],
			Role:       userResyncRole,
			RevokeKeys: userResyncRevokeKeys,
		}))
		if err != nil {
			return fmt.Errorf("resync user role: %w", err)
		}
		if authJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		keyNote := "keys left active"
		if userResyncRevokeKeys {
			keyNote = "keys revoked"
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Re-synced %s → role %s (%s)\n",
			args[0], resp.Msg.GetUser().GetRole(), keyNote)
		return err
	},
}

var authUserDeleteCmd = &cobra.Command{
	Use:   "delete <user-id>",
	Short: "Soft-delete a user (revokes their keys)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		if _, delErr := client.SoftDeleteUser(cmd.Context(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: args[0], Force: userForce})); delErr != nil {
			return fmt.Errorf("delete user: %w", delErr)
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Soft-deleted %s\n", args[0])
		return err
	},
}

var authUserPurgeCmd = &cobra.Command{
	Use:   "purge <user-id>",
	Short: "Permanently delete a user (irreversible)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := identityClient()
		if err != nil {
			return err
		}
		if _, purgeErr := client.PurgeUser(cmd.Context(), connect.NewRequest(&specv1.PurgeUserRequest{Id: args[0], Force: userForce})); purgeErr != nil {
			return fmt.Errorf("purge user: %w", purgeErr)
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Purged %s\n", args[0])
		return err
	},
}

func init() {
	authUserListCmd.Flags().StringVar(&userListKind, "kind", "", "filter by kind: human|service_account")
	authUserListCmd.Flags().StringVar(&userListRole, "role", "", "filter by role")
	authUserListCmd.Flags().BoolVar(&userListIncludeDel, "include-deleted", false, "include soft-deleted users")
	// --json on the JSON-emitting commands (shared authJSON var from auth.go).
	authUserListCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authUserShowCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authUserSetRoleCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authUserResyncCmd.Flags().StringVar(&userResyncRole, "role", "", "authoritative role to re-apply (required)")
	cobra.CheckErr(authUserResyncCmd.MarkFlagRequired("role"))
	authUserResyncCmd.Flags().BoolVar(&userResyncRevokeKeys, "revoke-keys", false, "also revoke the user's active API keys (hard off-board)")
	authUserResyncCmd.Flags().BoolVar(&authJSON, "json", false, "output as JSON")
	authUserDeleteCmd.Flags().BoolVar(&userForce, "force", false, "allow deleting the bootstrap admin")
	authUserPurgeCmd.Flags().BoolVar(&userForce, "force", false, "allow purging the bootstrap admin")
	authUserCmd.AddCommand(authUserListCmd, authUserShowCmd, authUserSetRoleCmd, authUserResyncCmd, authUserDeleteCmd, authUserPurgeCmd)
	authCmd.AddCommand(authUserCmd)
}
