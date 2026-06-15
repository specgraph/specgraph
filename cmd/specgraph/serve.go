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
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/server/probes"
	syncpkg "github.com/specgraph/specgraph/internal/sync"
	"github.com/specgraph/specgraph/internal/telemetry"
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
	serveCmd.Flags().String("pg-url", "", "PostgreSQL connection URL (overrides config; env: SPECGRAPH_SERVER_POSTGRES_URL)")
	serveCmd.Flags().String("listen", "", "Address to listen on (overrides config; env: SPECGRAPH_SERVER_LISTEN)")
	serveCmd.Flags().String("log-level", "", "Log level: debug, info, warn, error (overrides config; env: SPECGRAPH_LOG_LEVEL)")
	serveCmd.Flags().String("log-format", "", "Log format: json, text (overrides config; env: SPECGRAPH_LOG_FORMAT)")
	serveCmd.Flags().String("log-output", "", "Log output stream: stdout, stderr (overrides config; env: SPECGRAPH_LOG_OUTPUT)")
	rootCmd.AddCommand(serveCmd)
}

// appDeps holds everything constructed before (and independent of) the
// Postgres connection. Build failures here are fatal at startup, exactly as
// before this refactor: a broken binary, bad OIDC config, or bad CORS flag
// should fail fast on every boot regardless of database availability.
type appDeps struct {
	verifiers      []*auth.OIDCVerifier
	loginProviders []auth.LoginProvider
	authorizer     *auth.CedarAuthorizer
	knownRoles     map[auth.Role]bool
	skillsSrc      skills.Source
	webFS          fs.FS
	corsOrigin     string
}

func buildAppDeps(ctx context.Context, cfg *config.GlobalConfig, cmd *cobra.Command) (appDeps, error) {
	verifiers := make([]*auth.OIDCVerifier, 0, len(cfg.Auth.OIDC.Providers))
	for _, pc := range cfg.Auth.OIDC.Providers { //nolint:gocritic // rangeValCopy: provider list is small and startup-only
		issuerCtx, issuerCancel := context.WithTimeout(ctx, 10*time.Second)
		v, oidcErr := auth.NewOIDCVerifier(issuerCtx, pc)
		issuerCancel()
		if oidcErr != nil {
			return appDeps{}, fmt.Errorf("OIDC provider %s: %w", pc.ID, oidcErr)
		}
		verifiers = append(verifiers, v)
		slog.LogAttrs(context.Background(), slog.LevelInfo, "auth: OIDC provider configured",
			slog.String("id", pc.ID), slog.String("issuer", pc.Issuer))
	}

	loginProviders, lpErr := auth.BuildLoginProviders(ctx, cfg.Auth.OIDC.Providers, cfg.Auth.OIDC.BaseURL)
	if lpErr != nil {
		return appDeps{}, fmt.Errorf("OIDC login providers: %w", lpErr)
	}

	knownRoles := auth.KnownRolesFrom(cfg.Auth.Roles)

	policySources := []auth.PolicySource{auth.NewEmbeddedPolicySource()}
	for _, dir := range cfg.Auth.Policies.ExtraDirs {
		policySources = append(policySources, auth.NewDirectoryPolicySource(dir))
	}
	engine, err := auth.NewCedarEngine(ctx, policySources, auth.ActionNames())
	if err != nil {
		return appDeps{}, fmt.Errorf("policy engine: %w", err)
	}
	authorizer := auth.NewCedarAuthorizer(engine)

	skillsSrc, skillsErr := skills.NewEmbedded()
	if skillsErr != nil {
		return appDeps{}, fmt.Errorf("load embedded skills: %w", skillsErr)
	}

	webFS, err := fs.Sub(web.Build, "build")
	if err != nil {
		return appDeps{}, fmt.Errorf("embedded web FS: %w", err)
	}

	corsOrigin, err := cmd.Flags().GetString("cors-origin")
	if err != nil {
		return appDeps{}, fmt.Errorf("cors-origin flag: %w", err)
	}

	return appDeps{
		verifiers:      verifiers,
		loginProviders: loginProviders,
		authorizer:     authorizer,
		knownRoles:     knownRoles,
		skillsSrc:      skillsSrc,
		webFS:          webFS,
		corsOrigin:     corsOrigin,
	}, nil
}

// appHandler is the result of wiring the application once a live connection
// exists.
type appHandler struct {
	handler  http.Handler
	resolver auth.Resolver // for the post-listen no-auth warn
	cleanup  func()        // drains the usagetracker; call after server drain
}

func buildAppHandler(_ context.Context, cfg *config.GlobalConfig, deps *appDeps, res connectResult) (appHandler, error) {
	store := res.store // *postgres.Store satisfies the storage interfaces used below

	// usagetracker for async TouchLastUsed.
	tracker := usagetracker.NewManager(res.authStore, usagetracker.Config{
		BufferSize:    256,
		FlushInterval: 5 * time.Second,
	})
	cleanup := func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if trackerErr := tracker.Close(shutdownCtx); trackerErr != nil {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "usagetracker close", slog.Any("error", trackerErr))
		}
		if dropped := tracker.Dropped(); dropped > 0 {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "usagetracker dropped touches over session lifetime",
				slog.Uint64("count", dropped),
				slog.String("hint", "increase auth usagetracker BufferSize or decrease FlushInterval"))
		}
	}

	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:                   res.authStore,
		WebAuth:                 res.authStore,
		Verifiers:               deps.verifiers,
		Tracker:                 tracker,
		KnownRoles:              deps.knownRoles,
		JITEnabled:              cfg.Auth.OIDC.JITCreate.Enabled,
		JITDefaultRole:          auth.Role(cfg.Auth.OIDC.JITCreate.DefaultRole),
		JITClaimsMapping:        buildClaimsMappingByIssuer(cfg.Auth.OIDC.Providers),
		JITRateBurstPerHour:     cfg.Auth.OIDC.JITCreate.RateLimitPerHour,
		JITEmailDomainAllowlist: cfg.Auth.OIDC.JITCreate.EmailDomainAllowlist,
	})
	if err != nil {
		cleanup() // drain the tracker goroutine we just started
		return appHandler{}, fmt.Errorf("identity store: %w", err)
	}

	interceptor := auth.NewAuthInterceptor(resolver, deps.authorizer)
	maxBytes := connect.WithReadMaxBytes(4 << 20)
	interceptors := []connect.Interceptor{server.AccessLogInterceptor()} // outermost: always runs, sees final code
	otelIC, otelErr := telemetry.ServerInterceptor(telState.enabled)
	if otelErr != nil {
		return appHandler{}, fmt.Errorf("otel interceptor: %w", otelErr)
	}
	if otelIC != nil {
		interceptors = append(interceptors, otelIC) // before auth (AccessLogInterceptor is outermost)
	}
	interceptors = append(interceptors, interceptor) // existing auth interceptor
	opts := connect.WithInterceptors(interceptors...)

	mux := server.NewMux(store, opts, maxBytes)
	server.RegisterHealthService(mux, opts, maxBytes)
	server.RegisterDecisionService(mux, store, opts, maxBytes)
	server.RegisterGraphService(mux, store, opts, maxBytes)
	server.RegisterClaimService(mux, store, opts, maxBytes)
	server.RegisterConstitutionService(mux, store, opts, maxBytes)
	server.RegisterAuthoringService(mux, store, opts, maxBytes)
	server.RegisterAnalyticalPassService(mux, store, ".specgraph/templates", opts, maxBytes)
	server.RegisterExecutionService(mux, store, deps.skillsSrc, opts, maxBytes)
	server.RegisterSliceService(mux, store, opts, maxBytes)
	server.RegisterIdentityService(mux, res.authStore, opts, maxBytes)
	server.RegisterExportService(mux, store, cfg.Export.SigningKey, buildVersion(), opts, maxBytes)
	driftEngine := drift.NewEngine(store, nil)
	lintEngine := linter.NewEngine(store, nil)
	server.RegisterLifecycleService(mux, store, driftEngine, lintEngine, nil, opts, maxBytes)

	syncHandler := server.RegisterSyncService(mux, store, opts, maxBytes)
	runner := syncpkg.NewExecRunner()
	syncHandler.RegisterAdapter(syncpkg.NewBeadsAdapter(runner))
	syncHandler.RegisterAdapter(syncpkg.NewGitHubAdapter(runner, ""))

	server.RegisterAPIHandlers(mux, store, auth.RequireAuth(resolver))
	server.RegisterAuthHandlers(mux, resolver, res.authStore, auth.RequireAuth(resolver))
	server.RegisterOIDCLoginHandlers(mux, server.OIDCLoginConfig{
		Providers:       deps.loginProviders,
		Resolver:        resolver,
		WebAuth:         res.authStore,
		BaseURL:         cfg.Auth.OIDC.BaseURL,
		SessionTTL:      cfg.Auth.OIDC.SessionTTL,
		FlowTTL:         5 * time.Minute,
		Limiter:         server.NewIPRateLimiterForOIDC(cfg.Server.TrustedProxy),
		CLILoginEnabled: cfg.Auth.OIDC.CLILoginEnabled,
	})

	loopbackClient := newHTTPClient("")
	loopbackClient.Transport = telemetry.LoopbackTransport(telState.enabled, loopbackClient.Transport)
	mcpClient := mcppkg.NewClient(loopbackClient, selfBaseURL(cfg.Server.Listen))
	mcpSrv := mcppkg.NewServer(mcpClient)
	mcpHTTPHandler := mcpSrv.HTTPHandler(
		mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			if v := r.Header.Get("Authorization"); v != "" {
				scheme, token, ok := strings.Cut(v, " ")
				token = strings.TrimSpace(token)
				if ok && strings.EqualFold(scheme, "Bearer") && token != "" {
					ctx = auth.WithBearerToken(ctx, token)
				}
			}
			if slug := r.Header.Get("X-Specgraph-Project"); slug != "" {
				ctx = auth.WithProject(ctx, slug)
			}
			return ctx
		}),
	)
	mux.Handle("/mcp/", mcpHeaderLogger(auth.RequireAuth(resolver)(
		http.StripPrefix("/mcp", mcpHTTPHandler),
	)))

	mux.Handle("/", server.StaticHandler(deps.webFS))

	var rootHandler http.Handler = mux
	if cfg.Log.Requests {
		rootHandler = server.AccessLog(mux)
	}
	handler := server.SecurityHeaders(server.ProjectMiddleware(rootHandler))
	if telState.enabled {
		handler = telemetry.WrapHTTPHandler(handler)
	}
	if deps.corsOrigin != "" {
		handler = server.CORSMiddleware(deps.corsOrigin, handler)
	}

	return appHandler{handler: handler, resolver: resolver, cleanup: cleanup}, nil
}

// startMainServer runs srv.Serve in a goroutine and returns a channel that
// fires once if Serve returns a non-ErrServerClosed error, so the caller can
// trigger shutdown (a dead main listener is unrecoverable).
func startMainServer(srv *http.Server, ln net.Listener) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		serveErr := srv.Serve(ln)
		if serveErr == nil || errors.Is(serveErr, http.ErrServerClosed) {
			return
		}
		errCh <- serveErr
	}()
	return errCh
}

func runServe(cmd *cobra.Command, _ []string) error {
	startupBegin := time.Now()
	if os.Getenv("SPECGRAPH_PG_URL") != "" {
		slog.LogAttrs(context.Background(), slog.LevelWarn, "SPECGRAPH_PG_URL is no longer read; use SPECGRAPH_SERVER_POSTGRES_URL")
	}

	cfg, err := loadGlobalCfg(config.WithFlags(cmd.Flags()))
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}

	// Initialise structured logging as early as possible so every subsequent
	// slog call in this process uses the configured handler. All handlers
	// downstream capture slog.Default() at construction time, so this must
	// happen before any service is registered.
	logger, err := cfg.Log.Build()
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	if telState.tel != nil {
		// Layer enrichment (trace_id/span_id/project/identity) + OTLP log
		// export over the server's CONFIGURED handler, honoring cfg.Log knobs
		// (--log-format/--log-level/--log-output). When telemetry is disabled,
		// NewLogger returns the plain configured logger unchanged.
		slog.SetDefault(telState.tel.NewLogger(logger.Handler()))
	} else {
		slog.SetDefault(logger)
	}
	slog.LogAttrs(context.Background(), slog.LevelInfo, "specgraph server starting",
		slog.String("version", buildVersion()),
		slog.String("listen", cfg.Server.Listen),
		slog.String("log_level", cfg.Log.Level),
		slog.String("log_format", cfg.Log.Format),
		slog.String("log_output", cfg.Log.Output),
	)

	ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// --pg-url / SPECGRAPH_SERVER_POSTGRES_URL and backend coercion are now
	// resolved inside the loader (cfg.Server.Postgres.URL, cfg.Server.Backend).

	if cfg.Server.Docker {
		composeFile, dockerErr := docker.EnsureComposeFile(xdg.DataHome())
		if dockerErr != nil {
			return dockerErr
		}
		slog.LogAttrs(context.Background(), slog.LevelInfo, "starting Docker Compose stack")
		if upErr := docker.ComposeUp(composeFile); upErr != nil {
			return upErr
		}
		defer func() {
			if stopErr := docker.ComposeStop(composeFile); stopErr != nil {
				slog.LogAttrs(context.Background(), slog.LevelWarn, "compose stop", slog.Any("error", stopErr))
			}
		}()
	}

	// Create backend store.
	connURL := cfg.Server.Postgres.URL
	if connURL == "" {
		return fmt.Errorf("postgres backend requires a connection URL (set server.postgres.url in config or use --pg-url)")
	}

	// PG-independent construction is fatal up front (unchanged behavior).
	deps, err := buildAppDeps(ctx, cfg, cmd)
	if err != nil {
		return err
	}

	probesCfg, err := validateServerConfig(cfg)
	if err != nil {
		return err
	}

	pol := defaultBackoffPolicy()

	// Bind the probe listener BEFORE the first (potentially slow) Postgres
	// connect. Liveness must never wait on storage: /livez answers 200 from
	// t≈0 and /readyz is a truthful 503 while we connect, so a kubelet liveness
	// probe can't kill the pod during a degraded boot. The dial is bounded
	// (postgres.defaultConnectTimeout), but even a bounded stall must not delay
	// the port binding past the liveness window. The readiness probe LOOP is
	// started later (probeHandler.Start, after the path decision) so a healthy
	// boot stays log-silent.
	probeHandler := probes.NewHandler()
	probeSrv, probeErrCh, err := startProbeListener(ctx, probeHandler, probesCfg)
	if err != nil {
		return err
	}
	// stopProbe shuts the probe listener down on a fatal early return that
	// happens before the shutdown goroutine (which otherwise owns it) is wired.
	stopProbe := func() {
		if probeSrv == nil {
			return
		}
		shutCtx, c := context.WithTimeout(context.Background(), probeShutdownTimeout)
		defer c()
		if shErr := probeSrv.Shutdown(shutCtx); shErr != nil {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "probe server shutdown", slog.Any("error", shErr))
		}
	}

	// Decide the boot path BEFORE starting any server or shutdown goroutine, so
	// a fatal docker:true / credential failure starts nothing (unchanged).
	var (
		res      connectResult
		haveConn bool
	)
	if cfg.Server.Docker {
		r, connErr := connectStore(ctx, connURL)
		if connErr != nil {
			stopProbe()
			return fmt.Errorf("connect to postgres: %w", connErr)
		}
		res, haveConn = r, true
	} else {
		r, degrade, connErr := firstConnect(ctx, connURL, pol)
		if connErr != nil {
			stopProbe()
			return connErr // credential-exhausted fatal, or ctx cancelled
		}
		if !degrade {
			res, haveConn = r, true
		} else {
			slog.LogAttrs(context.Background(), slog.LevelWarn,
				"postgres unavailable at startup; serving 503 until it connects",
				slog.String("listen", cfg.Server.Listen))
			if probesCfg.Listen == "" {
				slog.LogAttrs(context.Background(), slog.LevelWarn,
					"readiness is not observable: set server.probes.listen to expose /readyz")
			}
		}
	}

	// Shared activation: wires the app once a live connection exists, swaps in
	// the real handler, publishes the store to the probe pinger, prints the
	// bootstrap banner, and starts the sweeper. Returns the post-drain cleanup.
	pinger := newReadinessPinger()
	handlerRef := newAtomicHandler(notReadyHandler())
	addr := cfg.Server.Listen

	sweeperCtx, stopSweeper := context.WithCancel(ctx)
	var (
		cleanupMu sync.Mutex
		cleanup   func() // store/auth/tracker teardown; nil until activation
	)

	// closeStores releases the auth store (borrows the pool) then the store
	// (owns the pool), logging any error. Shared by the activation cleanup and
	// the shutdown-before-activation paths so a connection is never leaked when
	// wiring fails or shutdown races a late connect.
	closeStores := func(r connectResult) {
		if closeErr := r.authStore.Close(context.Background()); closeErr != nil {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "auth store close", slog.Any("error", closeErr))
		}
		if closeErr := r.store.Close(context.Background()); closeErr != nil {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "close store", slog.Any("error", closeErr))
		}
	}

	// runCleanup invokes the registered post-drain teardown if activation has
	// happened; it is a no-op before activation (cleanup is nil).
	runCleanup := func() {
		cleanupMu.Lock()
		cl := cleanup
		cleanupMu.Unlock()
		if cl != nil {
			cl()
		}
	}

	activate := func(r connectResult) error {
		// If shutdown began before we could wire this connection, release it
		// rather than opening a store/tracker that nothing will close.
		select {
		case <-ctx.Done():
			closeStores(r)
			return nil
		default:
		}
		// Print the one-time bootstrap banner immediately on a successful
		// connect — BEFORE the fallible post-connect wiring below — so a later
		// failure (e.g. an invalid identity-store config in buildAppHandler)
		// can never strand the admin token, which bootstrap.Ensure has already
		// minted and committed inside connectStore.
		if r.bootstrap.Created {
			fmt.Fprintf(os.Stderr,
				"\n========================================================================\n"+
					"SpecGraph created a bootstrap admin on first start.\n"+
					"  API key: %s\n"+
					"  Server:  http://%s\n"+
					"Copy this key into your CLI credentials now. Rotate it after you\n"+
					"configure OIDC. This key is shown ONCE and will not be displayed again.\n"+
					"========================================================================\n\n",
				r.bootstrap.Token, addr)
		}
		ah, buildErr := buildAppHandler(ctx, cfg, &deps, r)
		if buildErr != nil {
			closeStores(r)
			return buildErr
		}

		// Swap in the real handler BEFORE marking readiness healthy, so a
		// reconnect can never report ready (/readyz 200) while the main port
		// is still serving the blanket 503 handler.
		handlerRef.set(ah.handler)
		pinger.set(r.store)

		if hasAuth, hasAuthErr := ah.resolver.HasAuth(ctx); hasAuthErr != nil {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "auth: HasAuth check failed at startup", slog.Any("error", hasAuthErr))
		} else if !hasAuth {
			warnIfNoAuthOnPublicListen(cfg.Server.Listen, false)
		}

		server.StartSweeper(sweeperCtx, r.store, 60*time.Second)
		server.StartWebAuthSweeper(sweeperCtx, r.authStore, 60*time.Second)

		cleanupMu.Lock()
		cleanup = func() {
			// Post-drain teardown order (sweeper already stopped):
			// tracker → authStore → store.
			ah.cleanup()
			closeStores(r)
		}
		cleanupMu.Unlock()
		slog.LogAttrs(context.Background(), slog.LevelInfo, "storage connected; serving")
		return nil
	}

	// Happy path: wire and swap in the real handler BEFORE opening the main
	// port, so clients never observe a 503 window. activate also sets the
	// pinger's store, so the probe loop's first probe (started below) is
	// healthy (no spurious WARN). The probe listener is already bound, so a
	// failure here must stop it before returning.
	if haveConn {
		if actErr := activate(res); actErr != nil {
			stopSweeper()
			stopProbe()
			return actErr
		}
	}

	// Open the main port. On the happy path handlerRef already holds the real
	// handler; in the degraded window it holds the blanket-503 handler.
	mainLn, err := net.Listen("tcp", addr)
	if err != nil {
		stopSweeper()
		stopProbe()
		runCleanup()
		return fmt.Errorf("main listener bind %s: %w", addr, err)
	}
	srv := &http.Server{
		Handler:           handlerRef,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Now start the readiness probe loop. The listener was bound up front; on
	// the happy path the pinger's store is already set (silent first probe), on
	// the degraded path it is nil (first probe legitimately fails → one WARN).
	// Skip when probes are disabled (no listener) to preserve "probes off → no
	// probe goroutine, readiness not observable".
	if probeSrv != nil {
		probeHandler.Start(ctx, pinger, probesCfg.Interval, probesCfg.Timeout)
	}
	mainErrCh := startMainServer(srv, mainLn)

	// fatalErr is published by the shutdown goroutine; read after <-done.
	var fatalErr error
	wiringFatalCh := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
		case e := <-probeErrCh:
			slog.LogAttrs(context.Background(), slog.LevelError, "probe listener died; triggering shutdown", slog.Any("error", e))
			cancel()
		case e := <-mainErrCh:
			slog.LogAttrs(context.Background(), slog.LevelError, "main listener died; triggering shutdown", slog.Any("error", e))
			fatalErr = e
			cancel()
		case e := <-wiringFatalCh:
			slog.LogAttrs(context.Background(), slog.LevelError, "fatal wiring error; triggering shutdown", slog.Any("error", e))
			fatalErr = e
			cancel()
		}
		slog.LogAttrs(context.Background(), slog.LevelInfo, "server shutting down")
		stopSweeper()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			mainCtx, c := context.WithTimeout(context.Background(), 15*time.Second)
			defer c()
			if shErr := srv.Shutdown(mainCtx); shErr != nil {
				slog.LogAttrs(context.Background(), slog.LevelWarn, "main server shutdown", slog.Any("error", shErr))
			}
		}()
		if probeSrv != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				probeCtx, c := context.WithTimeout(context.Background(), probeShutdownTimeout)
				defer c()
				if shErr := probeSrv.Shutdown(probeCtx); shErr != nil {
					slog.LogAttrs(context.Background(), slog.LevelWarn, "probe server shutdown", slog.Any("error", shErr))
				}
			}()
		}
		wg.Wait()

		// Post-drain store/auth/tracker teardown (nil until activation).
		runCleanup()

		// Primary telemetry flush after the servers drain. run()'s deferred
		// Shutdown is a safe, concurrency-serialized no-op afterward.
		if telState.tel != nil {
			shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
			defer c()
			if shErr := telState.tel.Shutdown(shutdownCtx); shErr != nil {
				slog.LogAttrs(context.Background(), slog.LevelWarn, "telemetry shutdown", slog.Any("error", shErr))
			}
		}
	}()

	slog.LogAttrs(context.Background(), slog.LevelInfo, "server listening", slog.String("addr", "http://"+addr))
	slog.LogAttrs(context.Background(), slog.LevelInfo, "MCP endpoint available", slog.String("path", "/mcp/"))

	if telState.tel != nil {
		telemetry.RecordStartup(ctx, time.Since(startupBegin))
	}

	// Degraded path: retry in the background; activate on first success. The
	// ports are already open (serving 503) before this goroutine starts.
	if !haveConn {
		go func() {
			r, connErr := runConnector(ctx, func(c context.Context) (connectResult, error) {
				return connectStore(c, connURL)
			}, pol, ctxSleep)
			if connErr != nil {
				if ctx.Err() == nil { // genuine fatal (credential exhaustion), not shutdown
					wiringFatalCh <- connErr
				}
				return
			}
			if actErr := activate(r); actErr != nil {
				wiringFatalCh <- actErr
			}
		}()
	}

	<-done
	return fatalErr
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
	slog.LogAttrs(context.Background(), slog.LevelWarn, "server listening without authentication on non-loopback interface",
		slog.String("addr", listen),
		slog.String("risk", "configure API keys or OIDC providers"),
	)
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

// startProbeListener binds a plain-HTTP listener serving the handler's /livez
// and /readyz on the resolved ProbesConfig address. Returns (nil, nil, nil)
// when Listen is empty so callers treat probes as off. Bind errors abort
// startup rather than leaving a running API that kubelet can never mark ready.
//
// The listener is bound but the readiness probe loop is NOT started here — the
// caller owns that (handler.Start) so it can bring liveness up early (before
// the dependency connects) yet defer readiness probing until the dependency
// wiring is ready, keeping a healthy boot log-silent.
//
// The returned error channel fires once if the background Serve returns a
// non-ErrServerClosed error — a dead probe listener while the main server
// keeps serving traffic is a silent split-brain that kubelet can't recover
// from on its own, so the caller should treat a send on this channel as a
// shutdown trigger.
func startProbeListener(ctx context.Context, handler *probes.Handler, cfg config.ProbesConfig) (*http.Server, <-chan error, error) {
	if cfg.Listen == "" {
		return nil, nil, nil
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return nil, nil, fmt.Errorf("probe listener bind %s: %w", cfg.Listen, err)
	}
	probeHandler := handler.Mux()
	if cfg.LogRequests {
		probeHandler = server.AccessLog(probeHandler)
	}
	srv := &http.Server{
		// Addr reflects the resolved listener address, not the caller's
		// input, so callers passing ":0" can observe the ephemeral port.
		Addr:              ln.Addr().String(),
		Handler:           probeHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.LogAttrs(ctx, slog.LevelInfo, "probe endpoints listening",
		slog.String("addr", srv.Addr),
		slog.String("livez", "/livez"),
		slog.String("readyz", "/readyz"),
	)
	errCh := make(chan error, 1)
	go func() {
		serveErr := srv.Serve(ln)
		if serveErr == nil || errors.Is(serveErr, http.ErrServerClosed) {
			return
		}
		slog.LogAttrs(ctx, slog.LevelError, "probe server failed",
			slog.String("addr", srv.Addr),
			slog.Any("error", serveErr),
		)
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
		slog.LogAttrs(r.Context(), slog.LevelDebug, "mcp: inbound request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("raw_query", r.URL.RawQuery),
			slog.Any("header_keys", keys),
		)
		next.ServeHTTP(w, r)
	})
}

// buildClaimsMappingByIssuer converts a slice of OIDCProviderConfig into a
// map keyed by issuer URL, where each value is the provider's ClaimsMapping
// slice. Consumed by auth.IdentityStoreConfig.JITClaimsMapping at startup.
func buildClaimsMappingByIssuer(providers []config.OIDCProviderConfig) map[string][]config.ClaimMapping {
	out := make(map[string][]config.ClaimMapping, len(providers))
	for _, pc := range providers { //nolint:gocritic // rangeValCopy: provider list is small and startup-only
		if len(pc.ClaimsMapping) > 0 {
			out[pc.Issuer] = pc.ClaimsMapping
		}
	}
	return out
}
