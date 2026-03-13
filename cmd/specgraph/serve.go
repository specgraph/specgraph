// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/seanb4t/specgraph/internal/config"
	"github.com/seanb4t/specgraph/internal/docker"
	"github.com/seanb4t/specgraph/internal/drift"
	"github.com/seanb4t/specgraph/internal/linter"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	syncpkg "github.com/seanb4t/specgraph/internal/sync"
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
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.IsRemote() {
		return fmt.Errorf("config has remote server set — no need to run serve")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.Server.Mode == "docker" {
		composeFile, err := docker.EnsureComposeFile(".", cfg.Storage.Backend)
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

	switch cfg.Storage.Backend {
	case "memgraph":
		store, err := memgraph.New(ctx, cfg.Storage.Memgraph.BoltURI)
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

		constitutionPath := cfg.Storage.ConstitutionPath
		if bootstrapErr := bootstrapConstitution(ctx, store, constitutionPath); bootstrapErr != nil {
			return fmt.Errorf("constitution bootstrap: %w", bootstrapErr)
		}

		mux := server.NewMux(store)
		server.RegisterHealthService(mux)
		server.RegisterDecisionService(mux, store)
		server.RegisterGraphService(mux, store)
		server.RegisterClaimService(mux, store)
		server.RegisterConstitutionService(mux, store)
		server.RegisterAuthoringService(mux, store, store)
		server.RegisterExecutionService(mux, store)
		driftEngine := drift.NewEngine(store, nil)
		lintEngine := linter.NewEngine(store, nil)
		server.RegisterLifecycleService(mux, store, store, driftEngine, lintEngine, nil)
		// Derive inject output root from constitution path's parent directory.
		// This works for both relative paths (resolved against CWD) and absolute paths,
		// ensuring the server validates output_dir against the project root rather than
		// the server process's working directory.
		constitutionAbs, err := filepath.Abs(cfg.Storage.ConstitutionPath)
		if err != nil {
			return fmt.Errorf("resolve constitution path for inject root: %w", err)
		}
		// ConstitutionPath is like ".specgraph/constitution.yaml" — go up two levels to project root.
		projectRoot := filepath.Dir(filepath.Dir(constitutionAbs))
		syncHandler := server.RegisterSyncService(mux, store, store, store, projectRoot)
		runner := syncpkg.NewExecRunner()
		syncHandler.RegisterAdapter(syncpkg.NewBeadsAdapter(runner))
		syncHandler.RegisterAdapter(syncpkg.NewGitHubAdapter(runner, cfg.Sync.GitHubRepo))
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

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

		server.StartSweeper(sweeperCtx, store, 60*time.Second)
		fmt.Printf("SpecGraph server running at http://%s\n", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
	default:
		return fmt.Errorf("unsupported backend: %s", cfg.Storage.Backend)
	}

	return nil
}

const maxConstitutionSize = 1 << 20 // 1 MiB

func bootstrapConstitution(ctx context.Context, store storage.ConstitutionBackend, yamlPath string) error {
	// Check if constitution already exists in storage.
	_, err := store.GetConstitution(ctx)
	if err == nil {
		return nil // already exists
	}
	if !errors.Is(err, storage.ErrConstitutionNotFound) {
		return fmt.Errorf("check existing constitution: %w", err)
	}

	// Check file exists and size.
	info, err := os.Stat(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // missing constitution.yaml is intentional: server starts without one; callers use UpdateConstitution RPC
		}
		return fmt.Errorf("stat constitution YAML %s: %w", yamlPath, err)
	}
	if info.Size() > maxConstitutionSize {
		return fmt.Errorf("constitution YAML %s exceeds 1 MiB size limit", yamlPath)
	}

	cy, err := config.LoadConstitutionYAML(yamlPath)
	if err != nil {
		return fmt.Errorf("load constitution YAML: %w", err)
	}

	constitution := cy.ToDomain()

	if _, err := store.UpdateConstitution(ctx, constitution); err != nil {
		return fmt.Errorf("seed constitution: %w", err)
	}

	fmt.Println("Bootstrapped constitution from", yamlPath)
	return nil
}
