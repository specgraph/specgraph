-- SPDX-License-Identifier: MIT
-- Copyright 2026 Sean Brandt

-- +goose Up

-- Drop single-project uniqueness to allow multiple layers per project.
DROP INDEX IF EXISTS idx_constitutions_project;

-- Add source tracking columns for external layer sources.
ALTER TABLE constitutions ADD COLUMN IF NOT EXISTS source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE constitutions ADD COLUMN IF NOT EXISTS source_hash TEXT NOT NULL DEFAULT '';

-- Enforce at most one constitution row per (project, layer).
CREATE UNIQUE INDEX idx_constitutions_project_layer ON constitutions (project_slug, layer);

-- +goose Down

-- WARNING: The unique index on (project_slug) will fail if multiple rows per
-- project exist (i.e., different layers were stored). Before running this
-- rollback, deduplicate to one row per project_slug or the final CREATE INDEX
-- will error.
DROP INDEX IF EXISTS idx_constitutions_project_layer;
ALTER TABLE constitutions DROP COLUMN IF EXISTS source_hash;
ALTER TABLE constitutions DROP COLUMN IF EXISTS source_url;
CREATE UNIQUE INDEX idx_constitutions_project ON constitutions (project_slug);
