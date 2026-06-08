//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

// reservePort binds :0, records the address, releases it, and returns the
// host:port. There is an inherent TOCTOU window before something rebinds it;
// acceptable for a local integration test.
func reservePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

// tcpProxy forwards each connection on listenAddr to dialAddr until ctx ends.
func tcpProxy(t *testing.T, ctx context.Context, listenAddr, dialAddr string) {
	t.Helper()
	ln, err := net.Listen("tcp", listenAddr)
	require.NoError(t, err)
	go func() { <-ctx.Done(); _ = ln.Close() }()
	go func() {
		for {
			client, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				up, derr := net.Dial("tcp", dialAddr)
				if derr != nil {
					return
				}
				defer up.Close()
				go func() { _, _ = io.Copy(up, c) }()
				_, _ = io.Copy(c, up)
			}(client)
		}
	}()
}

// newServeCmd builds a *cobra.Command carrying the serve flag set and ctx, so
// runServe can be invoked directly. Values come from env (see configureServeEnv);
// flags default empty so config.WithFlags lets env win.
func newServeCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{Use: "serve", RunE: runServe}
	cmd.Flags().String("cors-origin", "", "")
	cmd.Flags().String("pg-url", "", "")
	cmd.Flags().String("listen", "", "")
	cmd.Flags().String("log-level", "", "")
	cmd.Flags().String("log-format", "", "")
	cmd.Flags().String("log-output", "", "")
	cmd.SetContext(ctx)
	return cmd
}

// configureServeEnv points loadGlobalCfg at an empty temp config and forces
// docker:false + the given addresses via SPECGRAPH_* env (t.Setenv restores).
func configureServeEnv(t *testing.T, pgURL, mainAddr, probeAddr string) {
	t.Helper()
	empty := t.TempDir() + "/config.yaml"
	require.NoError(t, os.WriteFile(empty, []byte("{}\n"), 0o600))
	old := cfgFile
	cfgFile = empty
	t.Cleanup(func() { cfgFile = old })

	t.Setenv("SPECGRAPH_SERVER_DOCKER", "false")
	t.Setenv("SPECGRAPH_SERVER_POSTGRES_URL", pgURL)
	t.Setenv("SPECGRAPH_SERVER_LISTEN", mainAddr)
	t.Setenv("SPECGRAPH_SERVER_PROBES_LISTEN", probeAddr)
	t.Setenv("SPECGRAPH_SERVER_PROBES_INTERVAL", "100ms")
	t.Setenv("SPECGRAPH_SERVER_PROBES_TIMEOUT", "100ms")
}

func httpStatus(url string) (int, bool) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url) //nolint:noctx // short-timeout test probe
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	return resp.StatusCode, true
}

func TestServe_DegradedThenRecovers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	realCS, err := postgrestest.ConnString(ctx)
	require.NoError(t, err)
	// realCS == "postgres://test:test@HOST:PORT/testdb"
	realHostPort := realCS[strings.LastIndex(realCS, "@")+1 : strings.LastIndex(realCS, "/")]

	frontAddr := reservePort(t) // server connects here: dead until the proxy starts
	mainAddr := reservePort(t)
	probeAddr := reservePort(t)
	frontCS := "postgres://test:test@" + frontAddr + "/testdb"

	configureServeEnv(t, frontCS, mainAddr, probeAddr)
	cmd := newServeCmd(ctx)

	serveErr := make(chan error, 1)
	go func() { serveErr <- runServe(cmd, nil) }()

	// Degraded: main port 503, /readyz 503.
	require.Eventually(t, func() bool {
		code, ok := httpStatus("http://" + mainAddr + "/")
		return ok && code == http.StatusServiceUnavailable
	}, 10*time.Second, 100*time.Millisecond, "main port must serve 503 while PG is down")
	code, ok := httpStatus("http://" + probeAddr + "/readyz")
	require.True(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, code)

	// Bring PG up at the front address → server self-heals.
	tcpProxy(t, ctx, frontAddr, realHostPort)
	require.Eventually(t, func() bool {
		c, ok := httpStatus("http://" + probeAddr + "/readyz")
		return ok && c == http.StatusOK
	}, 40*time.Second, 250*time.Millisecond, "/readyz must flip to 200 once PG is reachable")

	select {
	case e := <-serveErr:
		t.Fatalf("server exited prematurely: %v", e)
	default:
	}

	cancel()
	select {
	case <-serveErr:
	case <-time.After(20 * time.Second):
		t.Fatal("server did not shut down after cancel")
	}
}

func TestServe_CredentialFailureIsFatal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	realCS, err := postgrestest.ConnString(ctx)
	require.NoError(t, err)
	badCS := strings.Replace(realCS, "test:test@", "test:wrongpassword@", 1)

	configureServeEnv(t, badCS, reservePort(t), reservePort(t))
	cmd := newServeCmd(ctx)

	done := make(chan error, 1)
	go func() { done <- runServe(cmd, nil) }()

	select {
	case e := <-done:
		require.Error(t, e)
		assert.Contains(t, e.Error(), "credentials rejected")
	case <-time.After(15 * time.Second):
		t.Fatal("expected credential failure to be fatal within ~5 retries")
	}
}
