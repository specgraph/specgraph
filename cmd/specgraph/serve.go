// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/docker"
	"github.com/specgraph/specgraph/internal/drift"
	"github.com/specgraph/specgraph/internal/linter"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage/memgraph"
	syncpkg "github.com/specgraph/specgraph/internal/sync"
	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/specgraph/specgraph/web"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SpecGraph server",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().String("cors-origin", "", "Enable CORS for this origin (dev mode only)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, _ []string) error {
	cfg, err := config.LoadGlobal(xdg.ConfigFile())
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
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
		mgOpts := []memgraph.Option{memgraph.WithProject("_server")}
		if cfg.Server.Memgraph.Username != "" && cfg.Server.Memgraph.Password != "" {
			mgOpts = append(mgOpts, memgraph.WithAuth(cfg.Server.Memgraph.Username, cfg.Server.Memgraph.Password))
		}
		if cfg.Server.Memgraph.UseTLS {
			mgOpts = append(mgOpts, memgraph.WithTLS(true))
		}
		store, err := memgraph.New(ctx, cfg.Server.Memgraph.BoltURI, mgOpts...)
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

		authStore, err := auth.NewConfigStore(cfg.Auth, "")
		if err != nil {
			return fmt.Errorf("auth config: %w", err)
		}
		if !authStore.HasKeys() && !isLoopbackAddr(cfg.Server.Listen) {
			slog.Warn("server listening without authentication on non-loopback interface",
				"addr", cfg.Server.Listen,
				"risk", "all requests will have full admin access")
		}
		interceptor := auth.NewAuthInterceptor(authStore)
		maxBytes := connect.WithReadMaxBytes(4 << 20) // 4 MiB request body limit
		opts := connect.WithInterceptors(interceptor)

		mux := server.NewMux(store, opts, maxBytes)
		server.RegisterHealthService(mux, opts, maxBytes)
		server.RegisterDecisionService(mux, store, opts, maxBytes)
		server.RegisterGraphService(mux, store, opts, maxBytes)
		server.RegisterClaimService(mux, store, opts, maxBytes)
		server.RegisterConstitutionService(mux, store, opts, maxBytes)
		server.RegisterAuthoringService(mux, store, opts, maxBytes)
		// Template override dir defaults to .specgraph/templates in the working directory.
		// Users can place <pass_type>.md files there to customize analytical pass prompts.
		server.RegisterAnalyticalPassService(mux, store, ".specgraph/templates", opts, maxBytes)
		server.RegisterExecutionService(mux, store, opts, maxBytes)
		server.RegisterSliceService(mux, store, opts, maxBytes)
		server.RegisterExportService(mux, store, cfg.Export.SigningKey, buildVersion(), opts, maxBytes)
		driftEngine := drift.NewEngine(store, nil)
		lintEngine := linter.NewEngine(store, nil)
		server.RegisterLifecycleService(mux, store, driftEngine, lintEngine, nil, opts, maxBytes)

		// TODO(slice-7): Project root for inject should come from request context,
		// not daemon CWD. Pass empty string; inject handler needs rework.
		syncHandler := server.RegisterSyncService(mux, store, "", opts, maxBytes)
		runner := syncpkg.NewExecRunner()
		syncHandler.RegisterAdapter(syncpkg.NewBeadsAdapter(runner))
		syncHandler.RegisterAdapter(syncpkg.NewGitHubAdapter(runner, ""))

		// Register lightweight HTTP API endpoints (before static handler catch-all)
		server.RegisterAPIHandlers(mux, store, auth.RequireAuth(authStore))

		// Serve embedded UI static files
		webFS, err := fs.Sub(web.Build, "build")
		if err != nil {
			return fmt.Errorf("embedded web FS: %w", err)
		}
		// Register static handler as catch-all (after ConnectRPC paths)
		mux.Handle("/", server.StaticHandler(webFS))

		handler := server.SecurityHeaders(server.ProjectMiddleware(mux))

		// Optional CORS for dev mode (Vite on :5173 → Go on :8080)
		corsOrigin, err := cmd.Flags().GetString("cors-origin")
		if err != nil {
			return fmt.Errorf("cors-origin flag: %w", err)
		}
		if corsOrigin != "" {
			handler = server.CORSMiddleware(corsOrigin, handler)
		}
		addr := cfg.Server.Listen
		srv := &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		}

		go func() {
			<-ctx.Done()
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
		server.StartSweeper(sweeperCtx, store, 60*time.Second, slog.Default())
		fmt.Printf("SpecGraph server running at http://%s\n", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
	default:
		return fmt.Errorf("unsupported backend: %s", cfg.Server.Backend)
	}

	return nil
}

// isLoopbackAddr reports whether the listen address refers to a loopback interface.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
