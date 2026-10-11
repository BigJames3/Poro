BEGIN;

-- Name the client gives its device at sign-in, shown in the session list.
ALTER TABLE refresh_tokens ADD COLUMN device_name VARCHAR(60);

-- Erasure asked by the owner: the account is erased once this date passes,
-- unless its owner signs in again before.
ALTER TABLE users ADD COLUMN deletion_scheduled_at TIMESTAMPTZ;
CREATE INDEX idx_users_deletion_scheduled_at ON users (deletion_scheduled_at)
    WHERE deletion_scheduled_at IS NOT NULL AND deleted_at IS NULL;

-- An erased account keeps its row (and id) but no identifier.
ALTER TABLE users DROP CONSTRAINT users_identifier_check;
ALTER TABLE users ADD CONSTRAINT users_identifier_check
    CHECK (phone IS NOT NULL OR email IS NOT NULL OR status = 'DELETED');

COMMIT;
