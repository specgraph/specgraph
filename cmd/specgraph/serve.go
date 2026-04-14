// SPDX-License-Identifier: Apache-2.0
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
	mcppkg "github.com/specgraph/specgraph/internal/mcp"
	"github.com/specgraph/specgraph/internal/notify"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
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
	serveCmd.Flags().String("pg-url", "", "PostgreSQL connection URL (overrides config; env: SPECGRAPH_PG_URL)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, _ []string) error {
	cfg, err := config.LoadGlobal(xdg.ConfigFile())
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Resolve --pg-url flag / SPECGRAPH_PG_URL env override before Docker compose
	// so the correct backend URL is used when starting the compose stack.
	pgURL, err := cmd.Flags().GetString("pg-url")
	if err != nil {
		return fmt.Errorf("pg-url flag: %w", err)
	}
	if pgURL == "" {
		pgURL = os.Getenv("SPECGRAPH_PG_URL")
	}
	// Flag/env override selects postgres backend automatically.
	if pgURL != "" {
		cfg.Server.Backend = "postgres"
		cfg.Server.Postgres.URL = pgURL
	}

	if cfg.Server.Docker {
		composeFile, dockerErr := docker.EnsureComposeFile(xdg.DataHome())
		if dockerErr != nil {
			return dockerErr
		}
		fmt.Println("Starting Docker Compose stack...")
		if upErr := docker.ComposeUp(composeFile); upErr != nil {
			return upErr
		}
		defer func() {
			if downErr := docker.ComposeDown(composeFile); downErr != nil {
				fmt.Fprintf(os.Stderr, "warning: compose down: %v\n", downErr)
			}
		}()
	}

	// Create backend store.
	type backendStore interface {
		storage.Scoper
		storage.ScopedBackend
		server.ClaimSweeper
		Close(context.Context) error
	}
	var store backendStore

	connURL := cfg.Server.Postgres.URL
	if connURL == "" {
		return fmt.Errorf("postgres backend requires a connection URL (set server.postgres.url in config or use --pg-url)")
	}
	s, pgErr := postgres.New(ctx, connURL, postgres.WithProject("_server"))
	if pgErr != nil {
		return fmt.Errorf("connect to postgres: %w", pgErr)
	}
	store = s

	// Defers run LIFO: stopSweeper runs before store.Close, preventing races
	// where the sweeper goroutine calls ReleaseExpiredClaims on a closed store.
	defer func() {
		if closeErr := store.Close(ctx); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: close store: %v\n", closeErr)
		}
	}()
	sweeperCtx, stopSweeper := context.WithCancel(ctx)
	defer stopSweeper()

	// Register change notification subscribers.
	store.Subscribe(notify.NewImpactLogger())

	credPath := xdg.CredentialsFile()
	authStore, err := auth.NewConfigStore(cfg.Auth, credPath)
	if err != nil {
		return fmt.Errorf("auth config: %w", err)
	}

	mode := cfg.Auth.Mode
	if mode == "" {
		mode = "local"
	}

	// Validate auth mode.
	switch mode {
	case "local", "oidc", "mixed":
	default:
		return fmt.Errorf("invalid auth.mode %q (must be local, oidc, or mixed)", mode)
	}
	if mode == "oidc" && len(cfg.Auth.OIDCProviders) == 0 {
		return fmt.Errorf("auth.mode=oidc requires at least one oidc_providers entry")
	}

	// Bootstrap: generate default admin key in local mode if none configured.
	if mode == "local" && !authStore.HasKeys() {
		key, bootstrapErr := auth.Bootstrap(credPath)
		if bootstrapErr != nil {
			slog.Warn("auth bootstrap skipped (credentials directory not writable)",
				"path", credPath,
				"error", bootstrapErr.Error())
		} else {
			fmt.Fprintf(os.Stderr, "\n  SpecGraph generated a default admin API key:\n\n    %s\n\n  Save this key — it won't be shown again.\n  Stored in: %s\n\n", key, credPath)

			// Reload store with the new key.
			authStore, err = auth.NewConfigStore(cfg.Auth, credPath)
			if err != nil {
				return fmt.Errorf("reload auth after bootstrap: %w", err)
			}
		}
	} else if mode == "local" {
		if _, statErr := os.Stat(credPath); statErr == nil {
			fmt.Fprintf(os.Stderr, "  Auth: using credentials from %s\n", credPath)
		}
	}

	if warning := auth.CheckCredentialPermissions(credPath); warning != "" {
		slog.Warn(warning)
	}

	// Set up OIDC providers (only for oidc/mixed modes).
	var oidcStores []*auth.OIDCStore
	if mode != "local" {
		defaultRole := cfg.Auth.DefaultRole
		if defaultRole == "" {
			defaultRole = "reader"
		}

		rolePerms := make(map[auth.Role][]string)
		for role, perms := range auth.DefaultRolePermissions {
			rolePerms[role] = perms
		}
		for name, rc := range cfg.Auth.Roles {
			rolePerms[auth.Role(name)] = rc.Permissions
		}

		for _, pc := range cfg.Auth.OIDCProviders {
			issuerCtx, issuerCancel := context.WithTimeout(ctx, 10*time.Second)
			oidcStore, oidcErr := auth.NewOIDCStore(issuerCtx, pc, defaultRole, rolePerms)
			issuerCancel()
			if oidcErr != nil {
				return fmt.Errorf("OIDC provider %s: %w", pc.ID, oidcErr)
			}
			oidcStores = append(oidcStores, oidcStore)
			slog.Info("auth: OIDC provider configured", "id", pc.ID, "issuer", pc.Issuer)
		}
	}

	compositeStore, csErr := auth.NewCompositeStore(authStore, oidcStores, mode)
	if csErr != nil {
		return fmt.Errorf("auth composite store: %w", csErr)
	}

	if !compositeStore.HasAuth() && !isLoopbackAddr(cfg.Server.Listen) {
		slog.Warn("server listening without authentication on non-loopback interface",
			"addr", cfg.Server.Listen,
			"risk", "configure API keys or OIDC providers")
	}
	interceptor := auth.NewAuthInterceptor(compositeStore)
	maxBytes := connect.WithReadMaxBytes(4 << 20) // 4 MiB request body limit
	opts := connect.WithInterceptors(interceptor)

	mux := server.NewMux(store, opts, maxBytes)
	server.RegisterHealthService(mux, opts, maxBytes)
	server.RegisterDecisionService(mux, store, opts, maxBytes)
	server.RegisterGraphService(mux, store, opts, maxBytes)
	server.RegisterClaimService(mux, store, opts, maxBytes)
	server.RegisterConstitutionService(mux, store, opts, maxBytes)
	server.RegisterAuthoringService(mux, store, opts, maxBytes)
	server.RegisterAnalyticalPassService(mux, store, ".specgraph/templates", opts, maxBytes)
	server.RegisterExecutionService(mux, store, opts, maxBytes)
	server.RegisterSliceService(mux, store, opts, maxBytes)
	server.RegisterExportService(mux, store, cfg.Export.SigningKey, buildVersion(), opts, maxBytes)
	driftEngine := drift.NewEngine(store, nil)
	lintEngine := linter.NewEngine(store, nil)
	server.RegisterLifecycleService(mux, store, driftEngine, lintEngine, nil, opts, maxBytes)

	syncHandler := server.RegisterSyncService(mux, store, "", opts, maxBytes)
	runner := syncpkg.NewExecRunner()
	syncHandler.RegisterAdapter(syncpkg.NewBeadsAdapter(runner))
	syncHandler.RegisterAdapter(syncpkg.NewGitHubAdapter(runner, ""))

	server.RegisterAPIHandlers(mux, store, auth.RequireAuth(compositeStore))
	server.RegisterAuthHandlers(mux, compositeStore, auth.RequireAuth(compositeStore))

	// Mount MCP streamable HTTP endpoint.
	// TODO(auth): Derive profile from authenticated identity once MCP auth is implemented.
	// Currently profile is caller-supplied (header/query param). This is acceptable
	// because the MCP endpoint shares the same auth middleware as ConnectRPC — an
	// unauthenticated caller can't escalate beyond what the backend RPCs already enforce.
	mcpClient := mcppkg.NewClient(newHTTPClient(""), selfBaseURL(cfg.Server.Listen))
	mcpServer := mcppkg.NewServer(mcpClient)
	mux.Handle("/mcp/", http.StripPrefix("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		profileStr := r.Header.Get("X-Specgraph-MCP-Profile")
		if profileStr == "" {
			profileStr = r.URL.Query().Get("profile")
		}
		profile := mcppkg.ParseProfile(profileStr)
		mcpHandler := mcpServer.HTTPHandler(profile)
		mcpHandler.ServeHTTP(w, r)
	})))

	webFS, err := fs.Sub(web.Build, "build")
	if err != nil {
		return fmt.Errorf("embedded web FS: %w", err)
	}
	mux.Handle("/", server.StaticHandler(webFS))

	handler := server.SecurityHeaders(server.ProjectMiddleware(mux))

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

	server.StartSweeper(sweeperCtx, store, 60*time.Second, slog.Default())
	fmt.Printf("SpecGraph server running at http://%s\n", addr)
	slog.Info("MCP endpoint available", "path", "/mcp/")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

// selfBaseURL normalizes a listen address into a dialable HTTP base URL.
// Empty or wildcard hosts (e.g., ":8080", "0.0.0.0:8080") are replaced with 127.0.0.1.
func selfBaseURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://127.0.0.1"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
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
