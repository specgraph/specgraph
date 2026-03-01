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
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
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
		defer func() {
			if err := store.Close(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "warning: close store: %v\n", err)
			}
		}()

		mux := server.NewMux(store)
		server.RegisterHealthService(mux)
		server.RegisterDecisionService(mux, store)
		server.RegisterGraphService(mux, store)
		server.RegisterClaimService(mux, store)
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			fmt.Println("\nShutting down...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer shutdownCancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				fmt.Fprintf(os.Stderr, "warning: server shutdown: %v\n", err)
			}
		}()

		fmt.Printf("SpecGraph server running at http://%s\n", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
	default:
		return fmt.Errorf("unsupported backend: %s", cfg.Storage.Backend)
	}

	return nil
}
