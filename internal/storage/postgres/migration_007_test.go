// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver for database/sql
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestMigration007_RefusesNonEmptyTable verifies that the precondition guard in
// migration 007 prevents it from running when the specs table already contains rows.
//
// The test:
//  1. Spins up a fresh postgres container (independent of the TestMain container).
//  2. Applies migrations 001–006 via goose.UpTo.
//  3. Inserts a spec row directly using the pre-007 schema shape (has lifecycle column,
//     no provenance_type/provenance_detail columns).
//  4. Attempts to apply migration 007 via goose.UpTo.
//  5. Asserts that migration 007 errors with the precondition message.
func TestMigration007_RefusesNonEmptyTable(t *testing.T) {
	ctx := context.Background()

	// Spin up an independent container so this test doesn't conflict with
	// the shared container from TestMain.
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg18",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "mig007",
			"POSTGRES_PASSWORD": "mig007",
			"POSTGRES_DB":       "mig007db",
		},
		WaitingFor: wait.ForLog("database system is ready").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start dedicated postgres container for migration test")
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	cs := fmt.Sprintf("postgres://mig007:mig007@%s:%s/mig007db", host, port.Port())

	db, err := sql.Open("pgx", cs)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Wait for the database to be ready.
	for range 10 {
		if pingErr := db.Ping(); pingErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, db.Ping(), "database must be pingable before running migrations")

	// Resolve the migrations directory on disk relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed")
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "migrations")
	_, statErr := os.Stat(migrationsDir)
	require.NoError(t, statErr, "migrations directory must exist at %s", migrationsDir)

	// Apply migrations 001–006 only.
	goose.SetBaseFS(nil) // use the real filesystem, not embedded FS
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose.SetDialect: %v", err)
	}
	err = goose.UpTo(db, migrationsDir, 6)
	require.NoError(t, err, "migrations 001–006 must apply cleanly")

	// Insert a spec row using the pre-007 schema (has lifecycle column, no provenance columns).
	_, err = db.ExecContext(ctx,
		`INSERT INTO specs (slug, project_slug, intent, stage, priority, complexity, lifecycle, notes, content_hash, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		"existing-spec",         // slug
		"test-project",          // project_slug
		"an existing spec",      // intent
		"spark",                 // stage
		"p2",                    // priority
		"medium",                // complexity
		"task",                  // lifecycle (pre-007 column)
		"",                      // notes
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // content_hash (32 chars)
		int32(1),                // version
		time.Now().UTC(),        // created_at
		time.Now().UTC(),        // updated_at
	)
	require.NoError(t, err, "inserting pre-007 spec row must succeed")

	// Now attempt migration 007 — must fail with precondition error.
	err = goose.UpTo(db, migrationsDir, 7)
	require.Error(t, err, "migration 007 must refuse to run when specs table is non-empty")
	require.Contains(t, err.Error(), "migration 007 refuses to run on a non-empty specs table",
		"error must contain the precondition message from the migration guard")
}
