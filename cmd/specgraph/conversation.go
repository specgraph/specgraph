// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render/markdown"
	"github.com/spf13/cobra"
)

var conversationCmd = &cobra.Command{
	Use:   "conversation",
	Short: "Manage authoring conversation logs",
}

var conversationRecordCmd = &cobra.Command{
	Use:   "record <slug>",
	Short: "Record authoring conversation exchanges for a spec stage",
	Args:  cobra.ExactArgs(1),
	RunE:  runConversationRecord,
}

var conversationListCmd = &cobra.Command{
	Use:   "list <slug>",
	Short: "List authoring conversation logs for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runConversationList,
}

var (
	convRecordStage    string
	convRecordJSONFile string
	convRecordIsAmend  bool
	convListStage      string
	convListJSON       bool
	convRecordJSON     bool
)

func init() {
	conversationRecordCmd.Flags().StringVar(&convRecordStage, "stage", "", "authoring stage (spark, shape, specify, decompose, approve)")
	conversationRecordCmd.Flags().StringVar(&convRecordJSONFile, "json-file", "", "path to JSON file containing conversation exchanges")
	conversationRecordCmd.Flags().BoolVar(&convRecordIsAmend, "amend", false, "mark as amend re-entry")
	conversationRecordCmd.Flags().BoolVar(&convRecordJSON, "json", false, "output as JSON")
	cobra.CheckErr(conversationRecordCmd.MarkFlagRequired("stage"))
	cobra.CheckErr(conversationRecordCmd.MarkFlagRequired("json-file"))

	conversationListCmd.Flags().StringVar(&convListStage, "stage", "", "filter by authoring stage")
	conversationListCmd.Flags().BoolVar(&convListJSON, "json", false, "output as JSON")

	conversationCmd.AddCommand(conversationRecordCmd)
	conversationCmd.AddCommand(conversationListCmd)
	rootCmd.AddCommand(conversationCmd)
}

// conversationRecordInput is the JSON structure for conversation record input.
type conversationRecordInput struct {
	Exchanges []struct {
		Role          string `json:"role"`
		Content       string `json:"content"`
		Stage         string `json:"stage"`
		Sequence      int32  `json:"sequence"`
		DecisionPoint bool   `json:"decision_point,omitempty"`
	} `json:"exchanges"`
}

func runConversationRecord(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := authoringClient()
	if err != nil {
		return err
	}

	var input conversationRecordInput
	if loadErr := loadJSONFileRaw(convRecordJSONFile, &input); loadErr != nil {
		return fmt.Errorf("conversation record: %w", loadErr)
	}

	exchanges := make([]*specv1.ConversationExchange, len(input.Exchanges))
	for i, e := range input.Exchanges {
		exchanges[i] = &specv1.ConversationExchange{
			Role:          e.Role,
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}

	resp, err := client.RecordConversation(ctx, connect.NewRequest(&specv1.RecordConversationRequest{
		Slug:      args[0],
		Stage:     convRecordStage,
		Exchanges: exchanges,
		IsAmend:   convRecordIsAmend,
	}))
	if err != nil {
		return fmt.Errorf("conversation record: %w", err)
	}

	if convRecordJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	log := resp.Msg.GetConversationLog()
	if log == nil {
		return fmt.Errorf("conversation record: missing conversation_log in response")
	}
	fmt.Printf("Recorded conversation: %s (stage=%s, exchanges=%d)\n",
		log.Id,
		log.Stage,
		log.ExchangeCount)
	return nil
}

func runConversationList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := authoringClient()
	if err != nil {
		return err
	}

	resp, err := client.ListConversations(ctx, connect.NewRequest(&specv1.ListConversationsRequest{
		Slug:  args[0],
		Stage: convListStage,
	}))
	if err != nil {
		return fmt.Errorf("conversation list: %w", err)
	}

	if convListJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	output := markdown.ConversationLogList(resp.Msg.ConversationLogs)
	if output == "" {
		fmt.Println("No conversation logs found.")
		return nil
	}
	fmt.Print(output)
	return nil
}
