// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/docker"
	"github.com/specgraph/specgraph/internal/service"
	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Register SpecGraph as a user service (launchd/systemd) and start it",
	RunE:  runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
}

// installDeps names the side-effecting operations doInstall performs so tests
// can swap them for call-tracking fakes.
type installDeps struct {
	isInstalled    func() bool
	generateDef    func() (string, error)
	composeUp      func(composeFile string) error
	composeStop    func(composeFile string) error
	serviceInstall func(defPath string) error
	removeFile     func(path string) error
}

// installFns holds the static production dependencies. generateDef is filled
// in per-call by runInstall because it closes over runtime-resolved paths.
// Initialized at package scope so a zero-valued default is never observable.
var installFns = installDeps{
	isInstalled:    service.IsInstalled,
	composeUp:      docker.ComposeUp,
	composeStop:    docker.ComposeStop,
	serviceInstall: service.Install,
	removeFile:     os.Remove,
	generateDef: func() (string, error) {
		return "", errors.New("generateDef not wired; call runInstall or swap installFns in a test")
	},
}

func runInstall(_ *cobra.Command, _ []string) error {
	cfg, err := loadGlobalCfg()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}
	binaryPath, err = filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("resolve absolute binary path: %w", err)
	}

	destDir, err := serviceDestDir()
	if err != nil {
		return fmt.Errorf("service dest dir: %w", err)
	}
	defPath := filepath.Join(destDir, serviceDefinitionFilename())
	composeFile := filepath.Join(xdg.DataHome(), "docker-compose.yaml")

	// Only overwrite generateDef — the other fields were initialized at
	// package scope and must stay stable across runInstall calls so that
	// concurrent tests (or sub-command flows) don't race.
	installFns.generateDef = func() (string, error) {
		return service.Generate(destDir, service.Config{
			BinaryPath: binaryPath,
			ConfigPath: globalConfigPath(),
			LogPath:    filepath.Join(xdg.StateHome(), "server.log"),
		})
	}

	return doInstall(cfg, defPath, composeFile, os.Stderr)
}

// doInstall is the testable core. Production passes the real defPath/composeFile;
// tests swap installFns to observe call order and simulate failures.
func doInstall(cfg *config.GlobalConfig, defPath, composeFile string, out io.Writer) error {
	if cfg.Server.Mode == "manual" {
		return errors.New("cannot install in manual mode — change server.mode to \"service\" in config or use `specgraph serve` directly")
	}

	// Idempotent path: definition already on disk. Launchd/systemd keeps the
	// service alive via RunAtLoad + KeepAlive; we just make sure the container
	// backend is up and exit cleanly.
	if installFns.isInstalled() {
		if cfg.Server.Docker {
			if err := installFns.composeUp(composeFile); err != nil {
				return fmt.Errorf("compose up: %w", err)
			}
		}
		_, _ = fmt.Fprintln(out, "SpecGraph already installed — ensured container is running") //nolint:errcheck // writes to user stream
		return nil
	}

	// Generate definition first so service.Install has a file to register.
	generatedPath, err := installFns.generateDef()
	if err != nil {
		return fmt.Errorf("generate service definition: %w", err)
	}
	if generatedPath != "" {
		defPath = generatedPath
	}

	// Bring the container up BEFORE registering the service. generateDef
	// already wrote the plist, so if compose fails we roll that back.
	if cfg.Server.Docker {
		if err := installFns.composeUp(composeFile); err != nil {
			errs := []error{fmt.Errorf("compose up: %w", err)}
			// Clean up the definition file we just wrote so a retry starts fresh.
			// A missing file is expected on some failure paths; other errors
			// (permissions, read-only FS) need to surface so the user can fix them.
			if rmErr := installFns.removeFile(defPath); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("remove definition: %w", rmErr))
			}
			return errors.Join(errs...)
		}
	}

	// Register + load the service. If this fails, unwind in reverse order:
	// halt the container we just started, then remove the definition file.
	// Both cleanup errors are joined to the outer error so the user sees
	// orphan state (container still up, plist still on disk) rather than
	// a bare "install service failed" with no diagnostic.
	if err := installFns.serviceInstall(defPath); err != nil {
		errs := []error{fmt.Errorf("install service: %w", err)}
		if cfg.Server.Docker {
			if stopErr := installFns.composeStop(composeFile); stopErr != nil {
				errs = append(errs, fmt.Errorf("cleanup: compose stop: %w", stopErr))
			}
		}
		if rmErr := installFns.removeFile(defPath); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("cleanup: remove definition: %w", rmErr))
		}
		return errors.Join(errs...)
	}

	_, _ = fmt.Fprintln(out, "SpecGraph installed and service started") //nolint:errcheck // writes to user stream
	return nil
}
