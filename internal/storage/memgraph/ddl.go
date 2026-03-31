// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

// runDDLStatements executes each DDL statement (index or constraint) in its own
// auto-commit session. Memgraph requires DDL to run outside multi-statement
// transactions. Statements that fail with "already exists" are silently ignored.
func runDDLStatements(ctx context.Context, driver neo4j.Driver, stmts []string) error {
	for _, stmt := range stmts {
		session := driver.NewSession(ctx, neo4j.SessionConfig{})
		_, runErr := session.Run(ctx, stmt, nil)
		closeErr := session.Close(ctx)
		if runErr != nil && !strings.Contains(runErr.Error(), "already exists") {
			if closeErr != nil {
				return errors.Join(
					fmt.Errorf("run DDL %q: %w", stmt, runErr),
					fmt.Errorf("close session: %w", closeErr),
				)
			}
			return fmt.Errorf("run DDL %q: %w", stmt, runErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close session after DDL %q: %w", stmt, closeErr)
		}
	}
	return nil
}
