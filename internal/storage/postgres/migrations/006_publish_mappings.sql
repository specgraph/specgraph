-- SPDX-License-Identifier: Apache-2.0
-- Copyright 2026 Sean Brandt

-- +goose Up
CREATE TABLE page_mappings (
    spec_slug      TEXT NOT NULL,
    doc_kind       TEXT NOT NULL CHECK (doc_kind IN ('prd', 'sdd', 'adr')),
    decision_slug  TEXT NOT NULL DEFAULT '',
    page_id        TEXT NOT NULL,
    page_version   INTEGER NOT NULL DEFAULT 0,
    spec_version   INTEGER NOT NULL DEFAULT 0,
    state          TEXT NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'synced', 'error', 'unpublished')),
    error_message  TEXT NOT NULL DEFAULT '',
    last_sync      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (spec_slug, doc_kind, decision_slug)
);

CREATE TABLE feedback_entries (
    id             TEXT PRIMARY KEY,
    external_id    TEXT NOT NULL UNIQUE,
    spec_slug      TEXT NOT NULL,
    author         TEXT NOT NULL DEFAULT '',
    body           TEXT NOT NULL DEFAULT '',
    timestamp      TIMESTAMPTZ NOT NULL DEFAULT now(),
    kind           TEXT NOT NULL CHECK (kind IN ('inline', 'footer')),
    stage          TEXT NOT NULL DEFAULT '',
    is_question    BOOLEAN NOT NULL DEFAULT false,
    parent_id      TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_feedback_spec_slug ON feedback_entries (spec_slug);
CREATE INDEX idx_feedback_external_id ON feedback_entries (external_id);

-- +goose Down
DROP TABLE IF EXISTS feedback_entries;
DROP TABLE IF EXISTS page_mappings;
