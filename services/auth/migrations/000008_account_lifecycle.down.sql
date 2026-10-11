BEGIN;

-- Erased accounts have no identifier left: the restored constraint skips them.
ALTER TABLE users DROP CONSTRAINT users_identifier_check;
ALTER TABLE users ADD CONSTRAINT users_identifier_check
    CHECK (phone IS NOT NULL OR email IS NOT NULL) NOT VALID;

DROP INDEX IF EXISTS idx_users_deletion_scheduled_at;
ALTER TABLE users DROP COLUMN deletion_scheduled_at;
ALTER TABLE refresh_tokens DROP COLUMN device_name;

COMMIT;
