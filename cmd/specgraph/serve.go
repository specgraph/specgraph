// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seanb4t/specgraph/internal/config"
	"github.com/seanb4t/specgraph/internal/docker"
	"github.com/seanb4t/specgraph/internal/drift"
	"github.com/seanb4t/specgraph/internal/linter"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	syncpkg "github.com/seanb4t/specgraph/internal/sync"
	"github.com/seanb4t/specgraph/internal/xdg"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SpecGraph server",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	cfg, err := config.LoadGlobal(xdg.ConfigFile())
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.Server.Docker {
		composeFile, err := docker.EnsureComposeFile(xdg.DataHome(), cfg.Server.Backend)
		if err != nil {
			return err
		}
		fmt.Println("Starting Docker Compose stack...")
		if err := docker.ComposeUp(composeFile); err != nil {
			return err
		}
		defer func() {
			if err := docker.ComposeDown(composeFile); err != nil {
				fmt.Fprintf(os.Stderr, "warning: compose down: %v\n", err)
			}
		}()
	}

	switch cfg.Server.Backend {
	case "memgraph":
		store, err := memgraph.New(ctx, cfg.Server.Memgraph.BoltURI, memgraph.WithProject("_server"))
		if err != nil {
			return fmt.Errorf("connect to memgraph: %w", err)
		}

		// Defers run LIFO: stopSweeper runs before store.Close, preventing races
		// where the sweeper goroutine calls ReleaseExpiredClaims on a closed store.
		defer func() {
			if closeErr := store.Close(ctx); closeErr != nil {
				fmt.Fprintf(os.Stderr, "warning: close store: %v\n", closeErr)
			}
		}()
		sweeperCtx, stopSweeper := context.WithCancel(ctx)
		defer stopSweeper()

		mux := server.NewMux(store)
		server.RegisterHealthService(mux)
		server.RegisterDecisionService(mux, store)
		server.RegisterGraphService(mux, store)
		server.RegisterClaimService(mux, store)
		server.RegisterConstitutionService(mux, store)
		server.RegisterAuthoringService(mux, store)
		server.RegisterExecutionService(mux, store)
		driftEngine := drift.NewEngine(store, nil)
		lintEngine := linter.NewEngine(store, nil)
		server.RegisterLifecycleService(mux, store, driftEngine, lintEngine, nil)

		// TODO(slice-7): Project root for inject should come from request context,
		// not daemon CWD. Pass empty string; inject handler needs rework.
		syncHandler := server.RegisterSyncService(mux, store, "")
		runner := syncpkg.NewExecRunner()
		syncHandler.RegisterAdapter(syncpkg.NewBeadsAdapter(runner))
		syncHandler.RegisterAdapter(syncpkg.NewGitHubAdapter(runner, ""))

		handler := server.ProjectMiddleware(mux)
		addr := cfg.Server.Listen
		srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}

		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			fmt.Println("\nShutting down...")
			stopSweeper()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer shutdownCancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				fmt.Fprintf(os.Stderr, "warning: server shutdown: %v\n", err)
			}
		}()

		// TODO(slice-7): Sweeper only covers the _server project. A cross-project
		// sweeper needs to iterate all Project nodes and release expired claims
		// in each. Track this as a follow-up issue.
		server.StartSweeper(sweeperCtx, store, 60*time.Second)
		fmt.Printf("SpecGraph server running at http://%s\n", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
	default:
		return fmt.Errorf("unsupported backend: %s", cfg.Server.Backend)
	}

	return nil
}

