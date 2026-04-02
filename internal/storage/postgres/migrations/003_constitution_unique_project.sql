-- SPDX-License-Identifier: MIT
-- Copyright 2026 Sean Brandt

-- +goose Up

-- Ensure at most one constitution row per project to prevent concurrent first-write forks.
CREATE UNIQUE INDEX idx_constitutions_project ON constitutions (project_slug);

-- +goose Down

DROP INDEX IF EXISTS idx_constitutions_project;
