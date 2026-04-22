// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	connect "connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/docker"
	"github.com/specgraph/specgraph/internal/service"
	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/spf13/cobra"
)

// upIsInstalled is the seam tests swap to exercise the service-mode hint
// branch without touching the real launchd/systemd files. Kept as a bare
// var (not a deps struct) because `up` has exactly one seam; promote to a
// struct if a second seam ever appears.
var upIsInstalled = service.IsInstalled

// serviceModeHint returns the text `up` should print in service mode given
// the installation state, or "" if nothing should be printed. Extracted so
// tests can verify the branching without spinning up cobra/config machinery.
func serviceModeHint(installed bool) string {
	if installed {
		return ""
	}
	return "Service not installed — running container only. Run `specgraph install` to enable auto-start."
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the SpecGraph server (daemon, service, or manual)",
	RunE:  runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, _ []string) error {
	if err := xdg.EnsureDirs(); err != nil {
		return fmt.Errorf("ensure XDG dirs: %w", err)
	}

	cfg, err := config.LoadGlobal(globalConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	serverURL := cfg.Client.DefaultServer

	// If already running, exit early.
	if isServerHealthy(serverURL) {
		fmt.Printf("Already running at %s\n", serverURL)
		return nil
	}

	if cfg.Server.Docker {
		composeFile, err := docker.EnsureComposeFile(xdg.DataHome())
		if err != nil {
			return fmt.Errorf("ensure compose file: %w", err)
		}
		fmt.Println("Starting Docker Compose stack...")
		if err := docker.ComposeUp(composeFile); err != nil {
			return fmt.Errorf("compose up: %w", err)
		}
	}

	switch cfg.Server.Mode {
	case "service":
		// `up` no longer installs the service. Once registered, the OS service
		// manager keeps it alive — launchd via RunAtLoad + KeepAlive (see
		// internal/service/launchd.go), systemd via Restart=on-failure +
		// WantedBy=default.target (see internal/service/systemd.go). No action
		// needed here. If not installed, print a hint and continue so
		// docker-only users still get a running container.
		if hint := serviceModeHint(upIsInstalled()); hint != "" {
			fmt.Println(hint)
		}
	case "manual":
		fmt.Println("Manual mode: run `specgraph serve` in another terminal")
		return nil
	}

	// Health-check loop: up to 10 attempts, 1s apart, 2s timeout per attempt.
	client := specgraphv1connect.NewServerServiceClient(http.DefaultClient, serverURL)
	for range 10 {
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
		resp, err := client.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
		cancel()
		if err == nil && resp.Msg.Status != "" {
			fmt.Printf("SpecGraph server running at %s\n", serverURL)
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("server did not become healthy at %s", serverURL)
}

// isServerHealthy returns true if the server responds with a successful health check.
func isServerHealthy(serverURL string) bool {
	client := specgraphv1connect.NewServerServiceClient(http.DefaultClient, serverURL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := client.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
	return err == nil && resp.Msg.Status != ""
}

// serviceDestDir returns the OS-appropriate directory for the service definition file.
func serviceDestDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "Library", "LaunchAgents"), nil
	default: // linux
		return filepath.Join(filepath.Dir(xdg.ConfigHome()), "systemd", "user"), nil
	}
}

// serviceDefinitionFilename returns the OS-appropriate filename for the service definition.
func serviceDefinitionFilename() string {
	switch runtime.GOOS {
	case "darwin":
		return "com.specgraph.server.plist"
	default: // linux
		return "specgraph.service"
	}
}
