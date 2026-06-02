-- SPDX-License-Identifier: Apache-2.0
-- Copyright 2026 Sean Brandt

-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind            text NOT NULL CHECK (kind IN ('human','service_account')),
    display_name    text NOT NULL CHECK (length(display_name) <= 255),
    email           text NOT NULL DEFAULT '' CHECK (length(email) <= 320),
    role            text NOT NULL CHECK (length(role) <= 64),
    owner_user_id   uuid REFERENCES users(id),
    bootstrap       boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,

    CHECK (kind = 'service_account' OR owner_user_id IS NULL),
    CHECK (kind = 'human'           OR owner_user_id IS NOT NULL)
);

CREATE UNIQUE INDEX users_one_bootstrap ON users(bootstrap)
    WHERE bootstrap = true AND deleted_at IS NULL;

CREATE TABLE oidc_bindings (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer          text NOT NULL CHECK (length(issuer) <= 512),
    subject         text NOT NULL CHECK (length(subject) <= 255),
    email_at_bind   text NOT NULL DEFAULT '' CHECK (length(email_at_bind) <= 320),
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject)
);

CREATE TABLE api_keys (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prefix          text NOT NULL UNIQUE CHECK (length(prefix) >= 8 AND length(prefix) <= 16),
    phc_hash        text NOT NULL CHECK (length(phc_hash) >= 32 AND length(phc_hash) <= 256),
    role_downgrade  text NOT NULL DEFAULT '' CHECK (length(role_downgrade) <= 64),
    label           text NOT NULL DEFAULT '' CHECK (length(label) <= 128),
    expires_at      timestamptz,
    last_used_at    timestamptz,
    revoked_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX api_keys_active ON api_keys(user_id)
    WHERE revoked_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS api_keys_active;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS oidc_bindings;
DROP INDEX IF EXISTS users_one_bootstrap;
DROP TABLE IF EXISTS users;
