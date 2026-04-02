-- SPDX-License-Identifier: MIT
-- Copyright 2026 Sean Brandt

-- +goose Up

-- Add unique node IDs to specs and decisions, consistent with all other entity tables.
ALTER TABLE specs ADD COLUMN id TEXT NOT NULL DEFAULT '';
ALTER TABLE decisions ADD COLUMN id TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE decisions DROP COLUMN id;
ALTER TABLE specs DROP COLUMN id;
