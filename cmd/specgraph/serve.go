// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/docker"
	"github.com/specgraph/specgraph/internal/drift"
	"github.com/specgraph/specgraph/internal/linter"
	mcppkg "github.com/specgraph/specgraph/internal/mcp"
	"github.com/specgraph/specgraph/internal/mcp/skills"
	"github.com/specgraph/specgraph/internal/notify"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/server/probes"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	syncpkg "github.com/specgraph/specgraph/internal/sync"
	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/specgraph/specgraph/web"
	"github.com/spf13/cobra"
)

// probeShutdownTimeout bounds graceful drain of the probe listener,
// independent from the main server's budget so a slow main-server drain
// can't starve the probe server's shutdown.
const probeShutdownTimeout = 15 * time.Second

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
	cfg, err := loadGlobalCfg()
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
			if stopErr := docker.ComposeStop(composeFile); stopErr != nil {
				fmt.Fprintf(os.Stderr, "warning: compose stop: %v\n", stopErr)
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

	// Build the auth store via the existing postgres.Store.Pool() accessor.
	// s is the concrete *postgres.Store from which Pool() is accessible;
	// store is the narrower backendStore interface used everywhere else.
	authStore, err := postgres.NewAuth(ctx, s.Pool())
	if err != nil {
		return fmt.Errorf("auth store: %w", err)
	}
	defer func() {
		if closeErr := authStore.Close(ctx); closeErr != nil {
			slog.Warn("auth store close", "error", closeErr)
		}
	}()

	// Construct OIDC verifiers (one per provider).
	// Iterate cfg.Auth.OIDC.Providers — the canonical field. The config loader
	// copies any legacy cfg.Auth.OIDCProviders entries into this field, so
	// reading the legacy field here would yield ZERO verifiers for any config
	// that uses the auth.oidc.providers shape (silently breaking all JWT/JIT
	// auth). Must match the claims-mapping source below.
	verifiers := make([]*auth.OIDCVerifier, 0, len(cfg.Auth.OIDC.Providers))
	for _, pc := range cfg.Auth.OIDC.Providers {
		issuerCtx, issuerCancel := context.WithTimeout(ctx, 10*time.Second)
		v, oidcErr := auth.NewOIDCVerifier(issuerCtx, pc)
		issuerCancel()
		if oidcErr != nil {
			return fmt.Errorf("OIDC provider %s: %w", pc.ID, oidcErr)
		}
		verifiers = append(verifiers, v)
		slog.Info("auth: OIDC provider configured", "id", pc.ID, "issuer", pc.Issuer)
	}

	// usagetracker for async TouchLastUsed.
	tracker := usagetracker.NewManager(authStore, usagetracker.Config{
		BufferSize:    256,
		FlushInterval: 5 * time.Second,
	})
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if trackerErr := tracker.Close(shutdownCtx); trackerErr != nil {
			slog.Warn("usagetracker close", "error", trackerErr)
		}
		if dropped := tracker.Dropped(); dropped > 0 {
			slog.Warn("usagetracker dropped touches over session lifetime",
				"count", dropped,
				"hint", "increase auth usagetracker BufferSize or decrease FlushInterval")
		}
	}()

	// KnownRoles for JIT validation (built-ins ∪ custom role names). Under
	// Cedar, custom roles carry no permission list; their authorization is
	// expressed as Cedar policies, not YAML.
	knownRoles := auth.KnownRolesFrom(cfg.Auth.Roles)

	// Build the Resolver (pgIdentityStore backed by Postgres).
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:                   authStore,
		Verifiers:               verifiers,
		Tracker:                 tracker,
		KnownRoles:              knownRoles,
		JITEnabled:              cfg.Auth.OIDC.JITCreate.Enabled,
		JITDefaultRole:          auth.Role(cfg.Auth.OIDC.JITCreate.DefaultRole),
		JITClaimsMapping:        buildClaimsMappingByIssuer(cfg.Auth.OIDC.Providers),
		JITRateBurstPerHour:     cfg.Auth.OIDC.JITCreate.RateLimitPerHour,
		JITEmailDomainAllowlist: cfg.Auth.OIDC.JITCreate.EmailDomainAllowlist,
	})
	if err != nil {
		return fmt.Errorf("identity store: %w", err)
	}

	// Authorizer: Cedar policy engine. Built-in policies are always loaded;
	// operators add directories via auth.policies.extra_dirs.
	policySources := []auth.PolicySource{auth.NewEmbeddedPolicySource()}
	for _, dir := range cfg.Auth.Policies.ExtraDirs {
		policySources = append(policySources, auth.NewDirectoryPolicySource(dir))
	}
	engine, err := auth.NewCedarEngine(ctx, policySources, auth.ActionNames())
	if err != nil {
		return fmt.Errorf("policy engine: %w", err)
	}
	authorizer := auth.NewCedarAuthorizer(engine)

	interceptor := auth.NewAuthInterceptor(resolver, authorizer)

	// HasAuth signal for the existing warn path.
	hasAuth, hasAuthErr := resolver.HasAuth(ctx)
	if hasAuthErr != nil {
		slog.Warn("auth: HasAuth check failed at startup", "error", hasAuthErr)
	}
	if !hasAuth {
		warnIfNoAuthOnPublicListen(cfg.Server.Listen, false)
	}
	maxBytes := connect.WithReadMaxBytes(4 << 20) // 4 MiB request body limit
	opts := connect.WithInterceptors(interceptor)

	// Load embedded skills catalog once for the lifetime of the server.
	// The catalog is compiled into the binary; a parse failure means the
	// binary is broken, so fail fast here.
	skillsSrc, skillsErr := skills.NewEmbedded()
	if skillsErr != nil {
		return fmt.Errorf("load embedded skills: %w", skillsErr)
	}

	mux := server.NewMux(store, opts, maxBytes)
	server.RegisterHealthService(mux, opts, maxBytes)
	server.RegisterDecisionService(mux, store, opts, maxBytes)
	server.RegisterGraphService(mux, store, opts, maxBytes)
	server.RegisterClaimService(mux, store, opts, maxBytes)
	server.RegisterConstitutionService(mux, store, opts, maxBytes)
	server.RegisterAuthoringService(mux, store, opts, maxBytes)
	server.RegisterAnalyticalPassService(mux, store, ".specgraph/templates", opts, maxBytes)
	server.RegisterExecutionService(mux, store, skillsSrc, opts, maxBytes)
	server.RegisterSliceService(mux, store, opts, maxBytes)
	server.RegisterExportService(mux, store, cfg.Export.SigningKey, buildVersion(), opts, maxBytes)
	driftEngine := drift.NewEngine(store, nil)
	lintEngine := linter.NewEngine(store, nil)
	server.RegisterLifecycleService(mux, store, driftEngine, lintEngine, nil, opts, maxBytes)

	syncHandler := server.RegisterSyncService(mux, store, opts, maxBytes)
	runner := syncpkg.NewExecRunner()
	syncHandler.RegisterAdapter(syncpkg.NewBeadsAdapter(runner))
	syncHandler.RegisterAdapter(syncpkg.NewGitHubAdapter(runner, ""))

	server.RegisterAPIHandlers(mux, store, auth.RequireAuth(resolver))
	server.RegisterAuthHandlers(mux, resolver, auth.RequireAuth(resolver))

	// Mount MCP streamable HTTP endpoint with auth gating.
	// RequireAuth returns 401 for unauthenticated callers, which is the
	// MCP spec's signal for clients to initiate OAuth.
	mcpClient := mcppkg.NewClient(newHTTPClient(""), selfBaseURL(cfg.Server.Listen))
	mcpSrv := mcppkg.NewServer(mcpClient)
	mcpHTTPHandler := mcpSrv.HTTPHandler(
		mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			// Forward the raw bearer token into context so loopback
			// requests carry the caller's credentials. Mirror
			// RequireAuth's case-insensitive scheme parsing.
			if v := r.Header.Get("Authorization"); v != "" {
				scheme, token, ok := strings.Cut(v, " ")
				token = strings.TrimSpace(token)
				if ok && strings.EqualFold(scheme, "Bearer") && token != "" {
					ctx = auth.WithBearerToken(ctx, token)
				}
			}
			// Forward the project slug into context so loopback requests
			// to project-scoped RPC services satisfy the project middleware
			// without the MCP loopback client knowing the project ahead
			// of time.
			if slug := r.Header.Get("X-Specgraph-Project"); slug != "" {
				ctx = auth.WithProject(ctx, slug)
			}
			return ctx
		}),
	)
	mux.Handle("/mcp/", mcpHeaderLogger(auth.RequireAuth(resolver)(
		http.StripPrefix("/mcp", mcpHTTPHandler),
	)))

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

	probesCfg, err := validateServerConfig(cfg)
	if err != nil {
		return err
	}
	probeSrv, probeErrCh, err := startProbeListener(ctx, s, probesCfg)
	if err != nil {
		return err
	}

	go func() {
		select {
		case <-ctx.Done():
		case probeErr := <-probeErrCh:
			slog.Error("probe listener died; triggering shutdown", "error", probeErr)
			cancel()
		}
		fmt.Println("\nShutting down...")
		stopSweeper()
		// Shut down main and probe servers in parallel so a slow main-server
		// drain can't starve the probe server's graceful close (and vice versa).
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			mainCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := srv.Shutdown(mainCtx); err != nil {
				slog.Warn("main server shutdown", "error", err)
			}
		}()
		if probeSrv != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				probeCtx, cancel := context.WithTimeout(context.Background(), probeShutdownTimeout)
				defer cancel()
				if err := probeSrv.Shutdown(probeCtx); err != nil {
					slog.Warn("probe server shutdown", "error", err)
				}
			}()
		}
		wg.Wait()
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

// warnIfNoAuthOnPublicListen logs a single WARN line when the server is
// bound to a non-loopback interface with no authentication configured —
// the unconfigured default is reachable without credentials, and this
// warning is the only boot-log signal operators get.
func warnIfNoAuthOnPublicListen(listen string, hasAuth bool) {
	if hasAuth || isLoopbackAddr(listen) {
		return
	}
	slog.Warn("server listening without authentication on non-loopback interface",
		"addr", listen,
		"risk", "configure API keys or OIDC providers")
}

// validateServerConfig runs cross-section validation the main serve path
// depends on. Extracted so tests can exercise validation failures without
// spinning up the full runServe stack (config load, docker compose, auth
// composite store, …). Returns the resolved ProbesConfig so the caller
// doesn't re-resolve.
func validateServerConfig(cfg *config.GlobalConfig) (config.ProbesConfig, error) {
	probesCfg, err := cfg.Server.Probes.Resolved()
	if err != nil {
		return config.ProbesConfig{}, fmt.Errorf("invalid probes config: %w", err)
	}
	return probesCfg, nil
}

// startProbeListener binds a plain-HTTP listener serving /livez and /readyz
// using tuning from a resolved ProbesConfig. Returns (nil, nil, nil) when
// Listen is empty so callers treat probes as off. Bind errors abort startup
// rather than leaving a running API that kubelet can never mark ready.
//
// The returned error channel fires once if the background Serve returns a
// non-ErrServerClosed error — a dead probe listener while the main server
// keeps serving traffic is a silent split-brain that kubelet can't recover
// from on its own, so the caller should treat a send on this channel as a
// shutdown trigger.
func startProbeListener(ctx context.Context, pinger probes.Pinger, cfg config.ProbesConfig) (*http.Server, <-chan error, error) {
	if cfg.Listen == "" {
		return nil, nil, nil
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return nil, nil, fmt.Errorf("probe listener bind %s: %w", cfg.Listen, err)
	}
	h := probes.New(ctx, pinger, cfg.Interval, cfg.Timeout)
	srv := &http.Server{
		// Addr reflects the resolved listener address, not the caller's
		// input, so callers passing ":0" can observe the ephemeral port.
		Addr:              ln.Addr().String(),
		Handler:           h.Mux(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("probe endpoints listening", "addr", srv.Addr, "livez", "/livez", "readyz", "/readyz")
	errCh := make(chan error, 1)
	go func() {
		serveErr := srv.Serve(ln)
		if serveErr == nil || errors.Is(serveErr, http.ErrServerClosed) {
			return
		}
		slog.Error("probe server failed", "addr", srv.Addr, "error", serveErr)
		errCh <- serveErr
	}()
	return srv, errCh, nil
}

// mcpHeaderLogger logs the names of every inbound header on the /mcp/ endpoint
// for debugging client behavior. Logs only header names, not values, to avoid
// leaking bearer tokens or other sensitive content.
func mcpHeaderLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys := make([]string, 0, len(r.Header))
		for k := range r.Header {
			keys = append(keys, k)
		}
		slog.Debug("mcp: inbound request",
			"method", r.Method,
			"path", r.URL.Path,
			"raw_query", r.URL.RawQuery,
			"header_keys", keys,
		)
		next.ServeHTTP(w, r)
	})
}

// buildClaimsMappingByIssuer converts a slice of OIDCProviderConfig into a
// map keyed by issuer URL, where each value is the provider's ClaimsMapping
// slice. Consumed by auth.IdentityStoreConfig.JITClaimsMapping at startup.
func buildClaimsMappingByIssuer(providers []config.OIDCProviderConfig) map[string][]config.ClaimMapping {
	out := make(map[string][]config.ClaimMapping, len(providers))
	for _, pc := range providers {
		if len(pc.ClaimsMapping) > 0 {
			out[pc.Issuer] = pc.ClaimsMapping
		}
	}
	return out
}
