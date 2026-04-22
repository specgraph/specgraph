// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/docker"
	"github.com/specgraph/specgraph/internal/service"
	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Unregister the SpecGraph user service (launchd/systemd); preserves data",
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

// uninstallDeps names the side-effecting operations doUninstall performs.
// Tests swap these; production wiring is below.
type uninstallDeps struct {
	isInstalled      func() bool
	serviceUninstall func(defPath string) error
	composeStop      func(composeFile string) error
	composeFileExist func(composeFile string) bool
}

var uninstallFns = uninstallDeps{
	isInstalled:      service.IsInstalled,
	serviceUninstall: service.Uninstall,
	composeStop:      docker.ComposeStop,
	composeFileExist: func(p string) bool { _, err := os.Stat(p); return err == nil },
}

func runUninstall(_ *cobra.Command, _ []string) error {
	cfg, err := config.LoadGlobal(globalConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	destDir, err := serviceDestDir()
	if err != nil {
		return fmt.Errorf("service dest dir: %w", err)
	}
	defPath := filepath.Join(destDir, serviceDefinitionFilename())
	composeFile := filepath.Join(xdg.DataHome(), "docker-compose.yaml")

	return doUninstall(cfg, defPath, composeFile, os.Stderr)
}

// doUninstall is the testable core of `specgraph uninstall`.
//
// It deliberately does NOT remove the compose file. Preserving the file
// alongside the preserved volume is the whole point of non-destructive
// uninstall: a subsequent `install` finds the same compose file pointing
// at the same volume and resumes exactly where the user left off.
func doUninstall(cfg *config.GlobalConfig, defPath, composeFile string, out io.Writer) error {
	if cfg.Server.Mode == "manual" {
		return errors.New("nothing to uninstall in manual mode — no service is registered")
	}

	if !uninstallFns.isInstalled() {
		_, _ = fmt.Fprintln(out, "SpecGraph service is not installed; nothing to do") //nolint:errcheck // writes to user stream
		return nil
	}

	if err := uninstallFns.serviceUninstall(defPath); err != nil {
		return fmt.Errorf("uninstall service: %w", err)
	}

	if cfg.Server.Docker && uninstallFns.composeFileExist(composeFile) {
		if err := uninstallFns.composeStop(composeFile); err != nil {
			return fmt.Errorf("compose stop: %w", err)
		}
	}

	_, _ = fmt.Fprintln(out, "SpecGraph uninstalled (data preserved; run `specgraph down --purge` to remove volumes)") //nolint:errcheck // writes to user stream
	return nil
}
