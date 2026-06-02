-- SPDX-License-Identifier: Apache-2.0
-- Copyright 2026 Sean Brandt
--
-- E2E seed: insert a human admin user and a matching API key so that the
-- Playwright test harness can authenticate with:
--
--   Authorization: Bearer spgr_sk_e2eadmin_e2esecret32charsfixedpaddingaaa0
--
-- PHC details:
--   secret : e2esecret32charsfixedpaddingaaa0  (32 chars, apiKeySecretLen)
--   prefix : e2eadmin                           (8 chars, apiKeyPrefixLen)
--   salt   : e2esalte2esalt16                   (16 bytes, fixed → deterministic PHC)
--   params : m=19456,t=2,p=1                    (matches test-fixture params)
--   hash   : argon2id, base64.RawStdEncoding (no padding)
--
-- Idempotent: ON CONFLICT DO NOTHING — safe to run multiple times.

INSERT INTO users (id, kind, display_name, role)
VALUES ('e2e00000-0000-0000-0000-000000000001'::uuid, 'human', 'E2E Admin', 'admin')
ON CONFLICT (id) DO NOTHING;

INSERT INTO api_keys (id, user_id, prefix, phc_hash, role_downgrade, label)
VALUES (
    'e2e00000-0000-0000-0000-000000000002'::uuid,
    'e2e00000-0000-0000-0000-000000000001'::uuid,
    'e2eadmin',
    '$argon2id$v=19$m=19456,t=2,p=1$ZTJlc2FsdGUyZXNhbHQxNg$Zc9Glm0pc9ozY/IU2gdEFm+7T9DLuvBVgsvMeBbVOVw',
    '',
    'e2e'
)
ON CONFLICT (id) DO NOTHING;
