BEGIN;

DROP INDEX IF EXISTS idx_refresh_tokens_replaced_by;
DROP INDEX IF EXISTS idx_refresh_tokens_family_id;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS replaced_by;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS family_id;

COMMIT;
