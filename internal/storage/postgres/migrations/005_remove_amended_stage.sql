-- +goose Up
-- Move any specs in the removed "amended" stage to "spark" (safe re-entry default).
UPDATE specs SET stage = 'spark' WHERE stage = 'amended';

-- +goose Down
-- No-op: cannot restore the original stage since we don't know what it was.
