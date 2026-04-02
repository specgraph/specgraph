-- SPDX-License-Identifier: MIT
-- Copyright 2026 Sean Brandt

-- +goose Up

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE projects (
    slug           TEXT PRIMARY KEY,
    sync_adapters  TEXT[] NOT NULL DEFAULT '{}',
    github_repo    TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE specs (
    slug             TEXT NOT NULL,
    project_slug     TEXT NOT NULL REFERENCES projects(slug),
    intent           TEXT NOT NULL DEFAULT '',
    stage            TEXT NOT NULL DEFAULT 'spark',
    priority         TEXT NOT NULL DEFAULT '',
    complexity       TEXT NOT NULL DEFAULT '',
    lifecycle        TEXT NOT NULL DEFAULT 'task',
    notes            TEXT NOT NULL DEFAULT '',
    content_hash     TEXT NOT NULL DEFAULT '',
    superseded_by    TEXT NOT NULL DEFAULT '',
    supersedes       TEXT NOT NULL DEFAULT '',
    version          INTEGER NOT NULL DEFAULT 1,
    spark_output     JSONB,
    shape_output     JSONB,
    specify_output   JSONB,
    decompose_output JSONB,
    safety_flags     JSONB,
    embedding        vector(3072),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, slug)
);

CREATE TABLE decisions (
    slug                  TEXT NOT NULL,
    project_slug          TEXT NOT NULL REFERENCES projects(slug),
    title                 TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'proposed',
    body                  TEXT NOT NULL DEFAULT '',
    rationale             TEXT NOT NULL DEFAULT '',
    question              TEXT NOT NULL DEFAULT '',
    superseded_by         TEXT NOT NULL DEFAULT '',
    confidence            TEXT NOT NULL DEFAULT '',
    scope                 TEXT NOT NULL DEFAULT '',
    origin_spec           TEXT NOT NULL DEFAULT '',
    origin_stage          TEXT NOT NULL DEFAULT '',
    tags                  TEXT[] NOT NULL DEFAULT '{}',
    rejected_alternatives JSONB,
    content_hash          TEXT NOT NULL DEFAULT '',
    version               INTEGER NOT NULL DEFAULT 1,
    embedding             vector(3072),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, slug)
);

CREATE TABLE slices (
    slug           TEXT NOT NULL,
    project_slug   TEXT NOT NULL REFERENCES projects(slug),
    parent_slug    TEXT NOT NULL,
    slice_id       TEXT NOT NULL,
    intent         TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'open',
    assigned_to    TEXT NOT NULL DEFAULT '',
    verify         TEXT[] NOT NULL DEFAULT '{}',
    touches        TEXT[] NOT NULL DEFAULT '{}',
    depends_on     TEXT[] NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, slug),
    FOREIGN KEY (project_slug, parent_slug) REFERENCES specs(project_slug, slug)
);

CREATE TABLE edges (
    from_slug            TEXT NOT NULL,
    to_slug              TEXT NOT NULL,
    edge_type            TEXT NOT NULL,
    project_slug         TEXT NOT NULL REFERENCES projects(slug),
    content_hash_at_link TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, from_slug, to_slug, edge_type)
);

CREATE INDEX idx_edges_forward ON edges (project_slug, from_slug, edge_type) INCLUDE (to_slug);
CREATE INDEX idx_edges_reverse ON edges (project_slug, to_slug, edge_type) INCLUDE (from_slug);

CREATE TABLE changelog_entries (
    id           TEXT NOT NULL,
    spec_slug    TEXT NOT NULL,
    project_slug TEXT NOT NULL REFERENCES projects(slug),
    version      INTEGER NOT NULL,
    stage        TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    checkpoint   BOOLEAN NOT NULL DEFAULT false,
    summary      TEXT NOT NULL DEFAULT '',
    reason       TEXT NOT NULL DEFAULT '',
    changes      JSONB NOT NULL DEFAULT '[]',
    date         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);
CREATE INDEX idx_changelog_spec ON changelog_entries (project_slug, spec_slug, version);

CREATE TABLE findings (
    id           TEXT NOT NULL,
    spec_slug    TEXT NOT NULL,
    project_slug TEXT NOT NULL REFERENCES projects(slug),
    pass_type    TEXT NOT NULL,
    severity     TEXT NOT NULL DEFAULT '',
    summary      TEXT NOT NULL DEFAULT '',
    detail       TEXT NOT NULL DEFAULT '',
    constraint_  TEXT NOT NULL DEFAULT '',
    resolution   TEXT NOT NULL DEFAULT '',
    version      INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);
CREATE INDEX idx_findings_spec ON findings (project_slug, spec_slug, pass_type);

CREATE TABLE conversation_logs (
    id             TEXT NOT NULL,
    spec_slug      TEXT NOT NULL,
    project_slug   TEXT NOT NULL REFERENCES projects(slug),
    stage          TEXT NOT NULL DEFAULT '',
    version        INTEGER NOT NULL DEFAULT 0,
    is_amend       BOOLEAN NOT NULL DEFAULT false,
    exchanges      JSONB NOT NULL DEFAULT '[]',
    exchange_count INTEGER NOT NULL DEFAULT 0,
    date           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);
CREATE INDEX idx_conversations_spec ON conversation_logs (project_slug, spec_slug);

CREATE TABLE claims (
    spec_slug      TEXT NOT NULL,
    project_slug   TEXT NOT NULL REFERENCES projects(slug),
    agent          TEXT NOT NULL,
    lease_expires  TIMESTAMPTZ NOT NULL,
    claimed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, spec_slug)
);
-- Note: idx_claims_active partial index omitted — now() is volatile and cannot
-- be used in index predicates. Uniqueness is enforced by the PRIMARY KEY on
-- (project_slug, spec_slug). Lease expiry filtering is handled at query time.

CREATE TABLE execution_events (
    id           TEXT NOT NULL,
    spec_slug    TEXT NOT NULL,
    project_slug TEXT NOT NULL REFERENCES projects(slug),
    agent        TEXT NOT NULL DEFAULT '',
    event_type   TEXT NOT NULL,
    message      TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);
CREATE INDEX idx_exec_events_spec ON execution_events (project_slug, spec_slug, created_at DESC);

CREATE TABLE constitutions (
    id           TEXT NOT NULL,
    project_slug TEXT NOT NULL REFERENCES projects(slug),
    layer        TEXT NOT NULL DEFAULT '',
    name         TEXT NOT NULL DEFAULT '',
    version      INTEGER NOT NULL DEFAULT 1,
    data         JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);

CREATE TABLE sync_mappings (
    spec_slug     TEXT NOT NULL,
    project_slug  TEXT NOT NULL REFERENCES projects(slug),
    adapter       TEXT NOT NULL,
    external_id   TEXT NOT NULL DEFAULT '',
    state         TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    last_sync     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, spec_slug, adapter)
);

-- +goose Down

DROP TABLE IF EXISTS sync_mappings;
DROP TABLE IF EXISTS constitutions;
DROP TABLE IF EXISTS execution_events;
DROP TABLE IF EXISTS claims;
DROP TABLE IF EXISTS conversation_logs;
DROP TABLE IF EXISTS findings;
DROP TABLE IF EXISTS changelog_entries;
DROP TABLE IF EXISTS edges;
DROP TABLE IF EXISTS slices;
DROP TABLE IF EXISTS decisions;
DROP TABLE IF EXISTS specs;
DROP TABLE IF EXISTS projects;
DROP EXTENSION IF EXISTS vector;
