-- SPDX-License-Identifier: Apache-2.0
-- Copyright 2026 Sean Brandt

-- +goose Up

CREATE TABLE web_sessions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash    bytea NOT NULL UNIQUE,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer        text NOT NULL DEFAULT '' CHECK (length(issuer) <= 512),
    oidc_subject  text NOT NULL DEFAULT '' CHECK (length(oidc_subject) <= 255),
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz
);

CREATE INDEX web_sessions_user   ON web_sessions(user_id);
CREATE INDEX web_sessions_expiry ON web_sessions(expires_at) WHERE revoked_at IS NULL;

CREATE TABLE oidc_login_flows (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    state         text NOT NULL CHECK (length(state) <= 512),
    nonce         text NOT NULL CHECK (length(nonce) <= 512),
    code_verifier text NOT NULL CHECK (length(code_verifier) <= 256),
    provider_id   text NOT NULL CHECK (length(provider_id) <= 128),
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL
);

CREATE INDEX oidc_login_flows_expiry ON oidc_login_flows(expires_at);

-- +goose Down

DROP INDEX IF EXISTS oidc_login_flows_expiry;
DROP TABLE IF EXISTS oidc_login_flows;
DROP INDEX IF EXISTS web_sessions_expiry;
DROP INDEX IF EXISTS web_sessions_user;
DROP TABLE IF EXISTS web_sessions;
