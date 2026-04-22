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

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the SpecGraph server",
	RunE:  runDown,
}

type downFlags struct {
	rmRetired bool // --rm is retired; this flag exists only to produce a helpful error
	purge     bool
	yes       bool
}

var downFlagsVar downFlags

func init() {
	rootCmd.AddCommand(downCmd)
	downCmd.Flags().BoolVar(&downFlagsVar.rmRetired, "rm", false, "")
	_ = downCmd.Flags().MarkHidden("rm") //nolint:errcheck // flag just registered, MarkHidden can't fail
	downCmd.Flags().BoolVar(&downFlagsVar.purge, "purge", false, "remove containers AND data volumes (destructive; prompts for confirmation)")
	downCmd.Flags().BoolVar(&downFlagsVar.yes, "yes", false, "skip the --purge confirmation prompt")
}

// downDeps names the side-effecting operations doDown performs so tests can
// swap them for call-tracking fakes.
type downDeps struct {
	stopService      func() error
	composeStop      func(composeFile string) error
	composeDownVols  func(composeFile string) error
	composeFileExist func(composeFile string) bool
}

var downFns = downDeps{
	stopService:      service.Stop,
	composeStop:      docker.ComposeStop,
	composeDownVols:  docker.ComposeDownWithVolumes,
	composeFileExist: func(p string) bool { _, err := os.Stat(p); return err == nil },
}

func runDown(_ *cobra.Command, _ []string) error {
	cfg, err := config.LoadGlobal(globalConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return doDown(cfg, downFlagsVar, os.Stdin, os.Stderr, stdinIsTTY())
}

// doDown is the testable core of `specgraph down`. Production wiring lives
// in runDown; tests call doDown directly with fake deps and synthetic input.
func doDown(cfg *config.GlobalConfig, flags downFlags, in io.Reader, out io.Writer, isTTY bool) error {
	if flags.rmRetired {
		return errors.New(`--rm has been retired.
  To remove the service definition:   specgraph uninstall
  To remove containers and data:       specgraph down --purge`)
	}

	if flags.purge {
		msg := "Destroy all data in specgraph-data volume? All SpecGraph workspaces on this machine share this volume."
		if err := confirmDestructive(in, out, isTTY, flags.yes, msg); err != nil {
			return err
		}
	}

	var stopErr error
	if cfg.Server.Mode == "service" {
		if err := downFns.stopService(); err != nil {
			_, _ = fmt.Fprintf(out, "warning: stop service: %v\n", err) //nolint:errcheck // writes to user stream; failure is non-recoverable
			stopErr = err
		}
	}

	if cfg.Server.Docker {
		composeFile := filepath.Join(xdg.DataHome(), "docker-compose.yaml")
		if downFns.composeFileExist(composeFile) {
			var composeErr error
			if flags.purge {
				if err := downFns.composeDownVols(composeFile); err != nil {
					composeErr = fmt.Errorf("compose down --volumes: %w", err)
				}
			} else {
				if err := downFns.composeStop(composeFile); err != nil {
					composeErr = fmt.Errorf("compose stop: %w", err)
				}
			}
			if composeErr != nil {
				// Join so a compose failure doesn't silently mask a captured service-stop error.
				if stopErr != nil {
					return errors.Join(fmt.Errorf("stop service: %w", stopErr), composeErr)
				}
				return composeErr
			}
		}
	}

	if stopErr != nil {
		return fmt.Errorf("stop service: %w", stopErr)
	}

	if flags.purge {
		_, _ = fmt.Fprintln(out, "SpecGraph stopped and volumes removed") //nolint:errcheck // writes to user stream; failure is non-recoverable
	} else {
		_, _ = fmt.Fprintln(out, "SpecGraph stopped") //nolint:errcheck // writes to user stream; failure is non-recoverable
	}
	return nil
}
