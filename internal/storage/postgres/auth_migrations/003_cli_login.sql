-- SPDX-License-Identifier: Apache-2.0
-- Copyright 2026 Sean Brandt

-- +goose Up

ALTER TABLE oidc_login_flows
    ADD COLUMN cli_callback  text NOT NULL DEFAULT '' CHECK (length(cli_callback) <= 512),
    ADD COLUMN cli_state     text NOT NULL DEFAULT '' CHECK (length(cli_state) <= 512),
    ADD COLUMN cli_challenge text NOT NULL DEFAULT '' CHECK (length(cli_challenge) <= 256);

-- A flow is either a web flow (all CLI fields empty) or a CLI flow (all set).
-- Reject partial CLI state at the storage layer as defense-in-depth.
ALTER TABLE oidc_login_flows
    ADD CONSTRAINT oidc_login_flows_cli_all_or_none CHECK (
        (cli_callback = '' AND cli_state = '' AND cli_challenge = '')
        OR (cli_callback <> '' AND cli_state <> '' AND cli_challenge <> '')
    );

CREATE TABLE cli_login_codes (
    code_hash     bytea PRIMARY KEY,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    oidc_subject  text NOT NULL DEFAULT '' CHECK (length(oidc_subject) <= 255),
    cli_challenge text NOT NULL CHECK (length(cli_challenge) <= 256),
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL
);

CREATE INDEX cli_login_codes_expiry ON cli_login_codes(expires_at);

-- +goose Down

DROP INDEX IF EXISTS cli_login_codes_expiry;
DROP TABLE IF EXISTS cli_login_codes;
ALTER TABLE oidc_login_flows
    DROP CONSTRAINT IF EXISTS oidc_login_flows_cli_all_or_none,
    DROP COLUMN IF EXISTS cli_challenge,
    DROP COLUMN IF EXISTS cli_state,
    DROP COLUMN IF EXISTS cli_callback;
