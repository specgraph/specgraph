-- SPDX-License-Identifier: Apache-2.0
-- Copyright 2026 Sean Brandt

-- +goose Up
-- +goose StatementBegin

-- Precondition guard: this migration is a clean-break replacement.
-- It refuses to run if any rows exist in specs to prevent accidental
-- data loss in environments where the clean-break assumption is wrong.
DO $$
BEGIN
  IF (SELECT count(*) FROM specs) > 0 THEN
    RAISE EXCEPTION 'migration 007 refuses to run on a non-empty specs table; clean-break design assumes no data — see docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md';
  END IF;
END
$$;

ALTER TABLE specs DROP COLUMN lifecycle;
ALTER TABLE specs ADD COLUMN provenance_type TEXT NOT NULL DEFAULT 'authored';
ALTER TABLE specs ADD COLUMN provenance_detail JSONB NOT NULL DEFAULT '{"type":"authored","data":null}'::jsonb;

-- +goose StatementEnd

-- +goose Down
ALTER TABLE specs DROP COLUMN provenance_detail;
ALTER TABLE specs DROP COLUMN provenance_type;
ALTER TABLE specs ADD COLUMN lifecycle TEXT NOT NULL DEFAULT 'task';
