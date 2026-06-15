// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/browser"
	"github.com/specgraph/specgraph/internal/credentials"
	"github.com/specgraph/specgraph/internal/xdg"
)

var (
	loginProviderFlag string
	loginServerFlag   string
	loginNoBrowser    bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in via OIDC in the browser and store a session token",
	RunE:  runLogin,
}

func runLogin(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	serverURL := loginServerFlag
	if serverURL == "" {
		resolved, _, err := resolveBaseURL()
		if err != nil {
			return fmt.Errorf("resolve server URL: %w", err)
		}
		serverURL = resolved
	}
	serverURL = strings.TrimRight(serverURL, "/")

	if err := guardHTTPS(serverURL); err != nil {
		return err
	}
	if err := guardRemote(); err != nil {
		return err
	}

	provider, err := pickProvider(cmd.Context(), serverURL, loginProviderFlag)
	if err != nil {
		return err
	}

	verifier := oauth2.GenerateVerifier()
	challenge := oauth2.S256ChallengeFromVerifier(verifier)
	cliState := oauth2.GenerateVerifier() // reuse as a high-entropy opaque token

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start loopback listener: %w", err)
	}
	defer func() { _ = ln.Close() }() //nolint:errcheck // best-effort cleanup
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("loopback listener address is not TCP: %T", ln.Addr())
	}
	port := tcpAddr.Port
	cbURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	authURL := fmt.Sprintf("%s/api/auth/oidc/%s/start?%s", serverURL, url.PathEscape(provider),
		url.Values{"cli_callback": {cbURL}, "cli_state": {cliState}, "cli_challenge": {challenge}}.Encode())

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Handler: loopbackHandler(port, cliState, codeCh, errCh), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()  //nolint:errcheck // Serve always returns a non-nil error on Close
	defer func() { _ = srv.Close() }() //nolint:errcheck // best-effort cleanup

	if loginNoBrowser {
		fmt.Fprintf(w, "Open this URL to log in:\n\n  %s\n\n", authURL) //nolint:errcheck // writes to user stream
	} else if openErr := browser.Open(authURL); openErr != nil {
		fmt.Fprintf(w, "Could not open a browser. Open this URL to log in:\n\n  %s\n\n", authURL) //nolint:errcheck // writes to user stream
	}
	fmt.Fprintln(w, "Waiting for the browser to complete login…") //nolint:errcheck // writes to user stream

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return err
	case <-time.After(3 * time.Minute):
		return errors.New("login timed out, please retry")
	// Inert until the root wires signal-based cancellation; harmless to keep.
	case <-cmd.Context().Done():
		return cmd.Context().Err()
	}

	token, subject, err := exchangeCode(cmd.Context(), serverURL, code, verifier)
	if err != nil {
		return err
	}

	if err := writeCredentials(w, serverURL, token, subject); err != nil {
		return err
	}
	fmt.Fprintf(w, "Logged in as %s on %s\n", subject, serverURL) //nolint:errcheck // writes to user stream
	return nil
}

// loopbackHandler validates the Host header + cli_state and forwards the code.
func loopbackHandler(port int, cliState string, codeCh chan<- string, errCh chan<- error) http.Handler {
	wantHost := fmt.Sprintf("127.0.0.1:%d", port)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != wantHost {
			http.Error(w, "bad host", http.StatusBadRequest)
			return
		}
		// Ignore stray requests (favicon, prefetch, loopback port scans) without
		// aborting the login — only /callback drives the flow.
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(q.Get("cli_state")), []byte(cliState)) != 1 {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- errors.New("loopback state mismatch — aborting")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- errors.New("loopback callback missing code")
			return
		}
		fmt.Fprintln(w, "Login complete. You can close this tab and return to the terminal.") //nolint:errcheck // writes to user stream
		codeCh <- code
	})
}

func guardHTTPS(serverURL string) error {
	u, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("parse server URL: %w", err)
	}
	if u.Scheme == "https" || auth.IsLiteralLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("refusing to log in over plain http to a non-loopback server (%s); use https", serverURL)
}

// guardRemote rejects SSH/headless sessions where the loopback redirect cannot
// reach the CLI host. Applies regardless of --no-browser: over SSH a loopback
// login can never complete (the browser runs on a different machine), so we
// fail fast with actionable guidance instead of hanging until the timeout.
func guardRemote() error {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return errors.New("browser-based login isn't available over SSH/headless sessions; create an API key instead: specgraph auth api-key create")
	}
	return nil
}

type providersResponse struct {
	Providers []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"providers"`
}

func pickProvider(ctx context.Context, serverURL, want string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/api/auth/oidc/providers", http.NoBody)
	if err != nil {
		return "", fmt.Errorf("build providers request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("list providers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort cleanup
	var pr providersResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return "", fmt.Errorf("decode providers: %w", err)
	}
	if len(pr.Providers) == 0 {
		return "", errors.New("server has no interactive OIDC providers; use specgraph auth api-key create")
	}
	if want != "" {
		for _, p := range pr.Providers {
			if p.ID == want {
				return p.ID, nil
			}
		}
		return "", fmt.Errorf("provider %q not found on server", want)
	}
	if len(pr.Providers) == 1 {
		return pr.Providers[0].ID, nil
	}
	return "", errors.New("multiple OIDC providers configured; pass --provider <id>")
}

func exchangeCode(ctx context.Context, serverURL, code, verifier string) (token, subject string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	body, err := json.Marshal(map[string]string{"code": code, "cli_verifier": verifier})
	if err != nil {
		return "", "", fmt.Errorf("marshal exchange request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/api/auth/cli/exchange", bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("build exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("exchange code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort cleanup
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest {
			return "", "", errors.New("login expired, please retry")
		}
		return "", "", fmt.Errorf("exchange failed: server returned %d", resp.StatusCode)
	}
	var er struct {
		Token       string `json:"token"`
		OIDCSubject string `json:"oidc_subject"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return "", "", fmt.Errorf("decode exchange response: %w", err)
	}
	return er.Token, er.OIDCSubject, nil
}

func writeCredentials(w io.Writer, serverURL, token, subject string) error {
	credPath := xdg.CredentialsFile()
	f, err := credentials.Load(credPath)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	if existing := f.TokenFor(serverURL); existing != "" && !strings.HasPrefix(existing, "spgr_ws_") {
		fmt.Fprintf(w, "warning: replacing a non-session credential for %s\n", serverURL) //nolint:errcheck // writes to user stream
	}
	f.Upsert(serverURL, credentials.ServerCreds{Token: token, Label: "oidc:" + subject})
	if err := f.Save(credPath); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	return nil
}

func init() {
	loginCmd.Flags().StringVar(&loginProviderFlag, "provider", "", "OIDC provider id (skips the picker)")
	loginCmd.Flags().StringVar(&loginServerFlag, "server", "", "server base URL (overrides resolved)")
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "print the URL instead of opening a browser")
	rootCmd.AddCommand(loginCmd)
}
