// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/specgraph/specgraph/internal/bootstrap"
	"github.com/specgraph/specgraph/internal/notify"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/telemetry"
)

// errClass distinguishes connect failures that warrant different retry policies.
type errClass int

const (
	// classOther covers transient/operational failures (connection refused,
	// dial timeout, migration failure). Retried forever with backoff.
	classOther errClass = iota
	// classCredential covers PostgreSQL SQLSTATE class 28 (invalid auth).
	// Misconfiguration: retried a bounded number of times, then fatal.
	classCredential
)

// classifyConnectError inspects err for a pgx *pgconn.PgError with a SQLSTATE
// in the 28xxx class (28P01 invalid_password, 28000 invalid_authorization).
func classifyConnectError(err error) errClass {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Code, "28") {
		return classCredential
	}
	return classOther
}

// connectResult holds the live stores produced by a successful connect unit.
type connectResult struct {
	store     *postgres.Store
	authStore *postgres.AuthStore
	bootstrap bootstrap.Result
}

// connectStore runs the pool-touching prefix once:
// postgres.New → Subscribe → NewAuth → bootstrap.Ensure. bootstrap.Ensure is
// deliberately last so the one-time admin token is minted only when nothing
// fallible runs after it within an attempt.
//
// Partial-failure cleanup: if postgres.New succeeds but a later step fails, the
// created pool (and borrowing auth store) is closed before returning, so
// repeated retries never leak a pgxpool.Pool. postgres.New self-cleans its own
// internal failures; this covers the steps after it.
func connectStore(ctx context.Context, connURL string) (connectResult, error) {
	s, err := postgres.New(ctx, connURL, postgres.WithProject("_server"), postgres.WithTracing(telState.enabled))
	if err != nil {
		return connectResult{}, err
	}

	var authStore *postgres.AuthStore
	ok := false
	defer func() {
		if ok {
			return
		}
		// Teardown order mirrors shutdown: auth (borrows pool) before store
		// (owns pool).
		if authStore != nil {
			if cerr := authStore.Close(ctx); cerr != nil {
				slog.LogAttrs(ctx, slog.LevelWarn, "auth store close during connect cleanup", slog.Any("error", cerr))
			}
		}
		if cerr := s.Close(ctx); cerr != nil {
			slog.LogAttrs(ctx, slog.LevelWarn, "store close during connect cleanup", slog.Any("error", cerr))
		}
	}()

	s.Subscribe(notify.NewImpactLogger())
	if telState.enabled {
		s.Subscribe(telemetry.NewMetricsSubscriber())
	}

	authStore, err = postgres.NewAuth(ctx, s.Pool())
	if err != nil {
		return connectResult{}, err
	}

	res, err := bootstrap.Ensure(ctx, authStore, bootstrap.Options{})
	if err != nil {
		return connectResult{}, err
	}

	ok = true
	return connectResult{store: s, authStore: authStore, bootstrap: res}, nil
}

// backoffPolicy parametrises the connect retry loop.
type backoffPolicy struct {
	otherBase    time.Duration // first backoff for "other" errors (1s)
	otherMax     time.Duration // cap for "other" backoff (1m)
	credInterval time.Duration // fixed interval between credential retries (1s)
	credMaxRetry int           // max consecutive credential attempts before fatal (5)
}

func defaultBackoffPolicy() backoffPolicy {
	return backoffPolicy{
		otherBase:    time.Second,
		otherMax:     time.Minute,
		credInterval: time.Second,
		credMaxRetry: 5,
	}
}

type attemptFunc func(context.Context) (connectResult, error)

// ctxSleep sleeps for d or returns early if ctx is cancelled.
func ctxSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// runConnector retries attempt until it succeeds, ctx is cancelled, or
// consecutive credential failures are exhausted (returns a fatal error).
// Credential errors retry at a fixed interval up to credMaxRetry; a
// non-credential attempt resets the counter. All other errors retry forever
// with exponential backoff capped at otherMax, logged loudly.
func runConnector(ctx context.Context, attempt attemptFunc, pol backoffPolicy, sleep func(context.Context, time.Duration) error) (connectResult, error) {
	credCount := 0
	backoff := pol.otherBase
	for {
		res, err := attempt(ctx)
		if err == nil {
			return res, nil
		}
		if ctx.Err() != nil {
			return connectResult{}, ctx.Err()
		}
		switch classifyConnectError(err) {
		case classCredential:
			credCount++
			slog.LogAttrs(ctx, slog.LevelError, "postgres credential failure",
				slog.Int("attempt", credCount), slog.Int("max", pol.credMaxRetry), slog.Any("error", err))
			if credCount >= pol.credMaxRetry {
				return connectResult{}, fmt.Errorf("postgres credentials rejected after %d attempts: %w", credCount, err)
			}
			if serr := sleep(ctx, pol.credInterval); serr != nil {
				return connectResult{}, serr
			}
		default:
			credCount = 0
			slog.LogAttrs(ctx, slog.LevelWarn, "postgres unavailable, retrying",
				slog.Duration("backoff", backoff), slog.Any("error", err))
			if serr := sleep(ctx, backoff); serr != nil {
				return connectResult{}, serr
			}
			backoff *= 2
			if backoff > pol.otherMax {
				backoff = pol.otherMax
			}
		}
	}
}

// firstConnect performs the synchronous boot phase against the real
// connectStore. Returns (res, false, nil) on success; (zero, true, nil) when
// the first failure is non-credential (caller goes degraded); (zero, false,
// err) when credentials are exhausted or ctx ends.
func firstConnect(ctx context.Context, connURL string, pol backoffPolicy) (connectResult, bool, error) {
	credCount := 0
	for {
		res, err := connectStore(ctx, connURL)
		if err == nil {
			return res, false, nil
		}
		if ctx.Err() != nil {
			return connectResult{}, false, ctx.Err()
		}
		if classifyConnectError(err) != classCredential {
			return connectResult{}, true, nil
		}
		credCount++
		slog.LogAttrs(ctx, slog.LevelError, "postgres credential failure",
			slog.Int("attempt", credCount), slog.Int("max", pol.credMaxRetry), slog.Any("error", err))
		if credCount >= pol.credMaxRetry {
			return connectResult{}, false, fmt.Errorf("postgres credentials rejected after %d attempts: %w", credCount, err)
		}
		if serr := ctxSleep(ctx, pol.credInterval); serr != nil {
			return connectResult{}, false, serr
		}
	}
}
