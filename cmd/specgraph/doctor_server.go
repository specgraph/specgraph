// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/mark3labs/mcp-go/client"
	mcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/config"
)

// ServerReport describes the running SpecGraph server: reachability,
// version, MCP handshake status, and skill count.
type ServerReport struct {
	OK           bool   `json:"ok"`
	Reachable    bool   `json:"reachable"`
	Version      string `json:"version"`
	MCPHandshake string `json:"mcpHandshake"`        // "ok" | "failed" | "skipped"
	SkillsCount  int    `json:"skillsCount"`
	Error        string `json:"error,omitempty"`
}

// serverMCPURL resolves the MCP endpoint URL for the configured server.
func serverMCPURL() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	pc, err := config.LoadProject(cwd)
	if err != nil {
		if errors.Is(err, config.ErrProjectNotFound) {
			// No project config — fall through to globalCfg default server.
			globalCfg, gErr := loadGlobalCfg()
			if gErr != nil {
				return "", gErr
			}
			base := globalCfg.ResolveServer("", "")
			return strings.TrimRight(base, "/") + "/mcp/", nil
		}
		return "", err
	}
	globalCfg, err := loadGlobalCfg()
	if err != nil {
		return "", err
	}
	base := globalCfg.ResolveServer(pc.Slug, pc.Server)
	return strings.TrimRight(base, "/") + "/mcp/", nil
}

// countSkillsFromJSON parses the JSON array returned by specgraph_skills_list
// and returns the number of entries, or -1 on parse error.
func countSkillsFromJSON(text string) int {
	var entries []struct {
		Name    string `json:"name"`
		Summary string `json:"summary"`
		URI     string `json:"uri"`
	}
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		return -1
	}
	return len(entries)
}

// runServerGroup dials the configured SpecGraph server and runs three
// sub-checks: Connect Health RPC, MCP Streamable-HTTP Initialize handshake,
// and specgraph_skills_list count.
func runServerGroup(timeout time.Duration) ServerReport {
	rep := ServerReport{MCPHandshake: "skipped"}

	// ── Sub-check 1: Connect Health RPC ──────────────────────────────────
	hc, err := healthClient()
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := hc.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	rep.Reachable = true
	rep.Version = resp.Msg.GetVersion()

	// ── Sub-check 2: MCP Streamable-HTTP Initialize ───────────────────────
	mcpURL, err := serverMCPURL()
	if err != nil {
		rep.MCPHandshake = "failed"
		rep.Error = fmt.Sprintf("resolve MCP URL: %s", err)
		return rep
	}

	c, err := client.NewStreamableHttpClient(mcpURL)
	if err != nil {
		rep.MCPHandshake = "failed"
		rep.Error = fmt.Sprintf("mcp client: %s", err)
		return rep
	}

	mcpCtx, mcpCancel := context.WithTimeout(context.Background(), timeout)
	defer mcpCancel()

	startErr := c.Start(mcpCtx)
	if startErr != nil {
		_ = c.Close() //nolint:errcheck // cleanup path; start already failed
		rep.MCPHandshake = "failed"
		rep.Error = fmt.Sprintf("mcp start: %s", startErr)
		return rep
	}
	defer func() { _ = c.Close() }() //nolint:errcheck // best-effort cleanup

	_, err = c.Initialize(mcpCtx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "specgraph-doctor", Version: rep.Version},
		},
	})
	if err != nil {
		rep.MCPHandshake = "failed"
		rep.Error = fmt.Sprintf("mcp initialize: %s", err)
		return rep
	}
	rep.MCPHandshake = "ok"

	// ── Sub-check 3: specgraph_skills_list count ─────────────────────────
	listCtx, listCancel := context.WithTimeout(context.Background(), timeout)
	defer listCancel()

	res, err := c.CallTool(listCtx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "specgraph_skills_list"},
	})
	switch {
	case err != nil:
		rep.SkillsCount = -1
		rep.Error = fmt.Sprintf("skills list: %s", err)
		return rep
	case len(res.Content) == 0:
		rep.SkillsCount = -1
		rep.Error = "skills list: empty response"
		return rep
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		rep.SkillsCount = -1
		rep.Error = "skills list: first content item is not text"
		return rep
	}
	rep.SkillsCount = countSkillsFromJSON(tc.Text)
	if rep.SkillsCount < 0 {
		rep.Error = "skills list: invalid JSON payload"
		return rep
	}

	rep.OK = true
	return rep
}

// serverStatusLine renders the compact single-line form of a ServerReport.
func serverStatusLine(rep ServerReport) string {
	if rep.OK {
		return fmt.Sprintf("Server:         OK (v%s, mcp=%s, skills=%d)",
			rep.Version, rep.MCPHandshake, rep.SkillsCount)
	}
	if !rep.Reachable {
		return fmt.Sprintf("Server:         UNREACHABLE (%s)", rep.Error)
	}
	return fmt.Sprintf("Server:         PROBLEM (mcp=%s, %s)", rep.MCPHandshake, rep.Error)
}

var doctorServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Run only the Server group (used by `specgraph health`)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		timeout, err := cmd.Flags().GetDuration("timeout")
		if err != nil {
			return fmt.Errorf("timeout flag: %w", err)
		}
		rep := runServerGroup(timeout)
		fmt.Println(serverStatusLine(rep))
		if !rep.OK {
			cmd.SilenceUsage = true
			return fmt.Errorf("server unhealthy")
		}
		return nil
	},
}

func init() {
	doctorServerCmd.Flags().Duration("timeout", 2*time.Second, "Per-RPC timeout")
	doctorCmd.AddCommand(doctorServerCmd)
}
