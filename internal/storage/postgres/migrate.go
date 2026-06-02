// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver for database/sql
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// gooseMu serializes access to goose's package-global state
// (SetBaseFS / SetDialect / SetTableName). Both runMigrations and
// runAuthMigrations mutate that state, so they must never interleave.
var gooseMu sync.Mutex

func runMigrations(connString string) error {
	gooseMu.Lock()
	defer gooseMu.Unlock()

	db, err := sql.Open("pgx", connString)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			slog.Error("close migration connection", "error", cErr)
		}
	}()

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
