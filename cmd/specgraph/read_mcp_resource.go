// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 SpecGraph Contributors

package main

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
)

var readMCPResourceCmd = &cobra.Command{
	Use:   "read-mcp-resource <uri>",
	Short: "Read an MCP resource from the SpecGraph server and print its body.",
	Long: "Reads the requested MCP resource via streamable-HTTP transport from " +
		"the configured SpecGraph server (see resolveBaseURL) and prints the " +
		"first text content body to stdout. Bearer auth comes from " +
		"SPECGRAPH_API_KEY or the credentials file.",
	Args: cobra.ExactArgs(1),
	RunE: runReadMCPResource,
}

func init() {
	rootCmd.AddCommand(readMCPResourceCmd)
}

func runReadMCPResource(cmd *cobra.Command, args []string) error {
	uri := args[0]

	baseURL, project, err := resolveBaseURL()
	if err != nil {
		return fmt.Errorf("resolve server URL: %w", err)
	}
	mcpURL := strings.TrimRight(baseURL, "/") + "/mcp/"
	httpClient := newAuthenticatedHTTPClient(project)

	c, err := client.NewStreamableHttpClient(mcpURL, transport.WithHTTPBasicClient(httpClient))
	if err != nil {
		return fmt.Errorf("mcp client: %w", err)
	}
	defer c.Close() //nolint:errcheck // best-effort cleanup on exit

	ctx := cmd.Context()
	if startErr := c.Start(ctx); startErr != nil {
		return fmt.Errorf("mcp start: %w", startErr)
	}

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "specgraph-cli",
				Version: buildVersion(),
			},
		},
	}
	if _, initErr := c.Initialize(ctx, initReq); initErr != nil {
		return fmt.Errorf("mcp initialize: %w", initErr)
	}

	readReq := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: uri},
	}
	resp, err := c.ReadResource(ctx, readReq)
	if err != nil {
		return fmt.Errorf("read resource %s: %w", uri, err)
	}

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	for _, content := range resp.Contents {
		switch v := content.(type) {
		case mcp.TextResourceContents:
			fmt.Fprint(out, v.Text) //nolint:errcheck // stdout write; not actionable
		case mcp.BlobResourceContents:
			fmt.Fprintf(errOut, "warning: skipping non-text resource content (uri=%s, mime=%s)\n", v.URI, v.MIMEType) //nolint:errcheck // stderr write; not actionable
		default:
			fmt.Fprintf(errOut, "warning: skipping unknown resource content type %T\n", content) //nolint:errcheck // stderr write; not actionable
		}
	}
	fmt.Fprintln(out) //nolint:errcheck // stdout write; not actionable
	return nil
}
