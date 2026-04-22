-- SPDX-License-Identifier: MIT
-- Copyright 2026 Sean Brandt

-- +goose Up
ALTER TABLE conversation_logs ADD COLUMN posture TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE conversation_logs DROP COLUMN posture;
